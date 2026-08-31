package interceptor

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	rbacv1 "github.com/jabahum/go-foundation/gen/proto/rbac/v1"
	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	audit "github.com/jabahum/go-foundation/internal/domain/audit"
	"github.com/jabahum/go-foundation/internal/security"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const auditWriteTimeout = 2 * time.Second

// AuditUnary records authorized mutation attempts after the handler returns.
// It only extracts explicitly allow-listed identifiers and never serializes a
// complete request, preventing credentials and other sensitive fields from
// entering the audit trail.
func AuditUnary(repo audit.Repository, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		response, handlerErr := handler(ctx, req)
		event, ok := mutationEvent(info.FullMethod, req, response)
		if !ok {
			return response, handlerErr
		}

		event.ID = uuid.NewString()
		event.OccurredAt = time.Now().UTC()
		event.RequestID = RequestIDFromContext(ctx)
		event.RPCMethod = info.FullMethod
		event.GRPCCode = status.Code(handlerErr).String()
		event.ErrorReason = errorReason(handlerErr)
		if identity, exists := security.IdentityFromContext(ctx); exists {
			event.ActorID = identity.UserID
		}
		if remote, exists := peer.FromContext(ctx); exists && remote.Addr != nil {
			event.ClientIP = remote.Addr.String()
		}
		if md, exists := metadata.FromIncomingContext(ctx); exists {
			if values := md.Get("user-agent"); len(values) > 0 {
				event.UserAgent = values[0]
			}
		}

		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditWriteTimeout)
		defer cancel()
		if err := repo.Record(recordCtx, event); err != nil && logger != nil {
			logger.Error("record audit event", "error", err, "request_id", event.RequestID, "action", event.Action)
		}
		return response, handlerErr
	}
}

func mutationEvent(method string, req, response any) (*audit.Event, bool) {
	switch method {
	case "/user.v1.UserService/CreateUser":
		event := &audit.Event{Action: "user.create", ResourceType: "user", Metadata: map[string]string{}}
		if value, ok := response.(*userv1.CreateUserResponse); ok && value.GetUser() != nil {
			event.ResourceID = value.GetUser().GetId()
		}
		return event, true
	case "/rbac.v1.RBACService/AssignRole":
		value, _ := req.(*rbacv1.AssignRoleRequest)
		return relationshipEvent("role.assign", "user", value.GetUserId(), "role_id", value.GetRoleId()), true
	case "/rbac.v1.RBACService/RemoveRole":
		value, _ := req.(*rbacv1.RemoveRoleRequest)
		return relationshipEvent("role.remove", "user", value.GetUserId(), "role_id", value.GetRoleId()), true
	case "/rbac.v1.RBACService/AssignPermissionToRole":
		value, _ := req.(*rbacv1.AssignPermissionToRoleRequest)
		return relationshipEvent("permission.assign", "role", value.GetRoleId(), "permission_id", value.GetPermissionId()), true
	case "/rbac.v1.RBACService/RemovePermissionFromRole":
		value, _ := req.(*rbacv1.RemovePermissionFromRoleRequest)
		return relationshipEvent("permission.remove", "role", value.GetRoleId(), "permission_id", value.GetPermissionId()), true
	default:
		return nil, false
	}
}

func relationshipEvent(action, resourceType, resourceID, relatedKey, relatedID string) *audit.Event {
	return &audit.Event{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     map[string]string{relatedKey: relatedID},
	}
}

func errorReason(err error) string {
	if err == nil {
		return ""
	}
	for _, detail := range status.Convert(err).Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}
