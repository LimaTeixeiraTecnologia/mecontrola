//go:build integration

package idempotency_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type PostgresStorageIntegrationSuite struct {
	suite.Suite
}

func TestPostgresStorageIntegration(t *testing.T) {
	suite.Run(t, new(PostgresStorageIntegrationSuite))
}

func (s *PostgresStorageIntegrationSuite) TestRace10GoroutinesSameKey() {
	ctx := context.Background()
	db, _ := testcontainer.Postgres(s.T())

	storage := idempotency.NewPostgresStorage(db)

	userID := uuid.NewString()
	key := "race-key-" + uuid.NewString()
	scope := "card"
	requestHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	responseBody := []byte(`{"id":"card-race-1"}`)
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	type result struct {
		err error
	}
	results := make([]result, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			rec := idempotency.Record{
				Scope:          scope,
				Key:            key,
				UserID:         userID,
				RequestHash:    requestHash,
				ResponseStatus: 201,
				ResponseBody:   responseBody,
				ExpiresAt:      expiresAt,
			}
			results[idx].err = storage.Put(ctx, rec)
		}(i)
	}

	wg.Wait()

	errCount := 0
	for _, r := range results {
		if r.err != nil {
			errCount++
		}
	}
	s.Zero(errCount, "all puts should succeed (same hash = idempotent)")

	var rows []idempotency.Record
	got, err := storage.Get(ctx, scope, key, userID)
	s.NoError(err)
	rows = append(rows, got)

	s.Len(rows, 1, "must have exactly 1 row in idempotency_keys")
	s.Equal(requestHash, rows[0].RequestHash)
	s.Equal(responseBody, rows[0].ResponseBody)
}

func (s *PostgresStorageIntegrationSuite) TestExpiredKeyTreatedAsMiss() {
	ctx := context.Background()
	db, _ := testcontainer.Postgres(s.T())

	storage := idempotency.NewPostgresStorage(db)

	userID := uuid.NewString()
	key := "expired-key-" + uuid.NewString()
	scope := "card"
	requestHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	_, err := db.ExecContext(ctx, `
		INSERT INTO mecontrola.idempotency_keys
			(scope, key, user_id, request_hash, response_status, response_body, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		scope, key, userID, requestHash, 201, []byte(`{}`), expiredAt,
	)
	s.Require().NoError(err)

	_, getErr := storage.Get(ctx, scope, key, userID)
	s.ErrorIs(getErr, idempotency.ErrNotFound, "expired key must be treated as miss")
}

func (s *PostgresStorageIntegrationSuite) TestHashMismatchReturnsError() {
	ctx := context.Background()
	db, _ := testcontainer.Postgres(s.T())

	storage := idempotency.NewPostgresStorage(db)

	userID := uuid.NewString()
	key := "mismatch-key-" + uuid.NewString()
	scope := "card"

	hash1 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hash2 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	rec1 := idempotency.Record{
		Scope:          scope,
		Key:            key,
		UserID:         userID,
		RequestHash:    hash1,
		ResponseStatus: 201,
		ResponseBody:   []byte(`{"id":"1"}`),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	s.Require().NoError(storage.Put(ctx, rec1))

	rec2 := idempotency.Record{
		Scope:          scope,
		Key:            key,
		UserID:         userID,
		RequestHash:    hash2,
		ResponseStatus: 201,
		ResponseBody:   []byte(`{"id":"2"}`),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	err := storage.Put(ctx, rec2)
	s.ErrorIs(err, idempotency.ErrHashMismatch)
}
