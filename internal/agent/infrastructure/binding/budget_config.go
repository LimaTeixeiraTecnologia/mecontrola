package binding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

const sessionTTL = 30 * time.Minute

type budgetConversationUseCase interface {
	Execute(ctx context.Context, in usecases.ConfigureBudgetInput) (usecases.ConfigureBudgetOutput, error)
}

type BudgetConversationAdapter struct {
	uc budgetConversationUseCase
}

func NewBudgetConversationAdapter(uc budgetConversationUseCase) *BudgetConversationAdapter {
	return &BudgetConversationAdapter{uc: uc}
}

func (a *BudgetConversationAdapter) Configure(ctx context.Context, text string, draft budgetdraft.Draft) (appservices.BudgetConversationResult, error) {
	out, err := a.uc.Execute(ctx, usecases.ConfigureBudgetInput{Text: text, Draft: draft})
	if err != nil {
		return appservices.BudgetConversationResult{}, fmt.Errorf("agent: budget conversation: %w", err)
	}
	return appservices.BudgetConversationResult{
		Draft:    out.Draft,
		Complete: out.Complete,
		Reply:    out.Reply,
	}, nil
}

type createBudgetUseCase interface {
	Execute(ctx context.Context, in budgetsinput.CreateBudgetInput) (budgetsoutput.BudgetOutput, error)
}

type activateBudgetUseCase interface {
	Execute(ctx context.Context, in budgetsinput.ActivateBudgetInput) (budgetsoutput.BudgetOutput, error)
}

type BudgetConfigCommitterAdapter struct {
	createUC   createBudgetUseCase
	activateUC activateBudgetUseCase
}

func NewBudgetConfigCommitterAdapter(createUC createBudgetUseCase, activateUC activateBudgetUseCase) *BudgetConfigCommitterAdapter {
	return &BudgetConfigCommitterAdapter{createUC: createUC, activateUC: activateUC}
}

func (a *BudgetConfigCommitterAdapter) Commit(ctx context.Context, userID uuid.UUID, draft budgetdraft.Draft) (string, error) {
	competence := draft.Competence()
	if competence == "" {
		competence = time.Now().UTC().Format("2006-01")
	}

	allocations := make([]budgetsinput.AllocationInput, 0, len(draft.Allocations()))
	for slug, bp := range draft.Allocations() {
		allocations = append(allocations, budgetsinput.AllocationInput{RootSlug: slug, BasisPoints: bp})
	}

	_, err := a.createUC.Execute(ctx, budgetsinput.CreateBudgetInput{
		UserID:      userID.String(),
		Competence:  competence,
		TotalCents:  draft.TotalCents(),
		Allocations: allocations,
	})
	if err != nil {
		if errors.Is(err, budgetsinterfaces.ErrBudgetConflict) {
			return "Já existe um orçamento neste mês. Quer substituir? Me confirme respondendo 'substituir'.", fmt.Errorf("agent: budget commit create: %w", err)
		}
		return "Não consegui criar seu orçamento agora. Pode tentar de novo em instantes?", fmt.Errorf("agent: budget commit create: %w", err)
	}

	out, err := a.activateUC.Execute(ctx, budgetsinput.ActivateBudgetInput{
		UserID:     userID.String(),
		Competence: competence,
	})
	if err != nil {
		if errors.Is(err, entities.ErrBudgetAllocationSumMustBe10000) {
			return "As porcentagens precisam somar exatamente 100%. Pode revisar as alocações?", fmt.Errorf("agent: budget commit activate: %w", err)
		}
		if errors.Is(err, budgetsinterfaces.ErrBudgetConflict) {
			return "Já existe um orçamento ativo neste mês. Quer substituir? Me confirme respondendo 'substituir'.", fmt.Errorf("agent: budget commit activate: %w", err)
		}
		return "Criei seu rascunho, mas não consegui ativar agora. Pode tentar de novo em instantes?", fmt.Errorf("agent: budget commit activate: %w", err)
	}

	return formatBudgetActivated(out.Competence, out.TotalCents), nil
}

type BudgetSessionGatewayAdapter struct {
	repo appinterfaces.AgentSessionRepository
	unit uow.UnitOfWork
}

func NewBudgetSessionGatewayAdapter(repo appinterfaces.AgentSessionRepository, unit uow.UnitOfWork) *BudgetSessionGatewayAdapter {
	return &BudgetSessionGatewayAdapter{repo: repo, unit: unit}
}

func (a *BudgetSessionGatewayAdapter) Load(ctx context.Context, userID uuid.UUID, channel string) (budgetdraft.Draft, bool, error) {
	record, err := a.repo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrAgentSessionNotFound) {
			return budgetdraft.Draft{}, false, nil
		}
		return budgetdraft.Draft{}, false, fmt.Errorf("agent: budget session load: %w", err)
	}
	if !usecases.IsBudgetConfigPending(record.PendingAction) {
		return budgetdraft.Draft{}, false, nil
	}
	draft, err := usecases.DecodeBudgetDraft(record.PendingAction)
	if err != nil {
		return budgetdraft.Draft{}, false, fmt.Errorf("agent: budget session decode: %w", err)
	}
	return draft, true, nil
}

func (a *BudgetSessionGatewayAdapter) Save(ctx context.Context, userID uuid.UUID, channel string, draft budgetdraft.Draft) error {
	pending, err := usecases.EncodeBudgetDraft(draft)
	if err != nil {
		return fmt.Errorf("agent: budget session encode: %w", err)
	}
	now := time.Now().UTC()
	record := appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		PendingAction: pending,
		RecentTurns:   []byte("[]"),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(sessionTTL),
	}
	persist := func(ctx context.Context, db database.DBTX) error {
		return a.repo.Upsert(ctx, record)
	}
	if a.unit == nil {
		if err := persist(ctx, nil); err != nil {
			return fmt.Errorf("agent: budget session save: %w", err)
		}
		return nil
	}
	if err := a.unit.Do(ctx, persist); err != nil {
		return fmt.Errorf("agent: budget session save: %w", err)
	}
	return nil
}

func (a *BudgetSessionGatewayAdapter) Clear(ctx context.Context, userID uuid.UUID, channel string) error {
	now := time.Now().UTC()
	record := appinterfaces.AgentSessionRecord{
		ID:            uuid.New(),
		UserID:        userID,
		Channel:       channel,
		PendingAction: []byte("{}"),
		RecentTurns:   []byte("[]"),
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(-time.Minute),
	}
	persist := func(ctx context.Context, db database.DBTX) error {
		return a.repo.Upsert(ctx, record)
	}
	if a.unit == nil {
		if err := persist(ctx, nil); err != nil {
			return fmt.Errorf("agent: budget session clear: %w", err)
		}
		return nil
	}
	if err := a.unit.Do(ctx, persist); err != nil {
		return fmt.Errorf("agent: budget session clear: %w", err)
	}
	return nil
}

func formatBudgetActivated(competence string, totalCents int64) string {
	return fmt.Sprintf("✅ Orçamento de %s ativado! Total planejado: %s. Já estou acompanhando seus gastos por categoria. 🎯",
		competence, formatBudgetCents(totalCents))
}

func formatBudgetCents(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	centavos := cents % 100
	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("R$ %s%d,%02d", sign, reais, centavos)
}
