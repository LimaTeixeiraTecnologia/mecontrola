package prompting

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

//go:embed parse_intent.system.tmpl
var parseIntentSystemRaw string

//go:embed parse_intent.user.tmpl
var parseIntentUserRaw string

//go:embed persona.system.tmpl
var personaSystemRaw string

//go:embed budgets.system.tmpl
var budgetsSystemRaw string

//go:embed working_memory.system.tmpl
var workingMemorySystemRaw string

var ErrUserTextEmpty = errors.New("agent.application.prompting: user text is empty")

var parseIntentUserTpl = template.Must(template.New("parse_intent.user").Parse(parseIntentUserRaw))

var personaSystemTpl = template.Must(template.New("persona.system").Parse(personaSystemRaw))

var budgetsSystemTpl = template.Must(template.New("budgets.system").Parse(budgetsSystemRaw))

var workingMemorySystemTpl = template.Must(template.New("working_memory.system").Parse(workingMemorySystemRaw))

type ParseIntentUserData struct {
	UserText string
}

type PersonaSystemData struct {
	JourneyHint        string
	WorkingMemory      string
	ObservationContext string
}

type BudgetsPersonaData struct {
	JourneyHint string
}

type WorkingMemorySystemData struct {
	WorkingMemory string
}

func RenderSystem() (string, error) {
	if strings.TrimSpace(parseIntentSystemRaw) == "" {
		return "", fmt.Errorf("agent.application.prompting: system template is empty")
	}
	return parseIntentSystemRaw, nil
}

func RenderPersonaSystem(data PersonaSystemData) (string, error) {
	if strings.TrimSpace(personaSystemRaw) == "" {
		return "", fmt.Errorf("agent.application.prompting: persona template is empty")
	}
	var buf bytes.Buffer
	if err := personaSystemTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent.application.prompting: execute persona template: %w", err)
	}
	return buf.String(), nil
}

func RenderBudgetsPersona(data BudgetsPersonaData) (string, error) {
	if strings.TrimSpace(budgetsSystemRaw) == "" {
		return "", fmt.Errorf("agent.application.prompting: budgets template is empty")
	}
	var buf bytes.Buffer
	if err := budgetsSystemTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent.application.prompting: execute budgets template: %w", err)
	}
	return buf.String(), nil
}

func RenderUser(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ErrUserTextEmpty
	}
	var buf bytes.Buffer
	if err := parseIntentUserTpl.Execute(&buf, ParseIntentUserData{UserText: trimmed}); err != nil {
		return "", fmt.Errorf("agent.application.prompting: execute user template: %w", err)
	}
	return buf.String(), nil
}

func RenderWorkingMemorySystem(data WorkingMemorySystemData) (string, error) {
	var buf bytes.Buffer
	if err := workingMemorySystemTpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent.application.prompting: execute working_memory template: %w", err)
	}
	return buf.String(), nil
}

func ParseIntentJSONSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           parseIntentSchemaProperties(true),
		"required":             append(parseIntentSchemaRequired(), "plan"),
		"additionalProperties": false,
	}
}

func parseIntentSchemaProperties(withPlan bool) map[string]any {
	properties := map[string]any{
		"kind": map[string]any{
			"type": "string",
			"enum": []string{
				"record_expense",
				"record_income",
				"query_category",
				"query_goal",
				"query_card",
				"monthly_summary",
				"how_am_i_doing",
				"configure_budget",
				"record_card_purchase",
				"list_transactions",
				"delete_last_transaction",
				"edit_last_transaction",
				"create_recurring",
				"list_recurring",
				"list_cards",
				"create_card",
				"count_cards",
				"update_card",
				"delete_card",
				"edit_category_percentage",
				"query_income_summary",
				"budget_recurrence",
				"delete_transaction_by_ref",
				"edit_transaction_by_ref",
				"budget_details",
				"list_categories",
				"classify_category",
				"unknown",
			},
		},
		"amount_cents":       map[string]any{"type": "integer", "minimum": 0},
		"merchant":           map[string]any{"type": "string", "maxLength": 120},
		"category_hint":      map[string]any{"type": "string", "maxLength": 80},
		"payment_method":     map[string]any{"type": "string", "enum": []string{"", "pix", "credit", "debit", "cash", "transfer", "boleto", "unknown"}},
		"card_hint":          map[string]any{"type": "string", "maxLength": 80},
		"category_name":      map[string]any{"type": "string", "maxLength": 120},
		"goal_name":          map[string]any{"type": "string", "maxLength": 120},
		"card_name":          map[string]any{"type": "string", "maxLength": 120},
		"nickname":           map[string]any{"type": "string", "maxLength": 120},
		"ref_month":          map[string]any{"type": "string", "maxLength": 7},
		"raw_text":           map[string]any{"type": "string", "maxLength": 4096},
		"installments":       map[string]any{"type": "integer", "minimum": 0, "maximum": 24},
		"direction":          map[string]any{"type": "string", "enum": []string{"", "income", "outcome"}},
		"frequency":          map[string]any{"type": "string", "enum": []string{"", "monthly", "yearly"}},
		"day_of_month":       map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		"closing_day":        map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		"due_day":            map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		"limit_cents":        map[string]any{"type": "integer", "minimum": 0},
		"percentage":         map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		"new_nickname":       map[string]any{"type": "string", "maxLength": 120},
		"new_name":           map[string]any{"type": "string", "maxLength": 120},
		"new_closing_day":    map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		"new_due_day":        map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		"months":             map[string]any{"type": "integer", "minimum": 0, "maximum": 12},
		"source_competence":  map[string]any{"type": "string", "maxLength": 7},
		"search_query":       map[string]any{"type": "string", "maxLength": 120},
		"budget_total_cents": map[string]any{"type": "integer", "minimum": 0},
		"budget_allocations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"root_slug": map[string]any{
						"type": "string",
						"enum": []string{
							"expense.custo_fixo",
							"expense.conhecimento",
							"expense.prazeres",
							"expense.metas",
							"expense.liberdade_financeira",
						},
					},
					"basis_points": map[string]any{"type": "integer"},
				},
				"required":             []string{"root_slug", "basis_points"},
				"additionalProperties": false,
			},
		},
		"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
	}
	if withPlan {
		properties["plan"] = map[string]any{
			"type": []string{"array", "null"},
			"items": map[string]any{
				"type":                 "object",
				"properties":           parseIntentSchemaProperties(false),
				"required":             parseIntentSchemaRequired(),
				"additionalProperties": false,
			},
		}
	}
	return properties
}

func parseIntentSchemaRequired() []string {
	return []string{
		"kind",
		"amount_cents",
		"merchant",
		"category_hint",
		"payment_method",
		"card_hint",
		"category_name",
		"goal_name",
		"card_name",
		"nickname",
		"ref_month",
		"raw_text",
		"installments",
		"direction",
		"frequency",
		"day_of_month",
		"closing_day",
		"due_day",
		"limit_cents",
		"percentage",
		"new_nickname",
		"new_name",
		"new_closing_day",
		"new_due_day",
		"months",
		"source_competence",
		"search_query",
		"budget_total_cents",
		"budget_allocations",
		"confidence",
	}
}
