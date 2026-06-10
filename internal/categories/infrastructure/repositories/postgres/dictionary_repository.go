package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
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

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

type dictionaryRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewDictionaryRepository(o11y observability.Observability, db database.DBTX) interfaces.DictionaryRepository {
	return &dictionaryRepository{o11y: o11y, db: db}
}

func (r *dictionaryRepository) List(ctx context.Context, q interfaces.DictionaryQuery) (entries []entities.DictionaryEntry, nextCursor string, err error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.dictionary.list")
	defer span.End()

	pageSize := q.PageSize
	if pageSize <= 0 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	query, args := r.buildListQuery(q, pageSize)
	rows, qerr := r.db.QueryContext(ctx, query, args...)
	if qerr != nil {
		span.RecordError(qerr)
		return nil, "", fmt.Errorf("categories/postgres: dictionary list: %w", qerr)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("categories/postgres: close list rows: %w", cerr))
		}
	}()

	entries, err = r.scanEntries(rows)
	if err != nil {
		return nil, "", err
	}

	if len(entries) == 0 {
		return entries, "", nil
	}

	if len(entries) > pageSize {
		entries = entries[:pageSize]
		last := entries[len(entries)-1]
		nextCursor = encodeCursor(last)
	}

	return entries, nextCursor, nil
}

func (r *dictionaryRepository) Search(ctx context.Context, q interfaces.DictionarySearchQuery) (entries []entities.DictionaryEntry, err error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "categories.repository.dictionary.search")
	defer span.End()

	const query = `
		SELECT id, category_id, kind, term, signal_type, confidence, is_ambiguous, deprecated_at
		FROM mecontrola.category_dictionary
		WHERE kind = $1
		  AND term_normalized = lower(mecontrola.immutable_unaccent($2))
		  AND deprecated_at IS NULL
		ORDER BY
			CASE signal_type
				WHEN 'canonical_name' THEN 1
				WHEN 'alias' THEN 2
				WHEN 'phrase' THEN 3
				WHEN 'merchant' THEN 4
				WHEN 'segment' THEN 5
			END,
			term COLLATE "pt-BR-x-icu"
		LIMIT $3
	`

	rows, qerr := r.db.QueryContext(ctx, query, q.Kind.String(), q.Term, q.Limit)
	if qerr != nil {
		span.RecordError(qerr)
		return nil, fmt.Errorf("categories/postgres: dictionary search: %w", qerr)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("categories/postgres: close search rows: %w", cerr))
		}
	}()

	return r.scanEntries(rows)
}

func (r *dictionaryRepository) buildListQuery(q interfaces.DictionaryQuery, pageSize int) (string, []any) {
	args := []any{}
	argIdx := 0

	sql := `
		SELECT id, category_id, kind, term, signal_type, confidence, is_ambiguous, deprecated_at
		FROM mecontrola.category_dictionary
		WHERE deprecated_at IS NULL
	`

	if q.Kind != nil {
		argIdx++
		sql += fmt.Sprintf(" AND kind = $%d", argIdx)
		args = append(args, q.Kind.String())
	}

	if q.CategoryID != nil {
		argIdx++
		sql += fmt.Sprintf(" AND category_id = $%d", argIdx)
		args = append(args, *q.CategoryID)
	}

	if q.SignalType != nil {
		argIdx++
		sql += fmt.Sprintf(" AND signal_type = $%d", argIdx)
		args = append(args, q.SignalType.String())
	}

	if q.Cursor != "" {
		termNormalized, id, ok := decodeCursor(q.Cursor)
		if ok {
			argIdx += 2
			sql += fmt.Sprintf(" AND (term_normalized, id) > ($%d, $%d)", argIdx-1, argIdx)
			args = append(args, termNormalized, id)
		}
	}

	argIdx++
	sql += fmt.Sprintf(` ORDER BY term_normalized COLLATE "pt-BR-x-icu", id LIMIT $%d`, argIdx)
	args = append(args, pageSize+1)

	return sql, args
}

func (r *dictionaryRepository) scanEntries(rows database.Rows) ([]entities.DictionaryEntry, error) {
	var entries []entities.DictionaryEntry

	for rows.Next() {
		e, err := r.scanEntryFromRows(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("categories/postgres: rows iteration: %w", err)
	}

	return entries, nil
}

func (r *dictionaryRepository) scanEntryFromRows(rows database.Rows) (entities.DictionaryEntry, error) {
	var e entities.DictionaryEntry
	var kindStr, signalTypeStr, confidenceStr string
	var deprecatedAt sql.NullTime

	err := rows.Scan(
		&e.ID,
		&e.CategoryID,
		&kindStr,
		&e.Term,
		&signalTypeStr,
		&confidenceStr,
		&e.IsAmbiguous,
		&deprecatedAt,
	)
	if err != nil {
		return entities.DictionaryEntry{}, fmt.Errorf("categories/postgres: scan entry: %w", err)
	}

	kind, err := valueobjects.ParseKind(kindStr)
	if err != nil {
		return entities.DictionaryEntry{}, fmt.Errorf("categories/postgres: parse kind: %w", err)
	}
	e.Kind = kind

	signalType, err := valueobjects.ParseSignalType(signalTypeStr)
	if err != nil {
		return entities.DictionaryEntry{}, fmt.Errorf("categories/postgres: parse signal_type: %w", err)
	}
	e.SignalType = signalType

	confidence, err := valueobjects.ParseConfidence(confidenceStr)
	if err != nil {
		return entities.DictionaryEntry{}, fmt.Errorf("categories/postgres: parse confidence: %w", err)
	}
	e.Confidence = confidence

	if deprecatedAt.Valid {
		e.DeprecatedAt = &deprecatedAt.Time
	}

	return e, nil
}

func encodeCursor(e entities.DictionaryEntry) string {
	data := fmt.Sprintf("%s|%s", strings.ToLower(e.Term), e.ID.String())
	return base64.URLEncoding.EncodeToString([]byte(data))
}

func decodeCursor(cursor string) (string, uuid.UUID, bool) {
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", uuid.UUID{}, false
	}

	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return "", uuid.UUID{}, false
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return "", uuid.UUID{}, false
	}

	return parts[0], id, true
}
