package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type ListCards struct {
	factory interfaces.RepositoryFactory
	mgr     manager.Manager
	o11y    observability.Observability
}

func NewListCards(
	factory interfaces.RepositoryFactory,
	mgr manager.Manager,
	o11y observability.Observability,
) *ListCards {
	return &ListCards{factory: factory, mgr: mgr, o11y: o11y}
}

func (u *ListCards) Execute(ctx context.Context, in input.ListCards) (output.CardList, error) {
	ctx, span := u.o11y.Tracer().Start(ctx, "card.usecase.list",
		observability.WithAttributes(
			observability.String("user_id", in.UserID.String()),
		),
	)
	defer span.End()

	if in.Cursor != "" {
		if _, err := pagination.Decode(in.Cursor); err != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			return output.CardList{}, fmt.Errorf("%w: %s", domain.ErrInvalidCursor, err.Error())
		}
	}

	repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
	cards, nextCursor, err := repo.ListByUser(ctx, in.UserID.String(), in.Cursor, in.Limit)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", "internal_error"))
		return output.CardList{}, err
	}

	result := mappers.ToCardListOutput(cards, nextCursor)

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.list.served",
		observability.String("user_id", in.UserID.String()),
		observability.Int("count", len(result.Items)),
	)

	return result, nil
}
