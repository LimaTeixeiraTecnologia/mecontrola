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

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

const prefixUserIdentityRepository = "identity.repository.user_identity:"

type userIdentityRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewUserIdentityRepository(o11y observability.Observability, db database.DBTX) interfaces.UserIdentityRepository {
	return &userIdentityRepository{o11y: o11y, db: db}
}

func (r *userIdentityRepository) TryFindActive(ctx context.Context, channel valueobjects.Channel, externalID valueobjects.ExternalID) (entities.UserIdentity, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user_identity.try_find_active")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, external_id, verified_at, created_at
		  FROM mecontrola.user_identities
		 WHERE channel = $1 AND external_id = $2 AND unlinked_at IS NULL
		 LIMIT 1
	`

	var (
		id, userID                uuid.UUID
		channelRaw, externalIDRaw string
		verifiedAt, createdAt     time.Time
	)
	err := r.db.QueryRowContext(ctx, query, channel.String(), externalID.String()).
		Scan(&id, &userID, &channelRaw, &externalIDRaw, &verifiedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.UserIdentity{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user_identity.try_find_active.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "try_find_active"),
			observability.String("channel", channel.String()),
			observability.String("external_id_masked", externalID.Masked()),
			observability.Error(err),
		)
		return entities.UserIdentity{}, false, fmt.Errorf("%s try find active: %w", prefixUserIdentityRepository, err)
	}

	identity, hydrateErr := entities.HydrateUserIdentity(id, userID, channelRaw, externalIDRaw, verifiedAt, createdAt, time.Time{})
	if hydrateErr != nil {
		span.RecordError(hydrateErr)
		r.o11y.Logger().Error(ctx, "identity.repository.user_identity.try_find_active.hydrate_failed",
			observability.String("layer", "repository"),
			observability.String("operation", "try_find_active"),
			observability.String("identity_id", id.String()),
			observability.Error(hydrateErr),
		)
		return entities.UserIdentity{}, false, fmt.Errorf("%s hydrate: %w", prefixUserIdentityRepository, hydrateErr)
	}
	return identity, true, nil
}

func (r *userIdentityRepository) FindByUserAndChannel(ctx context.Context, userID uuid.UUID, channel valueobjects.Channel) (entities.UserIdentity, bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user_identity.find_by_user_and_channel")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, external_id, verified_at, created_at
		  FROM mecontrola.user_identities
		 WHERE user_id = $1 AND channel = $2 AND unlinked_at IS NULL
		 LIMIT 1
	`

	var (
		id, foundUserID           uuid.UUID
		channelRaw, externalIDRaw string
		verifiedAt, createdAt     time.Time
	)
	err := r.db.QueryRowContext(ctx, query, userID, channel.String()).
		Scan(&id, &foundUserID, &channelRaw, &externalIDRaw, &verifiedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entities.UserIdentity{}, false, nil
	}
	if err != nil {
		span.RecordError(err)
		return entities.UserIdentity{}, false, fmt.Errorf("%s find by user and channel: %w", prefixUserIdentityRepository, err)
	}

	identity, hydrateErr := entities.HydrateUserIdentity(id, foundUserID, channelRaw, externalIDRaw, verifiedAt, createdAt, time.Time{})
	if hydrateErr != nil {
		span.RecordError(hydrateErr)
		return entities.UserIdentity{}, false, fmt.Errorf("%s hydrate: %w", prefixUserIdentityRepository, hydrateErr)
	}
	return identity, true, nil
}

func (r *userIdentityRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]entities.UserIdentity, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user_identity.list_by_user")
	defer span.End()

	const query = `
		SELECT id, user_id, channel, external_id, verified_at, created_at, unlinked_at
		  FROM mecontrola.user_identities
		 WHERE user_id = $1
		 ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("%s list by user: %w", prefixUserIdentityRepository, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			span.RecordError(closeErr)
		}
	}()

	identities := make([]entities.UserIdentity, 0, 4)
	for rows.Next() {
		var (
			id, foundUserID           uuid.UUID
			channelRaw, externalIDRaw string
			verifiedAt, createdAt     time.Time
			unlinkedAt                sql.NullTime
		)
		if scanErr := rows.Scan(&id, &foundUserID, &channelRaw, &externalIDRaw, &verifiedAt, &createdAt, &unlinkedAt); scanErr != nil {
			span.RecordError(scanErr)
			return nil, fmt.Errorf("%s list by user scan: %w", prefixUserIdentityRepository, scanErr)
		}
		var unlinked time.Time
		if unlinkedAt.Valid {
			unlinked = unlinkedAt.Time
		}
		identity, hydrateErr := entities.HydrateUserIdentity(id, foundUserID, channelRaw, externalIDRaw, verifiedAt, createdAt, unlinked)
		if hydrateErr != nil {
			span.RecordError(hydrateErr)
			return nil, fmt.Errorf("%s hydrate: %w", prefixUserIdentityRepository, hydrateErr)
		}
		identities = append(identities, identity)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		span.RecordError(rowsErr)
		return nil, fmt.Errorf("%s list by user rows: %w", prefixUserIdentityRepository, rowsErr)
	}
	return identities, nil
}

func (r *userIdentityRepository) Insert(ctx context.Context, identity entities.UserIdentity) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user_identity.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.user_identities (id, user_id, channel, external_id, verified_at, created_at, unlinked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		identity.ID(),
		identity.UserID(),
		identity.Channel().String(),
		identity.ExternalID().String(),
		identity.VerifiedAt(),
		identity.CreatedAt(),
		sqlnull.Time(identity.UnlinkedAt()),
	)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user_identity.insert.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "insert"),
			observability.String("identity_id", identity.ID().String()),
			observability.String("channel", identity.Channel().String()),
			observability.Error(err),
		)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%s %w", prefixUserIdentityRepository, application.ErrUserIdentityAlreadyLinked)
		}
		return fmt.Errorf("%s insert: %w", prefixUserIdentityRepository, err)
	}
	return nil
}

func (r *userIdentityRepository) Unlink(ctx context.Context, id uuid.UUID, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "identity.repository.user_identity.unlink")
	defer span.End()

	const query = `
		UPDATE mecontrola.user_identities
		   SET unlinked_at = $1
		 WHERE id = $2 AND unlinked_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		span.RecordError(err)
		r.o11y.Logger().Error(ctx, "identity.repository.user_identity.unlink.failed",
			observability.String("layer", "repository"),
			observability.String("operation", "unlink"),
			observability.String("identity_id", id.String()),
			observability.Error(err),
		)
		return fmt.Errorf("%s unlink: %w", prefixUserIdentityRepository, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("%s unlink rows affected: %w", prefixUserIdentityRepository, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s %w", prefixUserIdentityRepository, application.ErrUserIdentityNotFound)
	}
	return nil
}
