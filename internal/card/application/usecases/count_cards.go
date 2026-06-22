package usecases

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
)

type CountCards struct {
	repo interfaces.CardRepository
	o11y observability.Observability
}

func NewCountCards(
	repo interfaces.CardRepository,
	o11y observability.Observability,
) *CountCards {
	return &CountCards{repo: repo, o11y: o11y}
}

func (u *CountCards) Execute(ctx context.Context, in input.CountCards) (output.CardCount, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.count",
		observability.WithAttributes(
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.CardCount{}, err
	}

	total, err := u.repo.CountActiveByUser(ctx, in.UserID.String())
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", "internal_error"))
		return output.CardCount{}, err
	}

	span.SetAttributes(observability.String("outcome", "success"))
	return output.CardCount{Total: total}, nil
}
