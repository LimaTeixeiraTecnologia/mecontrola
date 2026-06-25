package usecases

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"

var onboardingPhaseSchemas = map[string]*interfaces.JSONSchemaSpec{
	OnbPhaseObjective: {
		Name:   "onboarding_objective",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{ToolSaveOnboardingObjective, "clarify"},
				},
				"objective":         map[string]any{"type": "string", "maxLength": 280},
				"objective_profile": map[string]any{"type": "string", "enum": []string{"", "payoff_debt", "emergency_fund", "invest", "specific_goal", "organize_spending"}},
				"reply":             map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "objective", "objective_profile", "reply"},
			"additionalProperties": false,
		},
	},
	OnbPhaseBudget: {
		Name:   "onboarding_income",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{ToolSaveOnboardingIncome, "clarify"},
				},
				"income_cents": map[string]any{"type": "integer"},
				"reply":        map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "income_cents", "reply"},
			"additionalProperties": false,
		},
	},
	OnbPhaseCards: {
		Name:   "onboarding_card",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{ToolSaveOnboardingCard, "clarify"},
				},
				"cards": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"nickname":    map[string]any{"type": "string", "maxLength": 32},
							"closing_day": map[string]any{"type": "integer"},
						},
						"required":             []string{"nickname", "closing_day"},
						"additionalProperties": false,
					},
				},
				"reply": map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "cards", "reply"},
			"additionalProperties": false,
		},
	},
	OnbPhaseFinancialPlan: {
		Name:   "onboarding_budget_splits",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{ToolSaveOnboardingBudgetSplits, "clarify"},
				},
				"allocations": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"root_slug":    map[string]any{"type": "string", "enum": onboardingBudgetSlugs},
							"amount_cents": map[string]any{"type": "integer"},
						},
						"required":             []string{"root_slug", "amount_cents"},
						"additionalProperties": false,
					},
				},
				"reply": map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "allocations", "reply"},
			"additionalProperties": false,
		},
	},
	OnbPhaseFirstTx: {
		Name:   "onboarding_first_tx",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []string{toolRecordTransaction, "clarify"},
				},
				"direction":     map[string]any{"type": "string", "enum": []string{"income", "outcome", ""}},
				"amount_cents":  map[string]any{"type": "integer"},
				"merchant":      map[string]any{"type": "string", "maxLength": 120},
				"category_hint": map[string]any{"type": "string", "maxLength": 80},
				"reply":         map[string]any{"type": "string", "maxLength": 1000},
			},
			"required":             []string{"action", "direction", "amount_cents", "merchant", "category_hint", "reply"},
			"additionalProperties": false,
		},
	},
}

func onboardingPhaseSchema(phase string) (*interfaces.JSONSchemaSpec, bool) {
	s, ok := onboardingPhaseSchemas[phase]
	return s, ok
}
