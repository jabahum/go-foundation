package grpc

import (
	"context"
	"errors"
	"net"

	authv1 "github.com/jabahum/go-foundation/gen/proto/auth/v1"
	appauth "github.com/jabahum/go-foundation/internal/application/auth"
	apprbac "github.com/jabahum/go-foundation/internal/application/rbac"
	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
	"github.com/jabahum/go-foundation/internal/security"
	"github.com/jabahum/go-foundation/internal/transport/grpc/apierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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
	res, err := h.auth.Login(ctx, r.GetEmail(), r.GetPassword(), authClientInfo(ctx))
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidCredentials) {
			return nil, apierror.New(codes.Unauthenticated, "INVALID_CREDENTIALS", "invalid credentials")
		}
		if errors.Is(err, user.ErrDisabled) {
			return nil, apierror.New(codes.PermissionDenied, "USER_DISABLED", "user disabled")
		}
		return nil, apierror.New(codes.Internal, "LOGIN_FAILED", "login failed")
	}
	return &authv1.LoginResponse{
		AccessToken:          res.Token.AccessToken,
		ExpiresAtUnix:        res.Token.ExpiresAt.Unix(),
		RefreshToken:         res.RefreshToken,
		RefreshExpiresAtUnix: res.Session.ExpiresAt.Unix(),
		SessionId:            res.Session.ID,
		User:                 &authv1.UserIdentity{Id: res.User.ID, Name: res.User.Name, Email: res.User.Email, Provider: "local"},
	}, nil
}

func (h *AuthHandler) Refresh(ctx context.Context, request *authv1.RefreshRequest) (*authv1.RefreshResponse, error) {
	if h.auth == nil {
		return nil, apierror.New(codes.Unimplemented, "LOCAL_AUTH_DISABLED", "local authentication is disabled")
	}
	result, err := h.auth.Refresh(ctx, request.GetRefreshToken())
	if err != nil {
		if errors.Is(err, appauth.ErrInvalidRefreshToken) {
			return nil, apierror.New(codes.Unauthenticated, "REFRESH_TOKEN_INVALID", "invalid refresh token")
		}
		return nil, apierror.New(codes.Internal, "TOKEN_REFRESH_FAILED", "token refresh failed")
	}
	return &authv1.RefreshResponse{
		AccessToken:          result.Token.AccessToken,
		ExpiresAtUnix:        result.Token.ExpiresAt.Unix(),
		RefreshToken:         result.RefreshToken,
		RefreshExpiresAtUnix: result.Session.ExpiresAt.Unix(),
		SessionId:            result.Session.ID,
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, request *authv1.LogoutRequest) (*authv1.LogoutResponse, error) {
	if h.auth == nil {
		return nil, apierror.New(codes.Unimplemented, "LOCAL_AUTH_DISABLED", "local authentication is disabled")
	}
	if err := h.auth.Logout(ctx, request.GetRefreshToken()); err != nil {
		return nil, apierror.New(codes.Internal, "LOGOUT_FAILED", "logout failed")
	}
	return &authv1.LogoutResponse{}, nil
}

func (h *AuthHandler) ListSessions(ctx context.Context, _ *authv1.ListSessionsRequest) (*authv1.ListSessionsResponse, error) {
	if h.auth == nil {
		return nil, apierror.New(codes.Unimplemented, "LOCAL_AUTH_DISABLED", "local authentication is disabled")
	}
	identity, ok := security.IdentityFromContext(ctx)
	if !ok {
		return nil, apierror.New(codes.Unauthenticated, "AUTHENTICATION_REQUIRED", "authentication required")
	}
	sessions, err := h.auth.ListSessions(ctx, identity.UserID)
	if err != nil {
		return nil, apierror.New(codes.Internal, "SESSION_LIST_FAILED", "list sessions failed")
	}
	response := &authv1.ListSessionsResponse{Sessions: make([]*authv1.Session, 0, len(sessions))}
	for _, session := range sessions {
		value := &authv1.Session{
			Id:            session.ID,
			UserAgent:     session.UserAgent,
			ClientIp:      session.ClientIP,
			CreatedAtUnix: session.CreatedAt.Unix(),
			ExpiresAtUnix: session.ExpiresAt.Unix(),
			Current:       session.ID == identity.SessionID,
		}
		if session.LastUsedAt != nil {
			value.LastUsedAtUnix = session.LastUsedAt.Unix()
		}
		response.Sessions = append(response.Sessions, value)
	}
	return response, nil
}

func (h *AuthHandler) RevokeSession(ctx context.Context, request *authv1.RevokeSessionRequest) (*authv1.RevokeSessionResponse, error) {
	if h.auth == nil {
		return nil, apierror.New(codes.Unimplemented, "LOCAL_AUTH_DISABLED", "local authentication is disabled")
	}
	identity, ok := security.IdentityFromContext(ctx)
	if !ok {
		return nil, apierror.New(codes.Unauthenticated, "AUTHENTICATION_REQUIRED", "authentication required")
	}
	err := h.auth.RevokeSession(ctx, identity.UserID, request.GetSessionId())
	if err != nil {
		if errors.Is(err, domainauth.ErrSessionNotFound) {
			return nil, apierror.Resource(codes.NotFound, "SESSION_NOT_FOUND", "session not found", "session", request.GetSessionId())
		}
		return nil, apierror.New(codes.Internal, "SESSION_REVOCATION_FAILED", "revoke session failed")
	}
	return &authv1.RevokeSessionResponse{}, nil
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

func authClientInfo(ctx context.Context) appauth.ClientInfo {
	client := appauth.ClientInfo{}
	if remote, ok := peer.FromContext(ctx); ok && remote.Addr != nil {
		client.ClientIP = remote.Addr.String()
		if host, _, err := net.SplitHostPort(client.ClientIP); err == nil {
			client.ClientIP = host
		}
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get("user-agent"); len(values) > 0 {
			client.UserAgent = values[0]
		}
	}
	return client
}
