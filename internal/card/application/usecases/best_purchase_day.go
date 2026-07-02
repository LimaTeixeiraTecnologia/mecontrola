package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type BestPurchaseDay struct {
	factory interfaces.RepositoryFactory
	svc     services.PurchaseDayService
	db      database.DBTX
	o11y    observability.Observability
}

func NewBestPurchaseDay(
	factory interfaces.RepositoryFactory,
	db database.DBTX,
	o11y observability.Observability,
) *BestPurchaseDay {
	return &BestPurchaseDay{
		factory: factory,
		svc:     services.PurchaseDayService{},
		db:      db,
		o11y:    o11y,
	}
}

func (u *BestPurchaseDay) Execute(ctx context.Context, in input.BestPurchaseDay) (output.BestPurchaseDay, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.best_purchase_day")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.BestPurchaseDay{}, err
	}

	bank, err := valueobjects.NewBankCode(in.Bank)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.BestPurchaseDay{}, err
	}

	tz, err := services.NewSaoPauloLocation()
	if err != nil {
		return output.BestPurchaseDay{}, fmt.Errorf("card/best_purchase_day: timezone: %w", err)
	}

	bankReader := u.factory.BankDaysReader(u.db)
	days, err := bankReader.DaysBeforeDue(ctx, bank)
	if err != nil {
		span.RecordError(err)
		return output.BestPurchaseDay{}, fmt.Errorf("card/best_purchase_day: bank_days: %w", err)
	}

	pd := u.svc.Decide(in.DueDay, days, time.Now().UTC(), tz)

	span.SetAttributes(observability.String("outcome", "success"))
	return output.BestPurchaseDay{
		ClosingDay:      pd.ClosingDay,
		BestPurchaseDay: pd.BestDay,
	}, nil
}
