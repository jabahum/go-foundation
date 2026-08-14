package interceptor

import (
	"context"
	appauth "example.com/grpc-clean-starter/internal/application/auth"
	"example.com/grpc-clean-starter/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
)

func AuthenticationUnary(authn *appauth.AuthenticationService, public map[string]struct{}) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := public[info.FullMethod]; ok {
			return handler(ctx, req)
		}
		raw, err := bearer(ctx)
		if err != nil {
			return nil, err
		}
		id, err := authn.Authenticate(ctx, raw)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid access token")
		}
		return handler(security.WithIdentity(ctx, id), req)
	}
}
func bearer(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authorization metadata missing")
	}
	v := md.Get("authorization")
	if len(v) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization token missing")
	}
	parts := strings.SplitN(strings.TrimSpace(v[0]), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", status.Error(codes.Unauthenticated, "invalid authorization scheme")
	}
	return strings.TrimSpace(parts[1]), nil
}
