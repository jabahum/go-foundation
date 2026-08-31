package auth

import (
	"context"
	"errors"
	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
	domainuser "github.com/jabahum/go-foundation/internal/domain/user"
	"strings"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	users  domainuser.Repository
	hasher domainauth.PasswordHasher
	issuer domainauth.TokenIssuer
}
type LoginResult struct {
	User  *domainuser.User
	Token *domainauth.Token
}

func NewService(u domainuser.Repository, h domainauth.PasswordHasher, i domainauth.TokenIssuer) *Service {
	return &Service{users: u, hasher: h, issuer: i}
}
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !u.Enabled {
		return nil, domainuser.ErrDisabled
	}
	if u.AuthProvider != "local" || u.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	ok, err := s.hasher.Verify(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}
	tok, err := s.issuer.Issue(ctx, domainauth.Identity{UserID: u.ID, Username: u.Name, Email: u.Email, Provider: "local"})
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Token: tok}, nil
}
