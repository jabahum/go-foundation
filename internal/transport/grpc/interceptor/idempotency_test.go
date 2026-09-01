package interceptor

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	idempotency "github.com/jabahum/go-foundation/internal/domain/idempotency"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type memoryIdempotencyStore struct {
	mu      sync.Mutex
	records map[string]*idempotency.Record
}

func newMemoryIdempotencyStore() *memoryIdempotencyStore {
	return &memoryIdempotencyStore{records: make(map[string]*idempotency.Record)}
}

func (s *memoryIdempotencyStore) Acquire(_ context.Context, params idempotency.AcquireParams) (*idempotency.Record, idempotency.Disposition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scope := params.ActorID + params.RPCMethod + string(params.KeyHash)
	if existing, ok := s.records[scope]; ok {
		if !bytes.Equal(existing.RequestHash, params.RequestHash) {
			return existing, idempotency.DispositionConflict, nil
		}
		if existing.State == idempotency.StateCompleted {
			return existing, idempotency.DispositionReplay, nil
		}
		return existing, idempotency.DispositionInProgress, nil
	}
	record := &idempotency.Record{
		ID:          params.ID,
		ActorID:     params.ActorID,
		RPCMethod:   params.RPCMethod,
		KeyHash:     append([]byte(nil), params.KeyHash...),
		RequestHash: append([]byte(nil), params.RequestHash...),
		State:       idempotency.StateInProgress,
		OwnerToken:  params.OwnerToken,
		LockedUntil: params.LockedUntil,
		ExpiresAt:   params.ExpiresAt,
	}
	s.records[scope] = record
	return record, idempotency.DispositionAcquired, nil
}

func (s *memoryIdempotencyStore) Complete(_ context.Context, id, ownerToken, responseType string, responsePayload, statusPayload []byte, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.records {
		if record.ID == id && record.OwnerToken == ownerToken && record.State == idempotency.StateInProgress {
			record.State = idempotency.StateCompleted
			record.ResponseType = responseType
			record.ResponsePayload = append([]byte(nil), responsePayload...)
			record.StatusPayload = append([]byte(nil), statusPayload...)
			return nil
		}
	}
	return idempotency.ErrReservationLost
}

func (s *memoryIdempotencyStore) Release(_ context.Context, id, ownerToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for scope, record := range s.records {
		if record.ID == id && record.OwnerToken == ownerToken {
			delete(s.records, scope)
		}
	}
	return nil
}

func TestIdempotencyUnaryReplaysCompletedResponse(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("create-user-123")
	request := &userv1.CreateUserRequest{Name: "Ada", Email: "ada@example.com", Password: "password123"}
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	calls := 0
	handler := func(context.Context, any) (any, error) {
		calls++
		return &userv1.CreateUserResponse{User: &userv1.User{Id: "user-1", Name: "Ada", Email: "ada@example.com"}}, nil
	}

	first, err := intercept(ctx, request, info, handler)
	if err != nil {
		t.Fatal(err)
	}
	second, err := intercept(ctx, request, info, handler)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
	if !proto.Equal(first.(proto.Message), second.(proto.Message)) {
		t.Fatalf("replayed response differs: %v, %v", first, second)
	}
}

func TestIdempotencyUnaryRejectsKeyReuseWithDifferentRequest(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("create-user-456")
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	calls := 0
	handler := func(context.Context, any) (any, error) {
		calls++
		return &userv1.CreateUserResponse{}, nil
	}
	_, err := intercept(ctx, &userv1.CreateUserRequest{Email: "ada@example.com"}, info, handler)
	if err != nil {
		t.Fatal(err)
	}
	_, err = intercept(ctx, &userv1.CreateUserRequest{Email: "grace@example.com"}, info, handler)
	if status.Code(err) != codes.AlreadyExists || calls != 1 {
		t.Fatalf("code = %s, calls = %d, error = %v", status.Code(err), calls, err)
	}
}

func TestIdempotencyUnaryReplaysStableError(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("create-user-789")
	request := &userv1.CreateUserRequest{Email: "ada@example.com"}
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	calls := 0
	handler := func(context.Context, any) (any, error) {
		calls++
		return nil, apierror.New(codes.AlreadyExists, "USER_EMAIL_EXISTS", "email already exists")
	}
	_, firstErr := intercept(ctx, request, info, handler)
	_, secondErr := intercept(ctx, request, info, handler)
	if status.Code(firstErr) != codes.AlreadyExists || status.Code(secondErr) != codes.AlreadyExists || calls != 1 {
		t.Fatalf("first = %v, second = %v, calls = %d", firstErr, secondErr, calls)
	}
}

func TestIdempotencyUnaryReleasesTransientFailure(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("create-user-retry")
	request := &userv1.CreateUserRequest{Email: "ada@example.com"}
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	calls := 0
	handler := func(context.Context, any) (any, error) {
		calls++
		if calls == 1 {
			return nil, apierror.New(codes.Unavailable, "DATABASE_UNAVAILABLE", "service unavailable")
		}
		return &userv1.CreateUserResponse{}, nil
	}
	_, _ = intercept(ctx, request, info, handler)
	_, err := intercept(ctx, request, info, handler)
	if err != nil || calls != 2 {
		t.Fatalf("error = %v, calls = %d", err, calls)
	}
}

func TestIdempotencyUnaryRejectsConcurrentDuplicate(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("create-user-concurrent")
	request := &userv1.CreateUserRequest{Email: "ada@example.com"}
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := intercept(ctx, request, info, func(context.Context, any) (any, error) {
			close(started)
			<-release
			return &userv1.CreateUserResponse{}, nil
		})
		done <- err
	}()
	<-started

	_, err := intercept(ctx, request, info, func(context.Context, any) (any, error) {
		t.Fatal("concurrent duplicate reached handler")
		return nil, nil
	})
	if status.Code(err) != codes.Aborted {
		t.Fatalf("code = %s, error = %v", status.Code(err), err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestIdempotencyUnaryRejectsInvalidKey(t *testing.T) {
	store := newMemoryIdempotencyStore()
	intercept := IdempotencyUnary(store, testIdempotencyOptions(), nil)
	ctx := idempotencyContext("bad key")
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"}
	called := false
	_, err := intercept(ctx, &userv1.CreateUserRequest{}, info, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	if status.Code(err) != codes.InvalidArgument || called {
		t.Fatalf("code = %s, called = %v", status.Code(err), called)
	}
}

func testIdempotencyOptions() IdempotencyOptions {
	return IdempotencyOptions{Enabled: true, TTL: time.Hour, LockTimeout: time.Minute}
}

func idempotencyContext(key string) context.Context {
	ctx := security.WithIdentity(context.Background(), &auth.Identity{UserID: "d67f4944-ff2d-46c4-bdb9-7ddf27ff340f"})
	return metadata.NewIncomingContext(ctx, metadata.Pairs(idempotencyMetadataKey, key))
}
