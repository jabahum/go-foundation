package auth

import (
	"context"
	"errors"

	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
	user "github.com/jabahum/go-foundation/internal/domain/user"
)

var ErrInvalidToken = errors.New("invalid access token")

type AuthenticationService struct {
	verifiers []domainauth.TokenVerifier
	users     user.Repository
}

func NewAuthenticationService(users user.Repository, v ...domainauth.TokenVerifier) *AuthenticationService {
	return &AuthenticationService{users: users, verifiers: v}
}
func (s *AuthenticationService) Authenticate(ctx context.Context, raw string) (*domainauth.Identity, error) {
	for _, v := range s.verifiers {
		id, err := v.Verify(ctx, raw)
		if err != nil {
			continue
		}
		if id.Provider == "local" {
			u, err := s.users.GetByID(ctx, id.UserID)
			if err != nil || !u.Enabled {
				return nil, ErrInvalidToken
			}
			id.Username = u.Name
			id.Email = u.Email
			return id, nil
		}
		u, err := s.users.GetByExternalIdentity(ctx, id.Provider, id.UserID)
		if err != nil || !u.Enabled {
			return nil, ErrInvalidToken
		}
		return &domainauth.Identity{UserID: u.ID, Username: u.Name, Email: u.Email, Provider: id.Provider}, nil
	}
	return nil, ErrInvalidToken
}
