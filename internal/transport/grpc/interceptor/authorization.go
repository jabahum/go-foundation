package interceptor

import (
	"context"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	"github.com/jabahum/go-foundation/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthorizationPolicy map[string]string

func AuthorizationUnary(svc *apprbac.Service, policies AuthorizationPolicy, denied func(string)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		perm, ok := policies[info.FullMethod]
		if !ok {
			return handler(ctx, req)
		}
		id, ok := security.IdentityFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}
		allowed, err := svc.HasPermission(ctx, id.UserID, perm)
		if err != nil {
			return nil, status.Error(codes.Internal, "authorization check failed")
		}
		if !allowed {
			if denied != nil {
				denied(perm)
			}
			return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
		}
		return handler(ctx, req)
	}
}
