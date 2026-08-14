package grpc

import (
	"context"
	"errors"
	authv1 "example.com/grpc-clean-starter/gen/proto/auth/v1"
	appauth "example.com/grpc-clean-starter/internal/application/auth"
	apprbac "example.com/grpc-clean-starter/internal/application/rbac"
	domainuser "example.com/grpc-clean-starter/internal/domain/user"
	"example.com/grpc-clean-starter/internal/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	auth  *appauth.Service
	users domainuser.Repository
	rbac  *apprbac.Service
}

func NewAuthHandler(a *appauth.Service, u domainuser.Repository, r *apprbac.Service) *AuthHandler {
	return &AuthHandler{auth: a, users: u, rbac: r}
}
func (h *AuthHandler) Login(ctx context.Context, r *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if h.auth == nil {
		return nil, status.Error(codes.Unimplemented, "local login is disabled")
	}
	res, err := h.auth.Login(ctx, r.GetEmail(), r.GetPassword())
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		if errors.Is(err, domainuser.ErrDisabled) {
			return nil, status.Error(codes.PermissionDenied, "user disabled")
		}
		return nil, status.Error(codes.Internal, "login failed")
	}
	return &authv1.LoginResponse{AccessToken: res.Token.AccessToken, ExpiresAtUnix: res.Token.ExpiresAt.Unix(), User: &authv1.UserIdentity{Id: res.User.ID, Name: res.User.Name, Email: res.User.Email, Provider: "local"}}, nil
}
func (h *AuthHandler) Me(ctx context.Context, r *authv1.MeRequest) (*authv1.MeResponse, error) {
	id, ok := security.IdentityFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	u, err := h.users.GetByID(ctx, id.UserID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	roles, err := h.rbac.Roles(ctx, u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "load roles failed")
	}
	perms, err := h.rbac.Permissions(ctx, u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "load permissions failed")
	}
	rn := make([]string, 0, len(roles))
	for _, x := range roles {
		rn = append(rn, x.Name)
	}
	pn := make([]string, 0, len(perms))
	for _, x := range perms {
		pn = append(pn, x.Name)
	}
	return &authv1.MeResponse{User: &authv1.UserIdentity{Id: u.ID, Name: u.Name, Email: u.Email, Provider: id.Provider}, Roles: rn, Permissions: pn}, nil
}
