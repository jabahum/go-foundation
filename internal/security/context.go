package security

import (
	"context"
	domainauth "github.com/jabahum/go-foundation/internal/domain/auth"
)

type contextKey string

const identityKey contextKey = "identity"

func WithIdentity(ctx context.Context, identity *domainauth.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}
func IdentityFromContext(ctx context.Context) (*domainauth.Identity, bool) {
	v, ok := ctx.Value(identityKey).(*domainauth.Identity)
	return v, ok
}
