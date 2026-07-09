package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type IsBankRecognized struct {
	factory interfaces.RepositoryFactory
	db      database.DBTX
	o11y    observability.Observability
}

func NewIsBankRecognized(
	factory interfaces.RepositoryFactory,
	db database.DBTX,
	o11y observability.Observability,
) *IsBankRecognized {
	return &IsBankRecognized{
		factory: factory,
		db:      db,
		o11y:    o11y,
	}
}

func (u *IsBankRecognized) Execute(ctx context.Context, in input.IsBankRecognized) (output.IsBankRecognized, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.is_bank_recognized")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.IsBankRecognized{}, err
	}

	bank, err := valueobjects.NewBankCode(in.Bank)
	if err != nil {
		span.SetAttributes(observability.String("outcome", "invalid"))
		return output.IsBankRecognized{}, err
	}

	bankReader := u.factory.BankDaysReader(u.db)
	recognized, err := bankReader.IsBankRecognized(ctx, bank)
	if err != nil {
		span.RecordError(err)
		return output.IsBankRecognized{}, fmt.Errorf("card/is_bank_recognized: bank_days: %w", err)
	}

	span.SetAttributes(observability.String("outcome", "success"))
	return output.IsBankRecognized{Recognized: recognized}, nil
}
