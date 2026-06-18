package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type SoftDeleteCard struct {
	uow     uow.UnitOfWork
	factory interfaces.RepositoryFactory
	idem    idempotency.Storage
	o11y    observability.Observability
}

func NewSoftDeleteCard(
	u uow.UnitOfWork,
	factory interfaces.RepositoryFactory,
	idem idempotency.Storage,
	o11y observability.Observability,
) *SoftDeleteCard {
	return &SoftDeleteCard{uow: u, factory: factory, idem: idem, o11y: o11y}
}

func (u *SoftDeleteCard) Execute(ctx context.Context, in input.SoftDeleteCard) error {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.delete",
		observability.WithAttributes(
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	ic, hasIdem := idempotency.FromContext(ctx)

	err := u.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) error {
		repo := u.factory.CardRepository(tx)
		now := time.Now().UTC()

		if delErr := repo.SoftDeleteByIDForUser(ctx, in.ID.String(), in.UserID.String(), now); delErr != nil {
			return delErr
		}

		if hasIdem {
			body, marshalErr := json.Marshal(map[string]any{})
			if marshalErr != nil {
				return fmt.Errorf("soft_delete_card: marshal output: %w", marshalErr)
			}
			rec := idempotency.Record{
				Scope:          ic.Scope,
				Key:            ic.Key,
				UserID:         ic.UserID,
				RequestHash:    ic.RequestHash,
				ResponseStatus: http.StatusNoContent,
				ResponseBody:   body,
				ExpiresAt:      ic.ExpiresAt,
			}
			if putErr := u.idem.Put(ctx, rec); putErr != nil {
				return fmt.Errorf("soft_delete_card: idempotency put: %w", putErr)
			}
		}

		return nil
	})

	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		u.o11y.Logger().Error(ctx, "card.delete.failed",
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
			observability.Error(err),
		)
		return err
	}

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.delete.completed",
		observability.String("card_id", in.ID.String()),
		observability.String("user_id", in.UserID.String()),
	)
	return nil
}
