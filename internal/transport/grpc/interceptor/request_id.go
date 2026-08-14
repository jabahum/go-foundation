package interceptor

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type key string

const requestIDKey key = "request_id"

func RequestIDUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			v := md.Get("x-request-id")
			if len(v) > 0 {
				id = v[0]
			}
		}
		if id == "" {
			id = uuid.NewString()
		}
		ctx = context.WithValue(ctx, requestIDKey, id)
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", id))
		return handler(ctx, req)
	}
}
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}
