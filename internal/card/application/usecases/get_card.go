package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type GetCard struct {
	repo interfaces.CardRepository
	o11y observability.Observability
}

func NewGetCard(
	repo interfaces.CardRepository,
	o11y observability.Observability,
) *GetCard {
	return &GetCard{repo: repo, o11y: o11y}
}

func (u *GetCard) Execute(ctx context.Context, in input.GetCard) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.get",
		observability.WithAttributes(
			observability.String("card_id", in.ID.String()),
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.Card{}, err
	}

	card, err := u.repo.GetByIDForUser(ctx, in.ID.String(), in.UserID.String())
	if err != nil {
		outcome := classifyCardOutcome(err)
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", outcome))
		return output.Card{}, err
	}

	if card.IsDeleted() {
		span.SetAttributes(observability.String("outcome", "not_found"))
		return output.Card{}, fmt.Errorf("card/get_card: %w", domain.ErrCardNotFound)
	}

	span.SetAttributes(observability.String("outcome", "success"))
	return mappers.M.ToCardOutput(card), nil
}
