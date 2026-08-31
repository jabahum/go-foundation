package httptransport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	auditv1 "github.com/jabahum/go-foundation/gen/proto/audit/v1"
	authv1 "github.com/jabahum/go-foundation/gen/proto/auth/v1"
	rbacv1 "github.com/jabahum/go-foundation/gen/proto/rbac/v1"
	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewGateway(ctx context.Context, address, grpcEndpoint string, docsEnabled bool) (*http.Server, error) {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	return newGateway(ctx, address, grpcEndpoint, docsEnabled, dialOptions)
}

func newGateway(ctx context.Context, address, grpcEndpoint string, docsEnabled bool, dialOptions []grpc.DialOption) (*http.Server, error) {
	mux := runtime.NewServeMux()
	if err := auditv1.RegisterAuditServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dialOptions); err != nil {
		return nil, fmt.Errorf("register audit gateway: %w", err)
	}
	if err := authv1.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dialOptions); err != nil {
		return nil, fmt.Errorf("register auth gateway: %w", err)
	}
	if err := userv1.RegisterUserServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dialOptions); err != nil {
		return nil, fmt.Errorf("register user gateway: %w", err)
	}
	if err := rbacv1.RegisterRBACServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dialOptions); err != nil {
		return nil, fmt.Errorf("register rbac gateway: %w", err)
	}
	handler, err := withDocumentation(mux, docsEnabled)
	if err != nil {
		return nil, err
	}

	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}, nil
}
