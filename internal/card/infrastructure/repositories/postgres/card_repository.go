package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

const prefixCardRepository = "card.repository.pg:"

type cardRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewCardRepository(o11y observability.Observability, db database.DBTX) interfaces.CardRepository {
	return &cardRepository{o11y: o11y, db: db}
}

func (r *cardRepository) Insert(ctx context.Context, c entities.Card) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`

	version := c.Version
	if version <= 0 {
		version = 1
	}

	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, query,
		c.ID,
		c.UserID,
		c.Name.String(),
		c.Nickname.String(),
		c.Cycle.ClosingDay,
		c.Cycle.DueDay,
		c.LimitCents,
		version,
		c.CreatedAt.UTC(),
		c.UpdatedAt.UTC(),
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "cards_user_nickname_active_uniq_idx" {
				return fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrNicknameConflict)
			}
		}
		return fmt.Errorf("%s insert: %w", prefixCardRepository, err)
	}
	return nil
}

func (r *cardRepository) GetByIDForUser(ctx context.Context, cardID, userID string) (entities.Card, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.get_by_id")
	defer span.End()

	const query = `
		SELECT id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
		  FROM mecontrola.cards
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var (
		id, name, nickname   string
		uid                  string
		closingDay, dueDay   int
		limitCents           int64
		version              int64
		createdAt, updatedAt time.Time
	)

	err := r.db.QueryRowContext(ctx, query, cardID, userID).
		Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &limitCents, &version, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.Card{}, fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrCardNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.get_by_id.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "get_by_id"),
			observability.Error(err),
		)
		return entities.Card{}, fmt.Errorf("%s get_by_id: %w", prefixCardRepository, err)
	}

	return r.hydrate(ctx, span, "get_by_id", id, uid, name, nickname, closingDay, dueDay, limitCents, version, createdAt, updatedAt, nil)
}

