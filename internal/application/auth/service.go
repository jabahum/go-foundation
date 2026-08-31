package auth

import (
	"context"
	"errors"
	"strings"

	auth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Service struct {
	users  user.Repository
	hasher auth.PasswordHasher
	issuer auth.TokenIssuer
}
type LoginResult struct {
	User  *user.User
	Token *auth.Token
}

func NewService(u user.Repository, h auth.PasswordHasher, i auth.TokenIssuer) *Service {
	return &Service{users: u, hasher: h, issuer: i}
}
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !u.Enabled {
		return nil, user.ErrDisabled
	}
	if u.AuthProvider != "local" || u.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	ok, err := s.hasher.Verify(password, u.PasswordHash)
	if err != nil || !ok {
		return nil, ErrInvalidCredentials
	}
	tok, err := s.issuer.Issue(ctx, auth.Identity{UserID: u.ID, Username: u.Name, Email: u.Email, Provider: "local"})
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: u, Token: tok}, nil
}
