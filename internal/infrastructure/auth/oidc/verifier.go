package oidc

import (
	"context"
	"fmt"
	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
)

type Verifier struct{ verifier *coreoidc.IDTokenVerifier }

func New(ctx context.Context, issuer, clientID string) (*Verifier, error) {
	provider, err := coreoidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}
	return &Verifier{verifier: provider.Verifier(&coreoidc.Config{ClientID: clientID})}, nil
}
func (v *Verifier) Verify(ctx context.Context, raw string) (*domainauth.Identity, error) {
	t, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	var c struct {
		Email    string `json:"email"`
		Username string `json:"preferred_username"`
	}
	if err := t.Claims(&c); err != nil {
		return nil, fmt.Errorf("parse oidc claims: %w", err)
	}
	return &domainauth.Identity{UserID: t.Subject, Username: c.Username, Email: c.Email, Provider: "oidc"}, nil
}
