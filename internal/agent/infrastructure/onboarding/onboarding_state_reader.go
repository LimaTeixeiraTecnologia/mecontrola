package onboarding

import (
	"context"

	"github.com/google/uuid"

	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type onboardingStateReader struct {
	getContext *onbusecases.GetOnboardingContext
}

func NewOnboardingStateReader(getContext *onbusecases.GetOnboardingContext) appusecases.OnboardingStateReader {
	if getContext == nil {
		return nil
	}
	return &onboardingStateReader{getContext: getContext}
}

func (r *onboardingStateReader) Load(ctx context.Context, userID uuid.UUID) (appusecases.OnboardingSnapshot, error) {
	out, err := r.getContext.Execute(ctx, onbusecases.GetOnboardingContextInput{UserID: userID})
	if err != nil {
		return appusecases.OnboardingSnapshot{}, err
	}
	if !out.Found {
		return appusecases.OnboardingSnapshot{InProgress: false}, nil
	}

	cards := make([]appusecases.OnboardingSnapshotCard, 0, len(out.Cards))
	for _, c := range out.Cards {
		cards = append(cards, appusecases.OnboardingSnapshotCard{Name: c.Name, DueDay: c.DueDay})
	}
	splits := make([]appusecases.OnboardingSnapshotSplit, 0, len(out.CustomSplit))
	for _, s := range out.CustomSplit {
		slug := categoryKindStringToSlug[s.Kind]
		if slug == "" {
			slug = s.Kind
		}
		splits = append(splits, appusecases.OnboardingSnapshotSplit{Slug: slug, Percent: s.BasisPoints / 100})
	}

	return appusecases.OnboardingSnapshot{
		InProgress:      !out.State.IsTerminal(),
		State:           out.State.String(),
		Phase:           out.Phase,
		IncomeCents:     out.IncomeCents,
		Objective:       out.Objective,
		Cards:           cards,
		Splits:          splits,
		FirstTxRecorded: out.FirstTxRecorded,
	}, nil
}
