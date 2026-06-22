//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func setupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, _ := testcontainer.Postgres(t)
	return db
}

func insertTestUser(t *testing.T, db *sqlx.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	number := fmt.Sprintf("+5511%09d", time.Now().UnixNano()%1000000000)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, number,
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}
