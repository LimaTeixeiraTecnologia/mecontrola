package idempotency

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("idempotency: record not found")
	ErrHashMismatch     = errors.New("idempotency: request hash mismatch")
	ErrResponseTooLarge = errors.New("idempotency: response body exceeds 64 KB limit")
)

type Record struct {
	Scope          string
	Key            string
	UserID         string
	RequestHash    string
	ResponseStatus int
	ResponseBody   []byte
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

type Storage interface {
	Get(ctx context.Context, scope, key, userID string) (Record, error)
	Put(ctx context.Context, rec Record) error
}