func (r *cardRepository) FindCardsWithInvoiceDueWithin(ctx context.Context, windowDays, limit int) ([]entities.Card, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.find_cards_with_invoice_due_within")
	defer span.End()

	if windowDays < 0 {
		windowDays = 0
	}
	if limit <= 0 {
		limit = 500
	}

	days := dueDayWindow(time.Now().UTC(), windowDays)

	const query = `
		SELECT id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
		  FROM mecontrola.cards
		 WHERE deleted_at IS NULL
		   AND due_day = ANY($1)
		 ORDER BY user_id, id
		 LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, days, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("%s find_cards_with_invoice_due_within: %w", prefixCardRepository, err)
	}
	defer func() { _ = rows.Close() }()

	cards := make([]entities.Card, 0, limit)
	for rows.Next() {
		var (
			id, uid, name, nickname string
			closingDay, dueDay      int
			limitCents              int64
			version                 int64
			createdAt, updatedAt    time.Time
		)
		if scanErr := rows.Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &limitCents, &version, &createdAt, &updatedAt); scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("%s find_cards_with_invoice_due_within scan: %w", prefixCardRepository, scanErr)
		}
		card, hydrateErr := r.hydrate(ctx, span, "find_cards_with_invoice_due_within", id, uid, name, nickname, closingDay, dueDay, limitCents, version, createdAt, updatedAt, nil)
		if hydrateErr != nil {
			return nil, hydrateErr
		}
		cards = append(cards, card)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("%s find_cards_with_invoice_due_within rows: %w", prefixCardRepository, rowsErr)
	}
	return cards, nil
}

func dueDayWindow(now time.Time, windowDays int) []int32 {
	seen := make(map[int32]struct{}, windowDays+1)
	out := make([]int32, 0, windowDays+1)
	for i := 0; i <= windowDays+1; i++ {
		day := int32(now.AddDate(0, 0, i).Day())
		if _, ok := seen[day]; ok {
			continue
		}
		seen[day] = struct{}{}
		out = append(out, day)
	}
	return out
}

func (r *cardRepository) ListByUser(ctx context.Context, userID, cursor string, limit int) ([]entities.Card, string, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.list_by_user")
	defer span.End()

	fetch := limit + 1

	var (
		rows *sql.Rows
		err  error
	)

	if cursor == "" {
		const query = `
			SELECT id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
			  FROM mecontrola.cards
			 WHERE user_id = $1 AND deleted_at IS NULL
			 ORDER BY created_at DESC, id DESC
			 LIMIT $2
		`
		rows, err = r.db.QueryContext(ctx, query, userID, fetch)
	} else {
		cp, decErr := pagination.C.Decode(cursor)
		if decErr != nil {
			return nil, "", fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrInvalidCursor)
		}
		const query = `
			SELECT id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
			  FROM mecontrola.cards
			 WHERE user_id = $1
			   AND deleted_at IS NULL
			   AND (created_at, id) < ($2, $3)
			 ORDER BY created_at DESC, id DESC
			 LIMIT $4
		`
		rows, err = r.db.QueryContext(ctx, query, userID, cp.CreatedAt.UTC(), cp.ID, fetch)
	}

	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.list_by_user.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "list_by_user"),
			observability.Error(err),
		)
		return nil, "", fmt.Errorf("%s list_by_user: %w", prefixCardRepository, err)
	}
	defer func() { _ = rows.Close() }()

	cards := make([]entities.Card, 0, fetch)
	for rows.Next() {
		var (
			id, uid, name, nickname string
			closingDay, dueDay      int
			limitCents              int64
			version                 int64
			createdAt, updatedAt    time.Time
		)
		if scanErr := rows.Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &limitCents, &version, &createdAt, &updatedAt); scanErr != nil {
			span.RecordError(scanErr)
			return nil, "", fmt.Errorf("%s list_by_user scan: %w", prefixCardRepository, scanErr)
		}
		card, hydrateErr := r.hydrate(ctx, span, "list_by_user", id, uid, name, nickname, closingDay, dueDay, limitCents, version, createdAt, updatedAt, nil)
		if hydrateErr != nil {
			return nil, "", hydrateErr
		}
		cards = append(cards, card)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", fmt.Errorf("%s list_by_user rows: %w", prefixCardRepository, rowsErr)
	}

	var nextCursor string
	if len(cards) > limit {
		cards = cards[:limit]
		last := cards[len(cards)-1]
		encoded, encErr := pagination.C.Encode(last.CreatedAt, last.ID.String())
		if encErr != nil {
			return nil, "", fmt.Errorf("%s list_by_user encode_cursor: %w", prefixCardRepository, encErr)
		}
		nextCursor = encoded
	}

	return cards, nextCursor, nil
}

func (r *cardRepository) UpdateByIDForUser(ctx context.Context, c entities.Card) (entities.Card, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.update")
	defer span.End()

	const query = `
		UPDATE mecontrola.cards
		   SET name        = $1,
		       nickname    = $2,
		       closing_day = $3,
		       due_day     = $4,
		       limit_cents = $5,
		       version     = version + 1,
		       updated_at  = $6
		 WHERE id = $7 AND user_id = $8 AND deleted_at IS NULL
		RETURNING id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
	`

	var (
		id, uid, name, nickname string
		closingDay, dueDay      int
		limitCents              int64
		version                 int64
		createdAt, updatedAt    time.Time
	)

	err := r.db.QueryRowContext(ctx, query,
		c.Name.String(),
		c.Nickname.String(),
		c.Cycle.ClosingDay,
		c.Cycle.DueDay,
		c.LimitCents,
		c.UpdatedAt.UTC(),
		c.ID,
		c.UserID,
	).Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &limitCents, &version, &createdAt, &updatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return entities.Card{}, fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrCardNotFound)
	}
	if err != nil {
		span.RecordError(err)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "cards_user_nickname_active_uniq_idx" {
				return entities.Card{}, fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrNicknameConflict)
			}
		}
		r.o11y.Logger().Error(ctx, "card.repository.pg.update.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "update"),
			observability.Error(err),
		)
		return entities.Card{}, fmt.Errorf("%s update: %w", prefixCardRepository, err)
	}

	return r.hydrate(ctx, span, "update", id, uid, name, nickname, closingDay, dueDay, limitCents, version, createdAt, updatedAt, nil)
}

func (r *cardRepository) UpdateLimitByIDForUser(ctx context.Context, c entities.Card, expectedVersion int64) (entities.Card, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.update_limit")
	defer span.End()

	const query = `
		UPDATE mecontrola.cards
		   SET limit_cents = $1,
		       version     = version + 1,
		       updated_at  = $2
		 WHERE id = $3 AND user_id = $4 AND version = $5 AND deleted_at IS NULL
		RETURNING id, user_id, name, nickname, closing_day, due_day, limit_cents, version, created_at, updated_at
	`

	var (
		id, uid, name, nickname string
		closingDay, dueDay      int
		limitCents              int64
		version                 int64
		createdAt, updatedAt    time.Time
	)

	err := r.db.QueryRowContext(ctx, query,
		c.LimitCents,
		c.UpdatedAt.UTC(),
		c.ID,
		c.UserID,
		expectedVersion,
	).Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &limitCents, &version, &createdAt, &updatedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return entities.Card{}, fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrCardLimitConflict)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.update_limit.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "update_limit"),
			observability.Error(err),
		)
		return entities.Card{}, fmt.Errorf("%s update_limit: %w", prefixCardRepository, err)
	}

	return r.hydrate(ctx, span, "update_limit", id, uid, name, nickname, closingDay, dueDay, limitCents, version, createdAt, updatedAt, nil)
}

func (r *cardRepository) SoftDeleteByIDForUser(ctx context.Context, cardID, userID string, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.soft_delete")
	defer span.End()

	const query = `
		UPDATE mecontrola.cards
		   SET deleted_at = $1
		 WHERE id = $2 AND user_id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, now.UTC(), cardID, userID)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.soft_delete.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "soft_delete"),
			observability.Error(err),
		)
		return fmt.Errorf("%s soft_delete: %w", prefixCardRepository, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s soft_delete rows_affected: %w", prefixCardRepository, err)
	}
	if affected == 0 {
		return fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrCardNotFound)
	}
	return nil
}

