package grpc

import (
	"context"
	"errors"
	userv1 "github.com/jabahum/go-foundation/gen/proto/user/v1"
	appuser "github.com/jabahum/go-foundation/internal/application/user"
	domainuser "github.com/jabahum/go-foundation/internal/domain/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		return nil, mapUserErr(err)
	}
	return &userv1.CreateUserResponse{User: toProtoUser(u)}, nil
}
func (h *UserHandler) GetUser(ctx context.Context, r *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	u, err := h.svc.Get(ctx, r.GetId())
	if err != nil {
		return nil, mapUserErr(err)
	}
	return &userv1.GetUserResponse{User: toProtoUser(u)}, nil
}
func (h *UserHandler) ListUsers(ctx context.Context, r *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	res, err := h.svc.List(ctx, appuser.ListInput{PageSize: int(r.GetPageSize()), PageToken: r.GetPageToken(), Search: r.GetSearch()})
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	out := make([]*userv1.User, 0, len(res.Users))
	for _, u := range res.Users {
		out = append(out, toProtoUser(u))
	}
	return &userv1.ListUsersResponse{Users: out, NextPageToken: res.NextPageToken}, nil
}
func toProtoUser(u *domainuser.User) *userv1.User {
	return &userv1.User{Id: u.ID, Name: u.Name, Email: u.Email, Enabled: u.Enabled, CreatedAt: timestamppb.New(u.CreatedAt), UpdatedAt: timestamppb.New(u.UpdatedAt)}
}
func mapUserErr(err error) error {
	switch {
	case errors.Is(err, domainuser.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domainuser.ErrEmailExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domainuser.ErrInvalidUserID):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.InvalidArgument, err.Error())
	}
}
