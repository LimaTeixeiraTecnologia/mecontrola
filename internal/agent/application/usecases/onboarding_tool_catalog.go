package usecases

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
)

const (
	ToolSaveOnboardingObjective    = "save_onboarding_objective"
	ToolSaveOnboardingIncome       = "save_onboarding_income"
	ToolSaveOnboardingCard         = "save_onboarding_card"
	ToolSaveOnboardingBudgetSplits = "save_onboarding_budget_splits"
	ToolCompleteOnboardingSession  = "complete_onboarding_session"
)

var onboardingBudgetSlugs = []string{
	"expense.custo_fixo",
	"expense.conhecimento",
	"expense.prazeres",
	"expense.metas",
	"expense.liberdade_financeira",
}

func OnboardingToolCatalog() []interfaces.ToolSpec {
	specs := []interfaces.ToolSpec{
		{
			Name:        ToolSaveOnboardingObjective,
			Description: "Salva o objetivo financeiro principal do usuario no onboarding (ex: fazer uma viagem, quitar dividas).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"objective": map[string]any{
						"type":        "string",
						"description": "Objetivo principal em linguagem natural.",
						"maxLength":   280,
					},
					"objective_profile": map[string]any{
						"type":        "string",
						"description": "Perfil de objetivo identificado pelo LLM (opcional).",
						"enum":        []string{"payoff_debt", "emergency_fund", "invest", "specific_goal", "organize_spending"},
					},
				},
				"required":             []string{"objective"},
				"additionalProperties": false,
			},
		},
		{
			Name:        ToolSaveOnboardingIncome,
			Description: "Salva o orcamento mensal do usuario em centavos (ex: R$ 5.000,00 = 500000).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"income_cents": map[string]any{
						"type":        "integer",
						"description": "Orcamento mensal em centavos.",
						"minimum":     0,
					},
				},
				"required":             []string{"income_cents"},
				"additionalProperties": false,
			},
		},
		{
			Name:        ToolSaveOnboardingCard,
			Description: "Cadastra um cartao de credito no onboarding pedindo apenas apelido e dia de fechamento da fatura.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nickname": map[string]any{
						"type":        "string",
						"description": "Apelido do cartao (ex: nubank, itau roxinho).",
						"maxLength":   32,
					},
					"closing_day": map[string]any{
						"type":        "integer",
						"description": "Dia do mes em que a fatura fecha (1 a 31).",
						"minimum":     1,
						"maximum":     31,
					},
				},
				"required":             []string{"nickname", "closing_day"},
				"additionalProperties": false,
			},
		},
		{
			Name:        ToolSaveOnboardingBudgetSplits,
			Description: "Salva a distribuicao do orcamento por categoria em VALORES EM REAIS (centavos). A soma deve fechar o orcamento mensal (100%).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"allocations": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"root_slug": map[string]any{
									"type": "string",
									"enum": onboardingBudgetSlugs,
								},
								"amount_cents": map[string]any{
									"type":        "integer",
									"description": "Valor reservado para a categoria em centavos.",
									"minimum":     0,
								},
							},
							"required":             []string{"root_slug", "amount_cents"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"allocations"},
				"additionalProperties": false,
			},
		},
		{
			Name:        ToolCompleteOnboardingSession,
			Description: "Conclui o onboarding e ativa o usuario. So funciona apos um primeiro lancamento bem-sucedido.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
	}

	for _, t := range AgentToolCatalog() {
		if t.Name == toolRecordTransaction {
			specs = append(specs, t)
			break
		}
	}
	return specs
}
