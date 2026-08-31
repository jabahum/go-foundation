package interceptor

import (
	"context"
	"github.com/jabahum/go-foundation/internal/infrastructure/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"time"
)

func MetricsUnary(m *observability.Metrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		m.Requests.WithLabelValues(info.FullMethod, status.Code(err).String()).Inc()
		m.Duration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
		return resp, err
	}
}
