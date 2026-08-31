package grpc

import (
	"context"
	"errors"

	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	appuser "github.com/jabahum/go-foundation/internal/application/user"
	user "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	svc *appuser.Service
}

func NewUserHandler(s *appuser.Service) *UserHandler { return &UserHandler{svc: s} }
func (h *UserHandler) CreateUser(ctx context.Context, r *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	u, err := h.svc.Create(ctx, appuser.CreateInput{Name: r.GetName(), Email: r.GetEmail(), Password: r.GetPassword()})
	if err != nil {
		return nil, mapUserErr(err, "email", r.GetEmail())
	}
	return &userv1.CreateUserResponse{User: toProtoUser(u)}, nil
}
func (h *UserHandler) GetUser(ctx context.Context, r *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	u, err := h.svc.Get(ctx, r.GetId())
	if err != nil {
		return nil, mapUserErr(err, "id", r.GetId())
	}
	return &userv1.GetUserResponse{User: toProtoUser(u)}, nil
}
func (h *UserHandler) ListUsers(ctx context.Context, r *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	res, err := h.svc.List(ctx, appuser.ListInput{PageSize: int(r.GetPageSize()), PageToken: r.GetPageToken(), Search: r.GetSearch()})
	if err != nil {
		return nil, mapUserErr(err, "page_token", r.GetPageToken())
	}
	out := make([]*userv1.User, 0, len(res.Users))
	for _, u := range res.Users {
		out = append(out, toProtoUser(u))
	}
	return &userv1.ListUsersResponse{Users: out, NextPageToken: res.NextPageToken}, nil
}
func toProtoUser(u *user.User) *userv1.User {
	return &userv1.User{Id: u.ID, Name: u.Name, Email: u.Email, Enabled: u.Enabled, CreatedAt: timestamppb.New(u.CreatedAt), UpdatedAt: timestamppb.New(u.UpdatedAt)}
}
func mapUserErr(err error, field, resourceName string) error {
	switch {
	case errors.Is(err, user.ErrNotFound):
		return apierror.Resource(codes.NotFound, "USER_NOT_FOUND", "user not found", "user", resourceName)
	case errors.Is(err, user.ErrEmailExists):
		return apierror.Resource(codes.AlreadyExists, "USER_EMAIL_EXISTS", "email already exists", "user", resourceName)
	case errors.Is(err, user.ErrInvalidUserID):
		return apierror.InvalidArgument("USER_ID_INVALID", "invalid user id", apierror.FieldViolation{Field: field, Description: "must be a valid UUID"})
	case errors.Is(err, appuser.ErrInvalidPageToken):
		return apierror.InvalidArgument("PAGE_TOKEN_INVALID", "invalid page token", apierror.FieldViolation{Field: field, Description: "must be a valid page token"})
	default:
		return apierror.New(codes.Internal, "USER_OPERATION_FAILED", "user operation failed")
	}
}
