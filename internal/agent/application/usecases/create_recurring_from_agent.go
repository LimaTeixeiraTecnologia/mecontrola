package usecases

import (
	"context"
	"errors"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type RecurringTemplateCreator interface {
	Execute(ctx context.Context, in CreateRecurringCommand) (CreateRecurringResult, error)
}

type CreateRecurringCommand struct {
	UserID         string
	Direction      string
	Description    string
	RootCategoryID string
	SubcategoryID  string
	AmountCents    int64
	Frequency      string
	DayOfMonth     int
}

type CreateRecurringResult struct {
	Persisted bool
}

type CreateRecurringFromAgent struct {
	resolver       CategoryResolver
	creator        RecurringTemplateCreator
	o11y           observability.Observability
	persisted      observability.Counter
	resolveBad     observability.Counter
	scoreHistogram observability.Histogram
}

func NewCreateRecurringFromAgent(
	resolver CategoryResolver,
	creator RecurringTemplateCreator,
	o11y observability.Observability,
) *CreateRecurringFromAgent {
	persisted := o11y.Metrics().Counter(
		"agent_create_recurring_persisted_total",
		"Total de templates recorrentes persistidos a partir de intent do agente por direction",
		"1",
	)
	resolveBad := o11y.Metrics().Counter(
		"agent_create_recurring_failed_total",
		"Total de tentativas de recorrência que falharam ao resolver categoria ou persistir",
		"1",
	)
	return &CreateRecurringFromAgent{
		resolver:       resolver,
		creator:        creator,
		o11y:           o11y,
		persisted:      persisted,
		resolveBad:     resolveBad,
		scoreHistogram: newMatchScoreHistogram(o11y),
	}
}

type CreateRecurringFromAgentInput struct {
	UserID string
	Intent intent.Intent
}

type CreateRecurringFromAgentResult struct {
	Persisted    bool
	Direction    string
	AmountCents  int64
	Frequency    string
	DayOfMonth   int
	CategoryPath string
	Description  string
}

func (uc *CreateRecurringFromAgent) Execute(ctx context.Context, in CreateRecurringFromAgentInput) (CreateRecurringFromAgentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.create_recurring_from_agent")
	defer span.End()

	if in.Intent.Kind() != intent.KindCreateRecurring {
		return CreateRecurringFromAgentResult{}, ErrLogTransactionInvalidIntent
	}
	if strings.TrimSpace(in.UserID) == "" {
		return CreateRecurringFromAgentResult{}, errors.New("agent: create recurring: user id vazio")
	}
	if in.Intent.AmountCents() <= 0 {
		return CreateRecurringFromAgentResult{}, errors.New("agent: create recurring: amount invalido")
	}

	direction := in.Intent.Direction()
	categoryKind := categoriesvo.KindExpense
	if direction == directionIncome {
		categoryKind = categoriesvo.KindIncome
	}

	hint := resolveHint(in.Intent.CategoryHint(), in.Intent.Merchant())
	if hint == "" {
		if direction != directionIncome {
			uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_hint"))
			return CreateRecurringFromAgentResult{}, ErrLogTransactionNoCategoryHint
		}
		hint = defaultIncomeHint
	}

	candidate, path, err := resolveCategoryCandidate(ctx, uc.resolver, uc.resolveBad, uc.scoreHistogram, hint, categoryKind)
	if err != nil {
		return CreateRecurringFromAgentResult{}, err
	}

	description := strings.TrimSpace(in.Intent.Merchant())
	if description == "" {
		description = path
	}

	sub := ""
	if subID := candidateSubcategoryUUID(candidate); subID != nil {
		sub = subID.String()
	}

	_, err = uc.creator.Execute(ctx, CreateRecurringCommand{
		UserID:         in.UserID,
		Direction:      direction,
		Description:    description,
		RootCategoryID: candidate.RootCategoryID.String(),
		SubcategoryID:  sub,
		AmountCents:    in.Intent.AmountCents(),
		Frequency:      in.Intent.Frequency(),
		DayOfMonth:     in.Intent.DayOfMonth(),
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "create_failed"))
		return CreateRecurringFromAgentResult{}, err
	}

	uc.persisted.Add(ctx, 1, observability.String("direction", direction))
	return CreateRecurringFromAgentResult{
		Persisted:    true,
		Direction:    direction,
		AmountCents:  in.Intent.AmountCents(),
		Frequency:    in.Intent.Frequency(),
		DayOfMonth:   in.Intent.DayOfMonth(),
		CategoryPath: path,
		Description:  description,
	}, nil
}
