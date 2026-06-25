package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type PrincipalSource string

const (
	SourceWhatsApp PrincipalSource = "whatsapp"
	SourceHeader   PrincipalSource = "header"
)

type Principal struct {
	UserID uuid.UUID
	Source PrincipalSource
}

func (p Principal) IsZero() bool {
	return p.UserID == uuid.Nil
}

type principalCtxKey struct{}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	if !ok || p.IsZero() {
		return Principal{}, false
	}
	return p, true
}

var ErrSourceFromChannelUnknown = errors.New("identity: unknown channel for principal source")

func SourceFromChannel(channel string) (PrincipalSource, error) {
	switch channel {
	case string(SourceWhatsApp):
		return SourceWhatsApp, nil
	default:
		return SourceWhatsApp, fmt.Errorf("identity: %q: %w", channel, ErrSourceFromChannelUnknown)
	}
}