func (r *cardRepository) hydrate(
	ctx context.Context,
	span observability.Span,
	op string,
	id, userID, name, nickname string,
	closingDay, dueDay int,
	limitCents, version int64,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
) (entities.Card, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		span.RecordError(err)
		return entities.Card{}, fmt.Errorf("%s hydrate id: %w", prefixCardRepository, err)
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		span.RecordError(err)
		return entities.Card{}, fmt.Errorf("%s hydrate user_id: %w", prefixCardRepository, err)
	}

	cardName, err := valueobjects.NewCardName(name)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.hydrate_failed",
			observability.String("layer", "repository"),
			observability.String("operation", op),
			observability.Error(err),
		)
		return entities.Card{}, fmt.Errorf("%s hydrate name: %w", prefixCardRepository, err)
	}

	nick, err := valueobjects.NewNickname(nickname)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "card.repository.pg.hydrate_failed",
			observability.String("layer", "repository"),
			observability.String("operation", op),
			observability.Error(err),
		)
		return entities.Card{}, fmt.Errorf("%s hydrate nickname: %w", prefixCardRepository, err)
	}

	cycle, err := valueobjects.NewBillingCycle(closingDay, dueDay)
	if err != nil {
		span.RecordError(err)
		return entities.Card{}, fmt.Errorf("%s hydrate cycle: %w", prefixCardRepository, err)
	}

	return entities.HydrateCardWithVersion(parsedID, parsedUserID, cardName, nick, cycle, limitCents, version, createdAt, updatedAt, deletedAt), nil
}
