package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

const prefixCardRepository = "card.repository.pg:"

type cursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

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
		INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, query,
		c.ID,
		c.UserID,
		c.Name.String(),
		c.Nickname.String(),
		c.Cycle.ClosingDay,
		c.Cycle.DueDay,
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
		SELECT id, user_id, name, nickname, closing_day, due_day, created_at, updated_at
		  FROM mecontrola.cards
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	var (
		id, name, nickname   string
		uid                  string
		closingDay, dueDay   int
		createdAt, updatedAt time.Time
	)

	err := r.db.QueryRowContext(ctx, query, cardID, userID).
		Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &createdAt, &updatedAt)
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

	return r.hydrate(ctx, span, "get_by_id", id, uid, name, nickname, closingDay, dueDay, createdAt, updatedAt, nil)
}

func (r *cardRepository) ListByUser(ctx context.Context, userID, cursor string, limit int) ([]entities.Card, string, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "card.repository.pg.list_by_user")
	defer span.End()

	fetch := limit + 1

	var (
		rows database.Rows
		err  error
	)

	if cursor == "" {
		const query = `
			SELECT id, user_id, name, nickname, closing_day, due_day, created_at, updated_at
			  FROM mecontrola.cards
			 WHERE user_id = $1 AND deleted_at IS NULL
			 ORDER BY created_at DESC, id DESC
			 LIMIT $2
		`
		rows, err = r.db.QueryContext(ctx, query, userID, fetch)
	} else {
		cp, decErr := decodeCursor(cursor)
		if decErr != nil {
			return nil, "", fmt.Errorf("%s %w", prefixCardRepository, carddomain.ErrInvalidCursor)
		}
		const query = `
			SELECT id, user_id, name, nickname, closing_day, due_day, created_at, updated_at
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
			createdAt, updatedAt    time.Time
		)
		if scanErr := rows.Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &createdAt, &updatedAt); scanErr != nil {
			span.RecordError(scanErr)
			return nil, "", fmt.Errorf("%s list_by_user scan: %w", prefixCardRepository, scanErr)
		}
		card, hydrateErr := r.hydrate(ctx, span, "list_by_user", id, uid, name, nickname, closingDay, dueDay, createdAt, updatedAt, nil)
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
		nextCursor = encodeCursor(last.CreatedAt, last.ID.String())
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
		       updated_at  = $5
		 WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL
		RETURNING id, user_id, name, nickname, closing_day, due_day, created_at, updated_at
	`

	var (
		id, uid, name, nickname string
		closingDay, dueDay      int
		createdAt, updatedAt    time.Time
	)

	err := r.db.QueryRowContext(ctx, query,
		c.Name.String(),
		c.Nickname.String(),
		c.Cycle.ClosingDay,
		c.Cycle.DueDay,
		c.UpdatedAt.UTC(),
		c.ID,
		c.UserID,
	).Scan(&id, &uid, &name, &nickname, &closingDay, &dueDay, &createdAt, &updatedAt)

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

	return r.hydrate(ctx, span, "update", id, uid, name, nickname, closingDay, dueDay, createdAt, updatedAt, nil)
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

	return entities.HydrateCard(parsedID, parsedUserID, cardName, nick, cycle, createdAt, updatedAt, deletedAt), nil
}

func encodeCursor(t time.Time, id string) string {
	payload := cursorPayload{CreatedAt: t.UTC(), ID: id}
	b, _ := json.Marshal(payload)
	return base64.URLEncoding.EncodeToString(b)
}

func decodeCursor(cursor string) (cursorPayload, error) {
	b, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return cursorPayload{}, fmt.Errorf("decode base64: %w", err)
	}
	var cp cursorPayload
	if err := json.Unmarshal(b, &cp); err != nil {
		return cursorPayload{}, fmt.Errorf("decode json: %w", err)
	}
	if cp.ID == "" || cp.CreatedAt.IsZero() {
		return cursorPayload{}, fmt.Errorf("invalid cursor fields")
	}
	return cp, nil
}
