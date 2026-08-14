package interceptor

import (
	"context"
	"example.com/grpc-clean-starter/internal/security"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"log/slog"
	"time"
)

func LoggingUnary(l *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		attrs := []any{"method", info.FullMethod, "code", status.Code(err).String(), "duration", time.Since(start)}
		if id, ok := security.IdentityFromContext(ctx); ok {
			attrs = append(attrs, "user_id", id.UserID, "provider", id.Provider)
		}
		sc := trace.SpanContextFromContext(ctx)
		if sc.IsValid() {
			attrs = append(attrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
		}
		l.Info("grpc request", attrs...)
		return resp, err
	}
}
