package interceptor

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	idempotency "github.com/jabahum/go-foundation/internal/domain/idempotency"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	idempotencyMetadataKey  = "idempotency-key"
	idempotencyWriteTimeout = 2 * time.Second
)

type IdempotencyOptions struct {
	Enabled     bool
	TTL         time.Duration
	LockTimeout time.Duration
}

func IdempotencyUnary(store idempotency.Store, options IdempotencyOptions, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !options.Enabled || !idempotentMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		key := incomingIdempotencyKey(ctx)
		if key == "" {
			return handler(ctx, req)
		}
		if !validIdempotencyKey(key) {
			return nil, apierror.InvalidArgument(
				"IDEMPOTENCY_KEY_INVALID",
				"invalid idempotency key",
				apierror.FieldViolation{Field: idempotencyMetadataKey, Description: "must be 8-255 URL-safe characters"},
			)
		}
		identity, ok := security.IdentityFromContext(ctx)
		if !ok {
			return nil, apierror.New(codes.Unauthenticated, "AUTHENTICATION_REQUIRED", "authentication required")
		}
		request, ok := req.(proto.Message)
		if !ok {
			return nil, apierror.New(codes.Internal, "IDEMPOTENCY_CONFIGURATION_ERROR", "internal server error")
		}
		requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
		if err != nil {
			return nil, apierror.New(codes.Internal, "IDEMPOTENCY_FINGERPRINT_FAILED", "internal server error")
		}
		keyHash := sha256.Sum256([]byte(key))
		requestHash := sha256.Sum256(requestBytes)
		now := time.Now().UTC()
		ownerToken := uuid.NewString()
		record, disposition, err := store.Acquire(ctx, idempotency.AcquireParams{
			ID:          uuid.NewString(),
			ActorID:     identity.UserID,
			RPCMethod:   info.FullMethod,
			KeyHash:     keyHash[:],
			RequestHash: requestHash[:],
			OwnerToken:  ownerToken,
			Now:         now,
			LockedUntil: now.Add(options.LockTimeout),
			ExpiresAt:   now.Add(options.TTL),
		})
		if err != nil {
			logIdempotencyError(logger, "reserve idempotency key", err, ctx, info.FullMethod)
			return nil, apierror.New(codes.Unavailable, "IDEMPOTENCY_STORE_UNAVAILABLE", "idempotency service unavailable")
		}

		switch disposition {
		case idempotency.DispositionReplay:
			_ = grpc.SetHeader(ctx, metadata.Pairs("idempotency-replayed", "true"))
			return replayIdempotencyRecord(record)
		case idempotency.DispositionConflict:
			return nil, apierror.New(codes.AlreadyExists, "IDEMPOTENCY_KEY_REUSED", "idempotency key was already used with a different request")
		case idempotency.DispositionInProgress:
			return nil, apierror.New(
				codes.Aborted,
				"IDEMPOTENCY_REQUEST_IN_PROGRESS",
				"a request with this idempotency key is already in progress",
				&errdetails.RetryInfo{RetryDelay: durationpb.New(time.Second)},
			)
		case idempotency.DispositionAcquired:
			_ = grpc.SetHeader(ctx, metadata.Pairs("idempotency-replayed", "false"))
		default:
			return nil, apierror.New(codes.Internal, "IDEMPOTENCY_CONFIGURATION_ERROR", "internal server error")
		}

		response, handlerErr := handler(ctx, req)
		if transientIdempotencyError(handlerErr) {
			releaseIdempotencyRecord(store, record.ID, ownerToken, logger, ctx, info.FullMethod)
			return response, handlerErr
		}
		responseType, responsePayload, statusPayload, encodeErr := encodeIdempotencyResult(response, handlerErr)
		if encodeErr != nil {
			logIdempotencyError(logger, "encode idempotency result", encodeErr, ctx, info.FullMethod)
			return response, handlerErr
		}
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), idempotencyWriteTimeout)
		defer cancel()
		if err := store.Complete(writeCtx, record.ID, ownerToken, responseType, responsePayload, statusPayload, time.Now().UTC()); err != nil {
			logIdempotencyError(logger, "complete idempotency record", err, ctx, info.FullMethod)
		}
		return response, handlerErr
	}
}

func idempotentMethod(method string) bool {
	switch method {
	case "/auth.v1.AuthService/RevokeSession",
		"/user.v1.UserService/CreateUser",
		"/rbac.v1.RBACService/AssignRole",
		"/rbac.v1.RBACService/RemoveRole",
		"/rbac.v1.RBACService/AssignPermissionToRole",
		"/rbac.v1.RBACService/RemovePermissionFromRole":
		return true
	default:
		return false
	}
}

func incomingIdempotencyKey(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(idempotencyMetadataKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func validIdempotencyKey(key string) bool {
	if len(key) < 8 || len(key) > 255 {
		return false
	}
	for _, value := range []byte(key) {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '.' || value == '_' || value == ':' || value == '-' {
			continue
		}
		return false
	}
	return true
}

func encodeIdempotencyResult(response any, responseErr error) (string, []byte, []byte, error) {
	if responseErr != nil {
		payload, err := proto.Marshal(status.Convert(responseErr).Proto())
		return "", nil, payload, err
	}
	message, ok := response.(proto.Message)
	if !ok {
		return "", nil, nil, errors.New("idempotent response is not a protobuf message")
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return "", nil, nil, err
	}
	return string(message.ProtoReflect().Descriptor().FullName()), payload, nil, nil
}

func replayIdempotencyRecord(record *idempotency.Record) (any, error) {
	if len(record.StatusPayload) > 0 {
		value := &statuspb.Status{}
		if err := proto.Unmarshal(record.StatusPayload, value); err != nil {
			return nil, apierror.New(codes.Internal, "IDEMPOTENCY_REPLAY_FAILED", "internal server error")
		}
		return nil, status.ErrorProto(value)
	}
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(record.ResponseType))
	if err != nil {
		return nil, apierror.New(codes.Internal, "IDEMPOTENCY_REPLAY_FAILED", "internal server error")
	}
	message := messageType.New().Interface()
	if err := proto.Unmarshal(record.ResponsePayload, message); err != nil {
		return nil, apierror.New(codes.Internal, "IDEMPOTENCY_REPLAY_FAILED", "internal server error")
	}
	return message, nil
}

func transientIdempotencyError(err error) bool {
	switch status.Code(err) {
	case codes.Internal, codes.Unknown, codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Aborted, codes.Canceled:
		return true
	default:
		return false
	}
}

func releaseIdempotencyRecord(store idempotency.Store, id, ownerToken string, logger *slog.Logger, ctx context.Context, method string) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), idempotencyWriteTimeout)
	defer cancel()
	if err := store.Release(writeCtx, id, ownerToken); err != nil {
		logIdempotencyError(logger, "release idempotency record", err, ctx, method)
	}
}

func logIdempotencyError(logger *slog.Logger, message string, err error, ctx context.Context, method string) {
	if logger != nil {
		logger.Error(message, "error", err, "request_id", RequestIDFromContext(ctx), "method", method)
	}
}
