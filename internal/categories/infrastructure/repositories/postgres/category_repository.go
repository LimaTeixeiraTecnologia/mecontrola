package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

var ErrCategoryNotFound = fmt.Errorf("categories: category not found: %w", interfaces.ErrNotFound)

type categoryRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewCategoryRepository(o11y observability.Observability, db database.DBTX) interfaces.CategoryRepository {
	return &categoryRepository{o11y: o11y, db: db}
}

func (r *categoryRepository) List(ctx context.Context, q interfaces.CategoryQuery) (categories []entities.Category, err error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.category.list")
	defer span.End()

	query := r.buildListQuery(q)
	rows, qerr := r.db.QueryContext(ctx, query.sql, query.args...)
	if qerr != nil {
		span.RecordError(qerr)
		return nil, fmt.Errorf("categories/postgres: list: %w", qerr)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("categories/postgres: close list rows: %w", cerr))
		}
	}()

	return r.scanCategories(rows)
}

func (r *categoryRepository) ListByIDs(ctx context.Context, ids []uuid.UUID) (categories []entities.Category, err error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.category.list_by_ids")
	defer span.End()

	if len(ids) == 0 {
		return nil, nil
	}

	query, args := buildListByIDsQuery(ids)

	rows, qerr := r.db.QueryContext(ctx, query, args...)
	if qerr != nil {
		span.RecordError(qerr)
		return nil, fmt.Errorf("categories/postgres: list_by_ids: %w", qerr)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("categories/postgres: close list_by_ids rows: %w", cerr))
		}
	}()

	return r.scanCategories(rows)
}

func buildListByIDsQuery(ids []uuid.UUID) (string, []any) {
	args := make([]any, 0, len(ids))
	var placeholders strings.Builder
	for i, id := range ids {
		if i > 0 {
			placeholders.WriteString(", ")
		}
		fmt.Fprintf(&placeholders, "$%d", i+1)
		args = append(args, id)
	}
	query := `
		SELECT id, slug, name, kind, parent_id, allocation_type, deprecated_at
		FROM mecontrola.categories
		WHERE id IN (` + placeholders.String() + `)
	`
	return query, args
}

func (r *categoryRepository) GetByID(ctx context.Context, id uuid.UUID) (entities.Category, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.category.get_by_id")
	defer span.End()

	const query = `
		SELECT id, slug, name, kind, parent_id, allocation_type, deprecated_at
		FROM mecontrola.categories
		WHERE id = $1
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanCategory(row)
}

type listQuery struct {
	sql  string
	args []any
}

func (r *categoryRepository) buildListQuery(q interfaces.CategoryQuery) listQuery {
	var args []any
	argIdx := 0

	sql := `
		SELECT id, slug, name, kind, parent_id, allocation_type, deprecated_at
		FROM mecontrola.categories
		WHERE 1=1
	`

	if q.Kind.IsValid() {
		argIdx++
		sql += fmt.Sprintf(" AND kind = $%d", argIdx)
		args = append(args, q.Kind)
	}

	if q.ParentID != nil {
		argIdx++
		sql += fmt.Sprintf(" AND parent_id = $%d", argIdx)
		args = append(args, *q.ParentID)
	}

	if !q.IncludeDeprecated {
		sql += " AND deprecated_at IS NULL"
	}

	sql += ` ORDER BY name COLLATE "pt-BR-x-icu"`

	return listQuery{sql: sql, args: args}
}

func (r *categoryRepository) scanCategories(rows database.Rows) ([]entities.Category, error) {
	var categories []entities.Category

	for rows.Next() {
		c, err := r.scanCategoryFromRows(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories/postgres: rows iteration: %w", err)
	}

	return categories, nil
}

func (r *categoryRepository) scanCategoryFromRows(rows database.Rows) (entities.Category, error) {
	var c entities.Category
	var kindStr, allocationTypeStr string
	var parentID uuid.NullUUID
	var deprecatedAt sql.NullTime

	err := rows.Scan(
		&c.ID,
		&c.Slug,
		&c.Name,
		&kindStr,
		&parentID,
		&allocationTypeStr,
		&deprecatedAt,
	)
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: scan category: %w", err)
	}

	kind, err := valueobjects.ParseKind(kindStr)
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: parse kind: %w", err)
	}
	c.Kind = kind

	allocationType, err := valueobjects.ParseAllocationType(allocationTypeStr)
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: parse allocation_type: %w", err)
	}
	c.AllocationType = allocationType

	if parentID.Valid {
		c.ParentID = &parentID.UUID
	}
	if deprecatedAt.Valid {
		c.DeprecatedAt = &deprecatedAt.Time
	}

	return c, nil
}

func (r *categoryRepository) scanCategory(row database.Row) (entities.Category, error) {
	var c entities.Category
	var kindStr, allocationTypeStr string
	var parentID uuid.NullUUID
	var deprecatedAt sql.NullTime

	err := row.Scan(
		&c.ID,
		&c.Slug,
		&c.Name,
		&kindStr,
		&parentID,
		&allocationTypeStr,
		&deprecatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Category{}, ErrCategoryNotFound
	}
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: scan category: %w", err)
	}

	kind, err := valueobjects.ParseKind(kindStr)
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: parse kind: %w", err)
	}
	c.Kind = kind

	allocationType, err := valueobjects.ParseAllocationType(allocationTypeStr)
	if err != nil {
		return entities.Category{}, fmt.Errorf("categories/postgres: parse allocation_type: %w", err)
	}
	c.AllocationType = allocationType

	if parentID.Valid {
		c.ParentID = &parentID.UUID
	}
	if deprecatedAt.Valid {
		c.DeprecatedAt = &deprecatedAt.Time
	}

	return c, nil
}
