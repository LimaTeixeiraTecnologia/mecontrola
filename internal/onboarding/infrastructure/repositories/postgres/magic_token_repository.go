package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

type magicTokenRepository struct {
	o11y observability.Observability
	db   database.DBTX
}

func NewMagicTokenRepository(o11y observability.Observability, db database.DBTX) appinterfaces.MagicTokenRepository {
	return &magicTokenRepository{o11y: o11y, db: db}
}

func (r *magicTokenRepository) Insert(ctx context.Context, token entities.MagicToken) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.insert")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.onboarding_tokens
		       (id, token_hash, activation_token_ciphertext, status, plan_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		token.ID(),
		token.TokenHash(),
		token.ActivationTokenCiphertext(),
		token.Status().String(),
		token.PlanID(),
		token.ExpiresAt(),
		token.CreatedAt(),
	)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.insert: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) FindByHash(ctx context.Context, tokenHash []byte) (entities.MagicToken, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.find_by_hash")
	defer span.End()

	const query = `
		SELECT id, token_hash, status, plan_id, expires_at, created_at,
		       paid_at, consumed_at, outreach_sent_at,
		       activation_token_ciphertext, subscription_id,
		       customer_mobile_e164, customer_email, external_sale_id,
		       consumed_by_user_id, consumed_by_mobile_e164, activation_path,
		       telegram_external_id
		  FROM mecontrola.onboarding_tokens
		 WHERE token_hash = $1
	`

	row := r.db.QueryRowContext(ctx, query, tokenHash)
	return scanMagicToken(row)
}

func (r *magicTokenRepository) FindPaidByMobileForFallback(ctx context.Context, mobileE164 string) (entities.MagicToken, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.find_paid_by_mobile_for_fallback")
	defer span.End()

	const query = `
		SELECT id, token_hash, status, plan_id, expires_at, created_at,
		       paid_at, consumed_at, outreach_sent_at,
		       activation_token_ciphertext, subscription_id,
		       customer_mobile_e164, customer_email, external_sale_id,
		       consumed_by_user_id, consumed_by_mobile_e164, activation_path,
		       telegram_external_id
		  FROM mecontrola.onboarding_tokens
		 WHERE status = 'PAID'
		   AND customer_mobile_e164 = $1
		   AND outreach_sent_at IS NOT NULL
		 ORDER BY paid_at DESC
		 LIMIT 1
	`

	row := r.db.QueryRowContext(ctx, query, mobileE164)
	return scanMagicToken(row)
}

func (r *magicTokenRepository) FindPaidForOutreach(ctx context.Context, olderThan time.Time, limit int) ([]entities.MagicToken, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.find_paid_for_outreach")
	defer span.End()

	const query = `
		SELECT id, token_hash, status, plan_id, expires_at, created_at,
		       paid_at, consumed_at, outreach_sent_at,
		       activation_token_ciphertext, subscription_id,
		       customer_mobile_e164, customer_email, external_sale_id,
		       consumed_by_user_id, consumed_by_mobile_e164, activation_path,
		       telegram_external_id
		  FROM mecontrola.onboarding_tokens
		 WHERE status = 'PAID'
		   AND outreach_sent_at IS NULL
		   AND paid_at < $1
		   AND (customer_mobile_e164 IS NOT NULL OR telegram_external_id IS NOT NULL)
		 ORDER BY paid_at ASC
		 LIMIT $2
		   FOR UPDATE SKIP LOCKED
	`

	rows, err := r.db.QueryContext(ctx, query, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("onboarding: magic_token_repository.find_paid_for_outreach: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.WarnContext(ctx, "onboarding: magic_token_repository.find_paid_for_outreach: close rows", "error", closeErr)
		}
	}()

	var result []entities.MagicToken
	for rows.Next() {
		t, err := scanMagicToken(rows)
		if err != nil {
			return nil, fmt.Errorf("onboarding: magic_token_repository.find_paid_for_outreach: scan: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("onboarding: magic_token_repository.find_paid_for_outreach: rows: %w", err)
	}
	return result, nil
}

func (r *magicTokenRepository) UpdateMarkPaid(ctx context.Context, token entities.MagicToken) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.update_mark_paid")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET status                = $1,
		       paid_at               = $2,
		       subscription_id       = $3,
		       customer_mobile_e164  = $4,
		       customer_email        = $5,
		       external_sale_id      = $6
		 WHERE id     = $7
		   AND status = 'PENDING'
	`

	_, err := r.db.ExecContext(ctx, query,
		token.Status().String(),
		sqlnull.Time(token.PaidAt()),
		sqlnull.Str(token.SubscriptionID()),
		sqlnull.Str(token.CustomerMobileE164()),
		sqlnull.Str(token.CustomerEmail()),
		sqlnull.Str(token.ExternalSaleID()),
		token.ID(),
	)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.update_mark_paid: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) UpdateMarkConsumed(ctx context.Context, token entities.MagicToken) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.update_mark_consumed")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET status                  = $1,
		       consumed_at             = $2,
		       consumed_by_user_id     = $3,
		       consumed_by_mobile_e164 = $4,
		       activation_path         = $5
		 WHERE id     = $6
		   AND status = 'PAID'
	`

	_, err := r.db.ExecContext(ctx, query,
		token.Status().String(),
		sqlnull.Time(token.ConsumedAt()),
		sqlnull.Str(token.ConsumedByUserID()),
		sqlnull.Str(token.ConsumedByMobileE164()),
		token.ActivationPath().String(),
		token.ID(),
	)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.update_mark_consumed: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) UpdateMarkOutreachSent(ctx context.Context, tokenID string, sentAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.update_mark_outreach_sent")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET outreach_sent_at = $1
		 WHERE id                = $2
		   AND outreach_sent_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, sentAt, tokenID)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.update_mark_outreach_sent: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) UpdateMarkOutreachReset(ctx context.Context, tokenID string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.update_mark_outreach_reset")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET outreach_sent_at = NULL
		 WHERE id = $1
	`

	_, err := r.db.ExecContext(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.update_mark_outreach_reset: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) UpdateTelegramExternalID(ctx context.Context, tokenID, externalID string) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.update_telegram_external_id")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET telegram_external_id = $1
		 WHERE id = $2
	`

	_, err := r.db.ExecContext(ctx, query, sqlnull.Str(externalID), tokenID)
	if err != nil {
		return fmt.Errorf("onboarding: magic_token_repository.update_telegram_external_id: %w", err)
	}
	return nil
}

