package requestid

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

// New generates a request identifier.
func New() string {
	return uuid.NewString()
}

// With stores the request id on ctx.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the request id on ctx, or empty string.
func From(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
