package interceptor

import (
	"context"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
			return nil, apierror.New(codes.Unauthenticated, "AUTHENTICATION_REQUIRED", "authentication required")
		}
		allowed, err := svc.HasPermission(ctx, id.UserID, perm)
		if err != nil {
			return nil, apierror.New(codes.Internal, "AUTHORIZATION_CHECK_FAILED", "authorization check failed")
		}
		if !allowed {
			if denied != nil {
				denied(perm)
			}
			return nil, apierror.New(codes.PermissionDenied, "INSUFFICIENT_PERMISSIONS", "insufficient permissions")
		}
		return handler(ctx, req)
	}
}
