package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

const expireBatchSize = 1000

type ExpireTokens struct {
	db            database.DBTX
	factory       appinterfaces.RepositoryFactory
	idGen         id.Generator
	o11y          observability.Observability
	orphanExpired observability.Counter
}

func NewExpireTokens(
	db database.DBTX,
	factory appinterfaces.RepositoryFactory,
	idGen id.Generator,
	o11y observability.Observability,
) *ExpireTokens {
	orphanExpired := o11y.Metrics().Counter(
		"onboarding_orphan_expired_total",
		"Total de tokens PAID expirados sem consumo (subscription orfas)",
		"1",
	)
	return &ExpireTokens{db: db, factory: factory, idGen: idGen, o11y: o11y, orphanExpired: orphanExpired}
}

func (uc *ExpireTokens) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.expire_tokens")
	defer span.End()

	now := time.Now().UTC()
	db := uc.db
	tokenRepo := uc.factory.MagicTokenRepository(db)
	signalRepo := uc.factory.SupportSignalRepository(db)

	for {
		expired, err := tokenRepo.BulkExpire(ctx, now, expireBatchSize)
		if err != nil {
			return fmt.Errorf("onboarding: expire tokens: bulk expire: %w", err)
		}
		if len(expired) == 0 {
			break
		}

		for _, t := range expired {
			if t.Status() == valueobjects.TokenStatusPaid || !t.PaidAt().IsZero() {
				if sigErr := uc.emitOrphanSignal(ctx, signalRepo, t, now); sigErr != nil {
					slog.WarnContext(ctx, "onboarding.token.expired_with_paid_state.signal_failed",
						"token_id", t.ID(),
						"error", sigErr.Error(),
					)
				}
				uc.orphanExpired.Add(ctx, 1)
				slog.WarnContext(ctx, "onboarding.token.expired_with_paid_state",
					"token_id", t.ID(),
					"external_sale_id", t.ExternalSaleID(),
				)
			}
		}

		if len(expired) < expireBatchSize {
			break
		}
	}

	return nil
}

func (uc *ExpireTokens) emitOrphanSignal(
	ctx context.Context,
	signalRepo appinterfaces.SupportSignalRepository,
	token entities.MagicToken,
	expiredAt time.Time,
) error {
	payload, err := json.Marshal(map[string]any{
		"token_hash_prefix": valueobjects.TokenHashPrefix(token.TokenHash()),
		"external_sale_id":  token.ExternalSaleID(),
		"expired_at":        expiredAt,
		"has_paid_state":    true,
	})
	if err != nil {
		return fmt.Errorf("onboarding: expire tokens: marshal signal: %w", err)
	}
	sig, err := entities.NewSupportSignal(uc.idGen.NewID(), valueobjects.SupportSignalKindOrphanExpiredSubscription, payload)
	if err != nil {
		return fmt.Errorf("onboarding: expire tokens: new signal: %w", err)
	}
	if err := signalRepo.Insert(ctx, sig); err != nil {
		return fmt.Errorf("onboarding: expire tokens: insert signal: %w", err)
	}
	return nil
}
