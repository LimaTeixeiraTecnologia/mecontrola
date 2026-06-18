//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewProductionConnectionPath(t *testing.T) {
	_, dsn := NewTestDatabase(t)

	db, err := New(dsn,
		WithMaxOpenConns(5),
		WithMaxIdleConns(2),
		WithConnMaxLifetime(5*time.Minute),
		WithConnMaxIdleTime(2*time.Minute),
	)
	require.NoError(t, err)
	require.NotNil(t, db.DB())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	require.NoError(t, db.Ping(ctx))

	var one int
	require.NoError(t, db.DB().QueryRowContext(ctx, "SELECT 1").Scan(&one))
	require.Equal(t, 1, one)

	require.NoError(t, db.Shutdown(ctx))
	require.NoError(t, db.Shutdown(ctx))
	require.Nil(t, db.DB())
	require.Error(t, db.Ping(ctx))
}

func TestNewFailFastOnUnreachable(t *testing.T) {
	_, err := New("postgres://nouser:nopass@127.0.0.1:1/nodb?sslmode=disable")
	require.Error(t, err)
}

func TestNewRejectsEmptyURI(t *testing.T) {
	_, err := New("")
	require.Error(t, err)
}
