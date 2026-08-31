package interceptor

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

func ValidationUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		message, ok := req.(proto.Message)
		if !ok {
			return nil, apierror.New(
				codes.Internal,
				"VALIDATION_CONFIGURATION_ERROR",
				"internal server error",
			)
		}
		if err := protovalidate.Validate(message); err != nil {
			var validationErr *protovalidate.ValidationError
			if !errors.As(err, &validationErr) {
				return nil, apierror.New(codes.Internal, "VALIDATION_CONFIGURATION_ERROR", "internal server error")
			}
			violations := validationErr.ToProto().GetViolations()
			fields := make([]apierror.FieldViolation, 0, len(violations))
			for _, violation := range violations {
				fields = append(fields, apierror.FieldViolation{
					Field:       protovalidate.FieldPathString(violation.GetField()),
					Description: violation.GetMessage(),
				})
			}
			return nil, apierror.InvalidArgument("REQUEST_VALIDATION_FAILED", "request validation failed", fields...)
		}
		return handler(ctx, req)
	}
}
