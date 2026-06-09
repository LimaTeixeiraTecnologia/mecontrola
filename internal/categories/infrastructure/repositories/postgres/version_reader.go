package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
)

var ErrVersionNotFound = errors.New("categories: version not found")

type versionReader struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewVersionReader(o11y observability.Observability, db database.DBTX) interfaces.VersionReader {
	return &versionReader{o11y: o11y, db: db}
}

func (r *versionReader) Current(ctx context.Context) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.version.current")
	defer span.End()

	const query = `
		SELECT version
		FROM mecontrola.category_editorial_version
		LIMIT 1
	`

	var version int64
	err := r.db.QueryRowContext(ctx, query).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrVersionNotFound
	}
	if err != nil {
		span.RecordError(err)
		return 0, fmt.Errorf("categories/postgres: version current: %w", err)
	}

	return version, nil
}
