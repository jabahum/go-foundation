package apierror

import (
	"context"
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	statuspb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const Domain = "go-foundation"

type FieldViolation struct {
	Field       string
	Description string
}

func InvalidArgument(reason, message string, fields ...FieldViolation) error {
	violations := make([]*errdetails.BadRequest_FieldViolation, 0, len(fields))
	for _, field := range fields {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       field.Field,
			Description: field.Description,
		})
	}
	return New(codes.InvalidArgument, reason, message, &errdetails.BadRequest{FieldViolations: violations})
}

func Resource(code codes.Code, reason, message, resourceType, resourceName string) error {
	return New(code, reason, message, &errdetails.ResourceInfo{
		ResourceType: resourceType,
		ResourceName: resourceName,
		Description:  message,
	})
}

func New(code codes.Code, reason, message string, details ...proto.Message) error {
	statusProto := &statuspb.Status{Code: int32(code), Message: message}
	messages := append([]proto.Message{&errdetails.ErrorInfo{Reason: reason, Domain: Domain}}, details...)
	for _, detail := range messages {
		encoded, err := anypb.New(detail)
		if err == nil {
			statusProto.Details = append(statusProto.Details, encoded)
		}
	}
	return status.ErrorProto(statusProto)
}

// Ensure attaches a stable ErrorInfo detail and the request ID to every error.
// Plain Go errors are converted to a generic Internal status to avoid exposing
// implementation details to clients.
func Ensure(err error, requestID string) error {
	if err == nil {
		return nil
	}

	grpcStatus, ok := status.FromError(err)
	if !ok {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			grpcStatus = status.FromContextError(err)
		default:
			grpcStatus = status.New(codes.Internal, "internal server error")
		}
	}
	statusProto := proto.Clone(grpcStatus.Proto()).(*statuspb.Status)

	foundErrorInfo := false
	for index, detail := range statusProto.Details {
		if !detail.MessageIs(&errdetails.ErrorInfo{}) {
			continue
		}
		var info errdetails.ErrorInfo
		if err := detail.UnmarshalTo(&info); err != nil {
			continue
		}
		if info.Metadata == nil {
			info.Metadata = make(map[string]string)
		}
		if requestID != "" {
			info.Metadata["request_id"] = requestID
		}
		encoded, err := anypb.New(&info)
		if err == nil {
			statusProto.Details[index] = encoded
			foundErrorInfo = true
		}
	}

	if !foundErrorInfo {
		metadata := make(map[string]string)
		if requestID != "" {
			metadata["request_id"] = requestID
		}
		encoded, encodeErr := anypb.New(&errdetails.ErrorInfo{
			Reason:   defaultReason(grpcStatus.Code()),
			Domain:   Domain,
			Metadata: metadata,
		})
		if encodeErr == nil {
			statusProto.Details = append(statusProto.Details, encoded)
		}
	}

	return status.ErrorProto(statusProto)
}

func defaultReason(code codes.Code) string {
	switch code {
	case codes.Canceled:
		return "REQUEST_CANCELED"
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.DeadlineExceeded:
		return "DEADLINE_EXCEEDED"
	case codes.NotFound:
		return "RESOURCE_NOT_FOUND"
	case codes.AlreadyExists:
		return "RESOURCE_ALREADY_EXISTS"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	case codes.Unavailable:
		return "SERVICE_UNAVAILABLE"
	case codes.Unimplemented:
		return "NOT_IMPLEMENTED"
	case codes.Internal:
		return "INTERNAL_ERROR"
	default:
		return "RPC_ERROR"
	}
}
