package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

const resolveCardByNicknameLimit = 100

type ResolveCardByNickname struct {
	repo interfaces.CardRepository
	o11y observability.Observability
}

func NewResolveCardByNickname(
	repo interfaces.CardRepository,
	o11y observability.Observability,
) *ResolveCardByNickname {
	return &ResolveCardByNickname{repo: repo, o11y: o11y}
}

func (u *ResolveCardByNickname) Execute(ctx context.Context, in input.ResolveCardByNickname) (output.Card, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.resolve_by_nickname",
		observability.WithAttributes(
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.Card{}, err
	}

	cards, _, err := u.repo.ListByUser(ctx, in.UserID.String(), "", resolveCardByNicknameLimit)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", "internal_error"))
		return output.Card{}, err
	}

	target := strings.TrimSpace(in.Nickname)
	for _, c := range cards {
		if c.IsDeleted() {
			continue
		}
		if strings.EqualFold(c.Nickname.String(), target) {
			span.SetAttributes(observability.String("outcome", "success"))
			return mappers.M.ToCardOutput(c), nil
		}
	}

	span.SetAttributes(observability.String("outcome", "not_found"))
	return output.Card{}, fmt.Errorf("card/resolve_card_by_nickname: %w", domain.ErrCardNotFound)
}
