package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
)

var ErrConfigureBudgetEmptyText = errors.New("agent.usecase.configure_budget: text is empty")

type ConfigureBudgetInput struct {
	Text  string
	Draft budgetdraft.Draft
}

type ConfigureBudgetOutput struct {
	Draft    budgetdraft.Draft
	Complete bool
	Reply    string
}

type ConfigureBudgetConversation struct {
	interpreter   IntentInterpreter
	o11y          observability.Observability
	turnsTotal    observability.Counter
	mergeFailures observability.Counter
	schema        *interfaces.JSONSchemaSpec
}

func NewConfigureBudgetConversation(interpreter IntentInterpreter, o11y observability.Observability) (*ConfigureBudgetConversation, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.usecase.configure_budget: interpreter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.configure_budget: observability is nil")
	}
	turnsTotal := o11y.Metrics().Counter(
		"agent_budget_config_turns_total",
		"Total de turnos do fluxo multi-turno de configuração de orçamento por outcome",
		"1",
	)
	mergeFailures := o11y.Metrics().Counter(
		"agent_budget_config_merge_failed_total",
		"Total de falhas ao mesclar dados extraídos no rascunho de orçamento por motivo",
		"1",
	)
	schema := &interfaces.JSONSchemaSpec{
		Name:   "mecontrola_budget_config",
		Strict: true,
		Schema: prompting.BudgetConfigJSONSchema(),
	}
	return &ConfigureBudgetConversation{
		interpreter:   interpreter,
		o11y:          o11y,
		turnsTotal:    turnsTotal,
		mergeFailures: mergeFailures,
		schema:        schema,
	}, nil
}

func (uc *ConfigureBudgetConversation) Execute(ctx context.Context, input ConfigureBudgetInput) (ConfigureBudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.configure_budget")
	defer span.End()

	trimmed := strings.TrimSpace(input.Text)
	if trimmed == "" {
		return ConfigureBudgetOutput{}, ErrConfigureBudgetEmptyText
	}

	system, err := prompting.RenderBudgetConfigSystem()
	if err != nil {
		return ConfigureBudgetOutput{}, fmt.Errorf("agent.usecase.configure_budget: render system: %w", err)
	}

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: system,
		UserMessage:  trimmed,
		JSONSchema:   uc.schema,
	})
	if err != nil {
		span.RecordError(err)
		uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "provider_error"))
		return ConfigureBudgetOutput{}, fmt.Errorf("agent.usecase.configure_budget: provider: %w", err)
	}

	change, decodeErr := decodeBudgetChange(resp.RawJSON)
	if decodeErr != nil {
		span.RecordError(decodeErr)
		uc.mergeFailures.Add(ctx, 1, observability.String("reason", "decode"))
		uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "decode_error"))
		return ConfigureBudgetOutput{
			Draft: input.Draft,
			Reply: budgetConfigClarifyText(input.Draft),
		}, nil
	}

	merged, mergeErr := input.Draft.Merge(change)
	if mergeErr != nil {
		span.RecordError(mergeErr)
		uc.mergeFailures.Add(ctx, 1, observability.String("reason", "merge"))
		uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "merge_error"))
		return ConfigureBudgetOutput{
			Draft: input.Draft,
			Reply: budgetConfigClarifyText(input.Draft),
		}, nil
	}

	if merged.IsComplete() {
		uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "complete"))
		return ConfigureBudgetOutput{Draft: merged, Complete: true}, nil
	}

	uc.turnsTotal.Add(ctx, 1, observability.String("outcome", "incomplete"))
	return ConfigureBudgetOutput{Draft: merged, Reply: budgetConfigClarifyText(merged)}, nil
}

type budgetChangeDTO struct {
	TotalCents  int64                 `json:"total_cents"`
	Allocations []budgetAllocationDTO `json:"allocations"`
}

type budgetAllocationDTO struct {
	RootSlug    string `json:"root_slug"`
	BasisPoints int    `json:"basis_points"`
}

func decodeBudgetChange(raw []byte) (budgetdraft.Change, error) {
	cleaned := stripFences(raw)
	if len(cleaned) == 0 {
		return budgetdraft.Change{}, fmt.Errorf("agent.usecase.configure_budget: empty payload")
	}
	var dto budgetChangeDTO
	if err := json.Unmarshal(cleaned, &dto); err != nil {
		return budgetdraft.Change{}, fmt.Errorf("agent.usecase.configure_budget: unmarshal: %w", err)
	}
	allocations := make(map[string]int, len(dto.Allocations))
	for _, item := range dto.Allocations {
		slug := strings.TrimSpace(item.RootSlug)
		if slug == "" || item.BasisPoints <= 0 {
			continue
		}
		allocations[slug] = item.BasisPoints
	}
	return budgetdraft.Change{TotalCents: dto.TotalCents, Allocations: allocations}, nil
}

func budgetConfigClarifyText(draft budgetdraft.Draft) string {
	if draft.TotalCents() <= 0 {
		return "Beleza! Vamos montar seu orçamento. Qual é a sua renda mensal? Me diga o valor."
	}
	missing := draft.MissingSlugs()
	remaining := draft.RemainingBasisPoints()
	if len(missing) == 0 {
		if remaining > 0 {
			return fmt.Sprintf("Quase lá! Ainda faltam %d%% para fechar 100%%. Me diga como distribuir o restante.", remaining/100)
		}
		return "Quase lá! As porcentagens passaram de 100%. Pode me ajustar para somar exatamente 100%?"
	}
	labels := make([]string, 0, len(missing))
	for _, slug := range missing {
		labels = append(labels, budgetSlugLabel(slug))
	}
	if remaining > 0 {
		return fmt.Sprintf("Anotei! Ainda faltam %d%% para 100%%. Quais percentuais para: %s?",
			remaining/100, strings.Join(labels, ", "))
	}
	return fmt.Sprintf("Anotei! Faltam definir as categorias: %s. Qual percentual para cada uma?",
		strings.Join(labels, ", "))
}

func budgetSlugLabel(slug string) string {
	switch slug {
	case budgetdraft.SlugCustoFixo:
		return "Custo Fixo"
	case budgetdraft.SlugConhecimento:
		return "Conhecimento"
	case budgetdraft.SlugPrazeres:
		return "Prazeres"
	case budgetdraft.SlugMetas:
		return "Metas"
	case budgetdraft.SlugLiberdadeFinanceira:
		return "Liberdade Financeira"
	default:
		return slug
	}
}
