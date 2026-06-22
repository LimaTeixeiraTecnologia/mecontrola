package postgres

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func nullableUUID(id uuid.UUID, present bool) any {
	if !present || id == uuid.Nil {
		return nil
	}
	return id
}

func nullableTime(value time.Time, present bool) sql.NullTime {
	if !present || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}
