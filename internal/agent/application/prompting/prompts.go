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

var ErrUserTextEmpty = errors.New("agent.application.prompting: user text is empty")

var parseIntentUserTpl = template.Must(template.New("parse_intent.user").Parse(parseIntentUserRaw))

var personaSystemTpl = template.Must(template.New("persona.system").Parse(personaSystemRaw))

var budgetsSystemTpl = template.Must(template.New("budgets.system").Parse(budgetsSystemRaw))

type ParseIntentUserData struct {
	UserText string
}

type PersonaSystemData struct {
	JourneyHint string
}

type BudgetsPersonaData struct {
	JourneyHint string
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

func ParseIntentJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind": map[string]any{
				"type": "string",
				"enum": []string{
					"log_expense",
					"log_income",
					"query_category",
					"query_goal",
					"query_card",
					"monthly_summary",
					"how_am_i_doing",
					"configure_budget",
					"log_card_purchase",
					"list_transactions",
					"delete_last_transaction",
					"edit_last_transaction",
					"create_recurring",
					"list_recurring",
					"list_cards",
					"unknown",
				},
			},
			"amount_cents":   map[string]any{"type": "integer", "minimum": 0},
			"merchant":       map[string]any{"type": "string", "maxLength": 120},
			"category_hint":  map[string]any{"type": "string", "maxLength": 80},
			"payment_method": map[string]any{"type": "string", "enum": []string{"", "pix", "credit", "debit", "cash", "transfer", "boleto", "unknown"}},
			"card_hint":      map[string]any{"type": "string", "maxLength": 80},
			"category_name":  map[string]any{"type": "string", "maxLength": 120},
			"goal_name":      map[string]any{"type": "string", "maxLength": 120},
			"card_name":      map[string]any{"type": "string", "maxLength": 120},
			"ref_month":      map[string]any{"type": "string", "maxLength": 7},
			"raw_text":       map[string]any{"type": "string", "maxLength": 4096},
			"installments":   map[string]any{"type": "integer", "minimum": 0, "maximum": 24},
			"direction":      map[string]any{"type": "string", "enum": []string{"", "income", "outcome"}},
			"frequency":      map[string]any{"type": "string", "enum": []string{"", "monthly", "yearly"}},
			"day_of_month":   map[string]any{"type": "integer", "minimum": 0, "maximum": 31},
		},
		"required":             []string{"kind"},
		"additionalProperties": false,
	}
}
