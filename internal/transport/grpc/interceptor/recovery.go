package interceptor

import (
	"context"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"log/slog"
	"runtime/debug"
)

func RecoveryUnary(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				l.Error("panic recovered", "method", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = apierror.New(codes.Internal, "PANIC_RECOVERED", "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
