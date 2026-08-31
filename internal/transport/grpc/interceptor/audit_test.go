package interceptor

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	rbacv1 "github.com/jabahum/go-foundation/gen/proto/rbac/v1"
	domainaudit "github.com/jabahum/go-foundation/internal/domain/audit"
	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type auditRecorder struct {
	events []*domainaudit.Event
	err    error
}

func (r *auditRecorder) Record(_ context.Context, event *domainaudit.Event) error {
	r.events = append(r.events, event)
	return r.err
}

func (r *auditRecorder) List(context.Context, domainaudit.ListFilter) ([]*domainaudit.Event, error) {
	return nil, nil
}

func TestAuditUnaryRecordsMutation(t *testing.T) {
	recorder := &auditRecorder{}
	intercept := AuditUnary(recorder, slog.Default())
	ctx := security.WithIdentity(context.Background(), &auth.Identity{UserID: "d67f4944-ff2d-46c4-bdb9-7ddf27ff340f"})
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-request-id", "request-123", "user-agent", "audit-test"))
	request := &rbacv1.AssignRoleRequest{UserId: "f59c9456-baf0-42e0-8519-7e85e240832d", RoleId: "24db41d7-7e3a-4093-bdc1-b9e7d57caa60"}
	info := &grpc.UnaryServerInfo{FullMethod: "/rbac.v1.RBACService/AssignRole"}

	_, err := RequestIDUnary()(ctx, request, info, func(enriched context.Context, request any) (any, error) {
		return intercept(enriched, request, info, func(context.Context, any) (any, error) {
			return &rbacv1.AssignRoleResponse{}, nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("events = %d", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "role.assign" || event.ResourceID != request.UserId || event.ActorID == "" || event.GRPCCode != codes.OK.String() || event.RequestID != "request-123" || event.UserAgent != "audit-test" {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.Metadata["role_id"] != request.RoleId {
		t.Fatalf("role_id = %q", event.Metadata["role_id"])
	}
}

func TestAuditUnaryRecordsStructuredFailure(t *testing.T) {
	recorder := &auditRecorder{}
	intercept := AuditUnary(recorder, nil)
	wantErr := apierror.New(codes.Internal, "ROLE_ASSIGNMENT_FAILED", "assign role failed")

	_, gotErr := intercept(context.Background(), &rbacv1.AssignRoleRequest{}, &grpc.UnaryServerInfo{FullMethod: "/rbac.v1.RBACService/AssignRole"}, func(context.Context, any) (any, error) {
		return nil, wantErr
	})
	if gotErr != wantErr {
		t.Fatal("audit interceptor changed the handler error")
	}
	if recorder.events[0].GRPCCode != codes.Internal.String() || recorder.events[0].ErrorReason != "ROLE_ASSIGNMENT_FAILED" {
		t.Fatalf("unexpected event: %+v", recorder.events[0])
	}
}

func TestAuditUnaryDoesNotChangeResultWhenRecordingFails(t *testing.T) {
	recorder := &auditRecorder{err: errors.New("database unavailable")}
	intercept := AuditUnary(recorder, nil)
	response := &rbacv1.AssignRoleResponse{}
	got, err := intercept(context.Background(), &rbacv1.AssignRoleRequest{}, &grpc.UnaryServerInfo{FullMethod: "/rbac.v1.RBACService/AssignRole"}, func(context.Context, any) (any, error) {
		return response, nil
	})
	if err != nil || got != response {
		t.Fatalf("got response %v, error %v", got, err)
	}
}

func TestAuditUnaryIgnoresReadOperations(t *testing.T) {
	recorder := &auditRecorder{}
	intercept := AuditUnary(recorder, nil)
	_, _ = intercept(context.Background(), struct{}{}, &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/ListUsers"}, func(context.Context, any) (any, error) {
		return nil, nil
	})
	if len(recorder.events) != 0 {
		t.Fatalf("events = %d", len(recorder.events))
	}
}
