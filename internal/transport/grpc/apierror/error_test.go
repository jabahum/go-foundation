package apierror

import (
	"errors"
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEnsurePreservesStructuredDetailsAndAddsRequestID(t *testing.T) {
	err := InvalidArgument(
		"REQUEST_VALIDATION_FAILED",
		"request validation failed",
		FieldViolation{Field: "email", Description: "must be a valid email address"},
	)
	err = Ensure(err, "request-123")

	grpcStatus := status.Convert(err)
	if grpcStatus.Code() != codes.InvalidArgument {
		t.Fatalf("code = %s", grpcStatus.Code())
	}
	var foundInfo, foundBadRequest bool
	for _, detail := range grpcStatus.Details() {
		switch value := detail.(type) {
		case *errdetails.ErrorInfo:
			foundInfo = true
			if value.GetReason() != "REQUEST_VALIDATION_FAILED" || value.GetDomain() != Domain {
				t.Fatalf("error info = %#v", value)
			}
			if value.GetMetadata()["request_id"] != "request-123" {
				t.Fatalf("request ID metadata = %q", value.GetMetadata()["request_id"])
			}
		case *errdetails.BadRequest:
			foundBadRequest = true
			violations := value.GetFieldViolations()
			if len(violations) != 1 || violations[0].GetField() != "email" {
				t.Fatalf("field violations = %#v", violations)
			}
		}
	}
	if !foundInfo || !foundBadRequest {
		t.Fatalf("details missing: ErrorInfo=%v BadRequest=%v", foundInfo, foundBadRequest)
	}
}

func TestEnsureSanitizesPlainErrors(t *testing.T) {
	err := Ensure(errors.New("database password leaked"), "request-456")
	grpcStatus := status.Convert(err)
	if grpcStatus.Code() != codes.Internal || grpcStatus.Message() != "internal server error" {
		t.Fatalf("status = %s %q", grpcStatus.Code(), grpcStatus.Message())
	}
	if grpcStatus.Message() == "database password leaked" {
		t.Fatal("plain error was exposed")
	}
}

func TestResourceIncludesResourceInfo(t *testing.T) {
	grpcStatus := status.Convert(Resource(codes.NotFound, "USER_NOT_FOUND", "user not found", "user", "user-123"))
	for _, detail := range grpcStatus.Details() {
		if resource, ok := detail.(*errdetails.ResourceInfo); ok {
			if resource.GetResourceType() != "user" || resource.GetResourceName() != "user-123" {
				t.Fatalf("resource info = %#v", resource)
			}
			return
		}
	}
	t.Fatal("ResourceInfo detail missing")
}
