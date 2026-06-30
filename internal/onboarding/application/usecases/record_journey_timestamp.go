package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type RecordJourneyTimestamp struct {
	repo appinterfaces.MagicTokenRepository
	o11y observability.Observability
}

func NewRecordJourneyTimestamp(
	repo appinterfaces.MagicTokenRepository,
	o11y observability.Observability,
) *RecordJourneyTimestamp {
	return &RecordJourneyTimestamp{repo: repo, o11y: o11y}
}

func (uc *RecordJourneyTimestamp) Execute(ctx context.Context, in input.RecordJourneyTimestampInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "onboarding.usecase.record_journey_timestamp")
	defer span.End()

	if err := in.Validate(); err != nil {
		return nil
	}

	token, err := valueobjects.TokenFromClear(in.ClearToken)
	if err != nil {
		return nil
	}

	magicToken, err := uc.repo.FindByHash(ctx, token.Hash())
	if err != nil {
		if errors.Is(err, domain.ErrTokenNotFound) {
			return nil
		}
		span.RecordError(err)
		return fmt.Errorf("onboarding: record_journey_timestamp: find: %w", err)
	}

	now := time.Now().UTC()

	switch in.Event {
	case input.JourneyEventPageOpened:
		if err := uc.repo.MarkPageOpened(ctx, magicToken.ID(), now); err != nil {
			span.RecordError(err)
			return fmt.Errorf("onboarding: record_journey_timestamp: mark_page_opened: %w", err)
		}
	case input.JourneyEventWhatsAppOpened:
		if err := uc.repo.MarkWhatsAppOpened(ctx, magicToken.ID(), now); err != nil {
			span.RecordError(err)
			return fmt.Errorf("onboarding: record_journey_timestamp: mark_whatsapp_opened: %w", err)
		}
	}

	return nil
}
