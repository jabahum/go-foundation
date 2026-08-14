package local

import (
	"context"
	"crypto/rsa"
	domainauth "example.com/grpc-clean-starter/internal/domain/auth"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"os"
	"time"
)

type JWTProvider struct {
	privateKey       *rsa.PrivateKey
	publicKey        *rsa.PublicKey
	issuer, audience string
	ttl              time.Duration
}
type claims struct {
	Username string `json:"preferred_username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

func NewJWTProvider(privateFile, publicFile, issuer, audience string, ttl time.Duration) (*JWTProvider, error) {
	privPEM, err := os.ReadFile(privateFile)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	pubPEM, err := os.ReadFile(publicFile)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(privPEM)
	if err != nil {
		return nil, err
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, err
	}
	return &JWTProvider{privateKey: priv, publicKey: pub, issuer: issuer, audience: audience, ttl: ttl}, nil
}
func (p *JWTProvider) Issue(ctx context.Context, id domainauth.Identity) (*domainauth.Token, error) {
	now := time.Now().UTC()
	exp := now.Add(p.ttl)
	c := claims{Username: id.Username, Email: id.Email, RegisteredClaims: jwt.RegisteredClaims{Issuer: p.issuer, Subject: id.UserID, Audience: jwt.ClaimStrings{p.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp)}}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	raw, err := t.SignedString(p.privateKey)
	if err != nil {
		return nil, err
	}
	return &domainauth.Token{AccessToken: raw, ExpiresAt: exp}, nil
}
func (p *JWTProvider) Verify(ctx context.Context, raw string) (*domainauth.Identity, error) {
	parsed, err := jwt.ParseWithClaims(raw, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return p.publicKey, nil
	}, jwt.WithIssuer(p.issuer), jwt.WithAudience(p.audience), jwt.WithExpirationRequired())
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	c, ok := parsed.Claims.(*claims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	return &domainauth.Identity{UserID: c.Subject, Username: c.Username, Email: c.Email, Provider: "local"}, nil
}
