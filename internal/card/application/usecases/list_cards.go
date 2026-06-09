package usecases

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type cursorPayload struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

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
		if err := validateCursor(in.Cursor); err != nil {
			span.SetAttributes(observability.String("outcome", "invalid"))
			return output.CardList{}, err
		}
	}

	repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
	cards, nextCursor, err := repo.ListByUser(ctx, in.UserID.String(), in.Cursor, in.Limit)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(observability.String("outcome", "internal_error"))
		return output.CardList{}, err
	}

	out := make([]output.Card, 0, len(cards))
	for _, c := range cards {
		out = append(out, toCardOutput(c))
	}

	span.SetAttributes(observability.String("outcome", "success"))
	u.o11y.Logger().Info(ctx, "card.list.served",
		observability.String("user_id", in.UserID.String()),
		observability.Int("count", len(out)),
	)

	return output.CardList{
		Cards:      out,
		NextCursor: nextCursor,
	}, nil
}

func validateCursor(cursor string) error {
	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("%w: base64 decode: %s", domain.ErrInvalidCursor, err)
	}

	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("%w: json unmarshal: %s", domain.ErrInvalidCursor, err)
	}

	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created_at is zero", domain.ErrInvalidCursor)
	}

	if p.ID == "" {
		return fmt.Errorf("%w: id is empty", domain.ErrInvalidCursor)
	}

	return nil
}

func EncodeCursor(createdAt time.Time, id string) (string, error) {
	raw, err := json.Marshal(cursorPayload{CreatedAt: createdAt, ID: id})
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.URLEncoding.EncodeToString(raw), nil
}
