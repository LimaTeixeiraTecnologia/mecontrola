package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

const prefixUserRepository = "identity.repository.user:"

type userRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewUserRepository(o11y observability.Observability, db database.DBTX) interfaces.UserRepository {
	return &userRepository{o11y: o11y, db: db}
}

func (r *userRepository) UpsertByWhatsAppNumber(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.upsert_by_whatsapp_number")
	defer span.End()

	const query = `
		INSERT INTO users (id, whatsapp_number, email, display_name, status, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
		ON CONFLICT (whatsapp_number) WHERE deleted_at IS NULL
		DO UPDATE SET
			display_name = COALESCE(users.display_name, EXCLUDED.display_name),
			email        = COALESCE(users.email,        EXCLUDED.email),
			updated_at   = EXCLUDED.updated_at
		RETURNING id, whatsapp_number, email, display_name, status, created_at, updated_at
	`

	row := r.db.QueryRowContext(ctx, query,
		candidate.ID(),
		candidate.WhatsApp().String(),
		sqlnull.Str(candidate.Email().String()),
		sqlnull.Str(candidate.DisplayName()),
		string(entities.StatusActive),
		candidate.CreatedAt(),
		now,
	)

	var (
		id, whatsapp, status string
		email, displayName   sql.NullString
		createdAt, updatedAt time.Time
	)
	if err := row.Scan(&id, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt); err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user.upsert.scan_failed",
			observability.String("layer", "repository"),
			observability.String("operation", "upsert_by_whatsapp_number"),
			observability.String("whatsapp", candidate.WhatsApp().Masked()),
			observability.Error(err),
		)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			switch pgErr.ConstraintName {
			case "users_whatsapp_number_active_uniq":
				return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrWhatsAppNumberInUse)
			case "users_email_active_uniq":
				return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrEmailInUse)
			}
		}
		return entities.User{}, fmt.Errorf("%s upsert scan: %w", prefixUserRepository, err)
	}

	return entities.Hydrate(id, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, time.Time{}), nil
}

func (r *userRepository) FindByID(ctx context.Context, id string) (entities.User, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.find_by_id")
	defer span.End()

	const query = `
		SELECT id, whatsapp_number, email, display_name, status, created_at, updated_at
		  FROM users
		 WHERE id = $1 AND deleted_at IS NULL
	`

	var (
		foundID, whatsapp, status string
		email, displayName        sql.NullString
		createdAt, updatedAt      time.Time
	)
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&foundID, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrUserNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user.find_by_id.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "find_by_id"),
			observability.Error(err),
		)
		return entities.User{}, fmt.Errorf("%s find by id: %w", prefixUserRepository, err)
	}

	return entities.Hydrate(foundID, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, time.Time{}), nil
}

func (r *userRepository) FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (entities.User, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.find_by_whatsapp_number")
	defer span.End()

	const query = `
		SELECT id, whatsapp_number, email, display_name, status, created_at, updated_at
		  FROM users
		 WHERE whatsapp_number = $1 AND deleted_at IS NULL
	`

	var (
		id, whatsapp, status string
		email, displayName   sql.NullString
		createdAt, updatedAt time.Time
	)
	err := r.db.QueryRowContext(ctx, query, number.String()).
		Scan(&id, &whatsapp, &email, &displayName, &status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.User{}, fmt.Errorf("%s %w", prefixUserRepository, application.ErrUserNotFound)
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user.find_by_whatsapp.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "find_by_whatsapp_number"),
			observability.String("whatsapp", number.Masked()),
			observability.Error(err),
		)
		return entities.User{}, fmt.Errorf("%s find by whatsapp: %w", prefixUserRepository, err)
	}

	return entities.Hydrate(id, whatsapp, email.String, displayName.String, status, createdAt, updatedAt, time.Time{}), nil
}

func (r *userRepository) MarkDeleted(ctx context.Context, id string, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.mark_deleted")
	defer span.End()

	const query = `
		UPDATE users
		   SET status = $1, deleted_at = $2, updated_at = $2
		 WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, string(entities.StatusDeleted), now, id)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user.mark_deleted.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "mark_deleted"),
			observability.Error(err),
		)
		return fmt.Errorf("%s mark deleted: %w", prefixUserRepository, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s mark deleted rows affected: %w", prefixUserRepository, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s %w", prefixUserRepository, application.ErrUserNotFound)
	}
	return nil
}

func (r *userRepository) AppendWhatsAppHistory(ctx context.Context, userID string, entry interfaces.WhatsAppHistoryEntry) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user.append_whatsapp_history")
	defer span.End()

	const query = `
		INSERT INTO user_whatsapp_history (id, user_id, number, active, linked_at, unlinked_at, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		entry.ID,
		userID,
		entry.Number,
		entry.Active,
		entry.LinkedAt,
		sqlnull.Time(entry.UnlinkedAt),
		sqlnull.Str(entry.Reason),
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user.append_history.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "append_whatsapp_history"),
			observability.Error(err),
		)
		return fmt.Errorf("%s append whatsapp history: %w", prefixUserRepository, err)
	}
	return nil
}
