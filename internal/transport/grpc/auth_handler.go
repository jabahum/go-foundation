package grpc

import (
	"context"
	"errors"

	authv1 "github.com/jabahum/go-foundation/gen/proto/auth/v1"
	appauth "github.com/jabahum/go-foundation/internal/application/auth"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	user "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc/codes"
)

type AuthHandler struct {
	authv1.UnimplementedAuthServiceServer
	auth  *appauth.Service
	users user.Repository
	rbac  *apprbac.Service
}

func NewAuthHandler(a *appauth.Service, u user.Repository, r *apprbac.Service) *AuthHandler {
	return &AuthHandler{auth: a, users: u, rbac: r}
}
func (h *AuthHandler) Login(ctx context.Context, r *authv1.LoginRequest) (*authv1.LoginResponse, error) {
	if h.auth == nil {
		return nil, apierror.New(codes.Unimplemented, "LOCAL_AUTH_DISABLED", "local login is disabled")
	}
	res, err := h.auth.Login(ctx, r.GetEmail(), r.GetPassword())
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			return nil, apierror.New(codes.Unauthenticated, "INVALID_CREDENTIALS", "invalid credentials")
		}
		if errors.Is(err, user.ErrDisabled) {
			return nil, apierror.New(codes.PermissionDenied, "USER_DISABLED", "user disabled")
		}
		return nil, apierror.New(codes.Internal, "LOGIN_FAILED", "login failed")
	}
	return &authv1.LoginResponse{AccessToken: res.Token.AccessToken, ExpiresAtUnix: res.Token.ExpiresAt.Unix(), User: &authv1.UserIdentity{Id: res.User.ID, Name: res.User.Name, Email: res.User.Email, Provider: "local"}}, nil
}
func (h *AuthHandler) Me(ctx context.Context, r *authv1.MeRequest) (*authv1.MeResponse, error) {
	id, ok := security.IdentityFromContext(ctx)
	if !ok {
		return nil, apierror.New(codes.Unauthenticated, "AUTHENTICATION_REQUIRED", "authentication required")
	}
	u, err := h.users.GetByID(ctx, id.UserID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return nil, apierror.Resource(codes.NotFound, "USER_NOT_FOUND", "user not found", "user", id.UserID)
		}
		return nil, apierror.New(codes.Internal, "USER_LOOKUP_FAILED", "load user failed")
	}
	roles, err := h.rbac.Roles(ctx, u.ID)
	if err != nil {
		return nil, apierror.New(codes.Internal, "ROLE_LOOKUP_FAILED", "load roles failed")
	}
	perms, err := h.rbac.Permissions(ctx, u.ID)
	if err != nil {
		return nil, apierror.New(codes.Internal, "PERMISSION_LOOKUP_FAILED", "load permissions failed")
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
