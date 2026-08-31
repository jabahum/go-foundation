package interceptor

import (
	"context"
	"testing"

	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidationUnaryReturnsAllFieldViolations(t *testing.T) {
	called := false
	_, err := ValidationUnary()(
		context.Background(),
		&userv1.CreateUserRequest{Name: "   ", Email: "not-an-email", Password: "short"},
		&grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)
	if called {
		t.Fatal("handler was called for an invalid request")
	}
	grpcStatus := status.Convert(err)
	if grpcStatus.Code() != codes.InvalidArgument {
		t.Fatalf("code = %s", grpcStatus.Code())
	}

	fields := make(map[string]bool)
	for _, detail := range grpcStatus.Details() {
		badRequest, ok := detail.(*errdetails.BadRequest)
		if !ok {
			continue
		}
		for _, violation := range badRequest.GetFieldViolations() {
			fields[violation.GetField()] = true
		}
	}
	for _, field := range []string{"name", "email", "password"} {
		if !fields[field] {
			t.Errorf("missing violation for %q: %v", field, fields)
		}
	}
}

func TestValidationUnaryPassesValidRequest(t *testing.T) {
	called := false
	_, err := ValidationUnary()(
		context.Background(),
		&userv1.CreateUserRequest{Name: "Ada", Email: "ada@example.com", Password: "StrongPass123!"},
		&grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/CreateUser"},
		func(context.Context, any) (any, error) {
			called = true
			return &userv1.CreateUserResponse{}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler was not called for a valid request")
	}
}
