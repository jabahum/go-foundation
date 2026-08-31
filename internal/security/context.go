package security

import (
	"context"

	auth "github.com/jabahum/go-foundation/internal/domain/auth"
)

type contextKey string

const identityKey contextKey = "identity"

func WithIdentity(ctx context.Context, identity *auth.Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}
func IdentityFromContext(ctx context.Context) (*auth.Identity, bool) {
	v, ok := ctx.Value(identityKey).(*auth.Identity)
	return v, ok
}
