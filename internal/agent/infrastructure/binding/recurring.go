package binding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	transactionsusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
)

type RecurringCreatorAdapter struct {
	uc *usecases.CreateRecurringFromAgent
}

func NewRecurringCreatorAdapter(uc *usecases.CreateRecurringFromAgent) *RecurringCreatorAdapter {
	return &RecurringCreatorAdapter{uc: uc}
}

func (a *RecurringCreatorAdapter) Execute(ctx context.Context, in appservices.RecurringCreatorInput) (appservices.RecurringCreatorResult, error) {
	result, err := a.uc.Execute(ctx, usecases.CreateRecurringFromAgentInput{UserID: in.UserID, Intent: in.Intent})
	if err != nil {
		return appservices.RecurringCreatorResult{}, translateRecurringError(err)
	}
	return appservices.RecurringCreatorResult{
		Persisted:    result.Persisted,
		Direction:    result.Direction,
		AmountCents:  result.AmountCents,
		Frequency:    result.Frequency,
		DayOfMonth:   result.DayOfMonth,
		CategoryPath: result.CategoryPath,
		Description:  result.Description,
	}, nil
}

type recurringTemplateCreateUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateRecurringTemplate) (transactionsoutput.RecurringTemplate, error)
}

type RecurringTemplateCreatorAdapter struct {
	uc recurringTemplateCreateUseCase
}

func NewRecurringTemplateCreatorAdapter(uc recurringTemplateCreateUseCase) *RecurringTemplateCreatorAdapter {
	return &RecurringTemplateCreatorAdapter{uc: uc}
}

func (a *RecurringTemplateCreatorAdapter) Execute(ctx context.Context, in usecases.CreateRecurringCommand) (usecases.CreateRecurringResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return usecases.CreateRecurringResult{}, fmt.Errorf("agent: recurring creator: user id: %w", err)
	}
	rootID, err := uuid.Parse(strings.TrimSpace(in.RootCategoryID))
	if err != nil {
		return usecases.CreateRecurringResult{}, fmt.Errorf("agent: recurring creator: category id: %w", err)
	}
	var subID *uuid.UUID
	if trimmed := strings.TrimSpace(in.SubcategoryID); trimmed != "" {
		parsed, parseErr := uuid.Parse(trimmed)
		if parseErr != nil {
			return usecases.CreateRecurringResult{}, fmt.Errorf("agent: recurring creator: subcategory id: %w", parseErr)
		}
		subID = &parsed
	}

	ctx = withWhatsAppPrincipal(ctx, userID)

	_, err = a.uc.Execute(ctx, transactionsinput.RawCreateRecurringTemplate{
		Direction:     in.Direction,
		PaymentMethod: recurringPaymentMethod(in.Direction),
		AmountCents:   in.AmountCents,
		Description:   in.Description,
		CategoryID:    rootID,
		SubcategoryID: subID,
		Frequency:     in.Frequency,
		DayOfMonth:    in.DayOfMonth,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return usecases.CreateRecurringResult{}, fmt.Errorf("agent: recurring creator: criar: %w", err)
	}
	return usecases.CreateRecurringResult{Persisted: true}, nil
}

func recurringPaymentMethod(direction string) string {
	if direction == "income" {
		return "ted"
	}
	return "pix"
}

type listRecurringTemplatesUseCase interface {
	Execute(ctx context.Context, activeOnly bool, cursor string, limit int) (transactionsusecases.RecurringTemplatePage, error)
}

type RecurringListerAdapter struct {
	uc    listRecurringTemplatesUseCase
	limit int
}

func NewRecurringListerAdapter(uc listRecurringTemplatesUseCase) *RecurringListerAdapter {
	return &RecurringListerAdapter{uc: uc, limit: 200}
}

func (a *RecurringListerAdapter) Execute(ctx context.Context, userID string) ([]appservices.RecurringView, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("agent: recurring lister: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, parsed)

	page, err := a.uc.Execute(ctx, true, "", a.limit)
	if err != nil {
		return nil, fmt.Errorf("agent: recurring lister: %w", err)
	}
	views := make([]appservices.RecurringView, 0, len(page.Templates))
	for _, t := range page.Templates {
		views = append(views, appservices.RecurringView{
			Direction:   t.Direction,
			AmountCents: t.AmountCents,
			Description: t.Description,
			Frequency:   t.Frequency,
			DayOfMonth:  t.DayOfMonth,
		})
	}
	return views, nil
}