func (r *magicTokenRepository) BulkExpire(ctx context.Context, now time.Time, limit int) ([]entities.MagicToken, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.bulk_expire")
	defer span.End()

	const query = `
		UPDATE mecontrola.onboarding_tokens
		   SET status = 'EXPIRED'
		 WHERE id IN (
		     SELECT id
		       FROM mecontrola.onboarding_tokens
		      WHERE status IN ('PENDING', 'PAID')
		        AND expires_at < $1
		      LIMIT $2
		   FOR UPDATE SKIP LOCKED
		 )
		RETURNING id, token_hash, status, plan_id, expires_at, created_at,
		          paid_at, consumed_at, outreach_sent_at,
		          activation_token_ciphertext, subscription_id,
		          customer_mobile_e164, customer_email, external_sale_id,
		          consumed_by_user_id, consumed_by_mobile_e164, activation_path,
		          telegram_external_id
	`

	rows, err := r.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("onboarding: magic_token_repository.bulk_expire: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.WarnContext(ctx, "onboarding: magic_token_repository.bulk_expire: close rows", "error", closeErr)
		}
	}()

	var result []entities.MagicToken
	for rows.Next() {
		t, err := scanMagicToken(rows)
		if err != nil {
			return nil, fmt.Errorf("onboarding: magic_token_repository.bulk_expire: scan: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("onboarding: magic_token_repository.bulk_expire: rows: %w", err)
	}
	return result, nil
}

func (r *magicTokenRepository) CountPaidUnconsumed(ctx context.Context) (int64, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "onboarding.repository.magic_token.count_paid_unconsumed")
	defer span.End()

	const query = `SELECT COUNT(*) FROM mecontrola.onboarding_tokens WHERE status = 'PAID'`

	var count int64
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("onboarding: magic_token_repository.count_paid_unconsumed: %w", err)
	}
	return count, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMagicToken(s rowScanner) (entities.MagicToken, error) {
	var (
		id                        string
		tokenHash                 []byte
		statusRaw                 string
		planID                    string
		expiresAt                 time.Time
		createdAt                 time.Time
		paidAt                    sql.NullTime
		consumedAt                sql.NullTime
		outreachSentAt            sql.NullTime
		activationTokenCiphertext sql.NullString
		subscriptionID            sql.NullString
		customerMobileE164        sql.NullString
		customerEmail             sql.NullString
		externalSaleID            sql.NullString
		consumedByUserID          sql.NullString
		consumedByMobileE164      sql.NullString
		activationPathRaw         sql.NullString
		telegramExternalID        sql.NullString
	)

	if err := s.Scan(
		&id, &tokenHash, &statusRaw, &planID, &expiresAt, &createdAt,
		&paidAt, &consumedAt, &outreachSentAt,
		&activationTokenCiphertext, &subscriptionID,
		&customerMobileE164, &customerEmail, &externalSaleID,
		&consumedByUserID, &consumedByMobileE164, &activationPathRaw,
		&telegramExternalID,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entities.MagicToken{}, domain.ErrTokenNotFound
		}
		return entities.MagicToken{}, fmt.Errorf("onboarding: magic_token_repository.scan: %w", err)
	}

	status, err := valueobjects.ParseTokenStatus(statusRaw)
	if err != nil {
		return entities.MagicToken{}, fmt.Errorf("onboarding: magic_token_repository.scan: parse status: %w", err)
	}

	var activationPath valueobjects.ActivationPath
	if activationPathRaw.Valid && activationPathRaw.String != "" {
		activationPath, _ = valueobjects.ParseActivationPath(activationPathRaw.String)
	}

	return entities.HydrateMagicToken(
		id,
		tokenHash,
		status,
		planID,
		expiresAt,
		createdAt,
		nullTime(paidAt),
		nullTime(consumedAt),
		nullTime(outreachSentAt),
		nullStr(activationTokenCiphertext),
		nullStr(subscriptionID),
		nullStr(customerMobileE164),
		nullStr(customerEmail),
		nullStr(externalSaleID),
		nullStr(consumedByUserID),
		nullStr(consumedByMobileE164),
		activationPath,
		nullStr(telegramExternalID),
	), nil
}

func nullStr(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

func nullTime(t sql.NullTime) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}
