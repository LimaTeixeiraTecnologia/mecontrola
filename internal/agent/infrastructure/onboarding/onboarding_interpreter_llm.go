package onboarding

import (
	"context"
	"encoding/json"
	"math"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
)

func (i *onboardingInterpreter) ParseObjective(ctx context.Context, text string) (agentwf.ParsedObjective, error) {
	raw, ok := i.interpretStructured(ctx, "onboarding_objective_parse", objectiveParseSystemPrompt, objectiveParseSchema, text)
	if !ok {
		return i.parseObjectiveDeterministic(text), nil
	}
	switch strings.ToLower(raw.Action) {
	case "daily":
		return agentwf.ParsedObjective{DailyCommand: true}, nil
	case "clarify":
		return agentwf.ParsedObjective{Ambiguous: true}, nil
	case "save":
		objective := strings.TrimSpace(raw.Objective)
		if objective == "" {
			return agentwf.ParsedObjective{Ambiguous: true}, nil
		}
		return agentwf.ParsedObjective{Objective: objective}, nil
	default:
		return i.parseObjectiveDeterministic(text), nil
	}
}

func (i *onboardingInterpreter) ParseBudget(ctx context.Context, text string) (agentwf.ParsedBudget, error) {
	raw, ok := i.interpretStructured(ctx, "onboarding_budget_parse", budgetParseSystemPrompt, moneyParseSchema, text)
	if !ok {
		return i.parseBudgetDeterministic(text), nil
	}
	switch strings.ToLower(raw.Action) {
	case "daily":
		return agentwf.ParsedBudget{DailyCommand: true}, nil
	case "clarify":
		return agentwf.ParsedBudget{Ambiguous: true}, nil
	case "save":
		cents, valid := centsFromLLM(raw, text)
		if !valid {
			return agentwf.ParsedBudget{Ambiguous: true}, nil
		}
		return agentwf.ParsedBudget{IncomeCents: cents}, nil
	default:
		return i.parseBudgetDeterministic(text), nil
	}
}

func (i *onboardingInterpreter) ParseCards(ctx context.Context, text string, loop int) (agentwf.ParsedCards, error) {
	raw, ok := i.interpretStructured(ctx, "onboarding_cards_parse", cardsParseSystemPrompt, cardsParseSchema, text)
	if !ok {
		return i.parseCardsDeterministic(text, loop), nil
	}
	switch strings.ToLower(raw.Action) {
	case "daily":
		return agentwf.ParsedCards{DailyCommand: true}, nil
	case "skip":
		return agentwf.ParsedCards{Skip: true}, nil
	case "add_another":
		if loop == 0 {
			return agentwf.ParsedCards{AddAnother: true}, nil
		}
		return agentwf.ParsedCards{Ambiguous: true}, nil
	case "clarify":
		return agentwf.ParsedCards{Ambiguous: true}, nil
	case "save":
		nickname := strings.TrimSpace(raw.Nickname)
		if nickname == "" || raw.DueDay < 1 || raw.DueDay > 31 {
			return agentwf.ParsedCards{Ambiguous: true}, nil
		}
		return agentwf.ParsedCards{Nickname: nickname, DueDay: raw.DueDay}, nil
	default:
		return i.parseCardsDeterministic(text, loop), nil
	}
}

func (i *onboardingInterpreter) ParseCategoriesConfirm(ctx context.Context, text string) (bool, error) {
	raw, ok := i.interpretStructured(ctx, "onboarding_categories_parse", categoriesParseSystemPrompt, categoriesParseSchema, text)
	if !ok {
		return i.parseCategoriesConfirmDeterministic(text), nil
	}
	return strings.ToLower(raw.Action) == "confirm", nil
}

func (i *onboardingInterpreter) ParseValue(ctx context.Context, text string) (agentwf.ParsedValue, error) {
	raw, ok := i.interpretStructured(ctx, "onboarding_value_parse", valueParseSystemPrompt, moneyParseSchema, text)
	if !ok {
		return i.parseValueDeterministic(text), nil
	}
	switch strings.ToLower(raw.Action) {
	case "daily":
		return agentwf.ParsedValue{DailyCommand: true}, nil
	case "clarify":
		return agentwf.ParsedValue{Ambiguous: true}, nil
	case "save":
		cents, valid := centsFromLLM(raw, text)
		if !valid {
			return agentwf.ParsedValue{Ambiguous: true}, nil
		}
		return agentwf.ParsedValue{ValueCents: cents}, nil
	default:
		return i.parseValueDeterministic(text), nil
	}
}

func (i *onboardingInterpreter) parseCategoriesConfirmDeterministic(text string) bool {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandText(trimmed) {
		return false
	}
	return isConfirmation(strings.ToLower(trimmed))
}

type onboardingParse struct {
	Action      string  `json:"action"`
	Objective   string  `json:"objective"`
	AmountText  string  `json:"amount_text"`
	AmountReais float64 `json:"amount_reais"`
	Nickname    string  `json:"nickname"`
	DueDay      int     `json:"due_day"`
}

func (i *onboardingInterpreter) interpretStructured(ctx context.Context, name, system, schemaJSON, text string) (onboardingParse, bool) {
	if i.interpreter == nil {
		return onboardingParse{}, false
	}
	var schema map[string]any
	if json.Unmarshal([]byte(schemaJSON), &schema) != nil {
		return onboardingParse{}, false
	}
	resp, err := i.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: system,
		UserMessage:  text,
		JSONSchema: &interfaces.JSONSchemaSpec{
			Name:   name,
			Strict: true,
			Schema: schema,
		},
		MaxTokens: i.maxTokens,
	})
	if err != nil || len(resp.RawJSON) == 0 {
		return onboardingParse{}, false
	}
	var raw onboardingParse
	if json.Unmarshal(resp.RawJSON, &raw) != nil {
		return onboardingParse{}, false
	}
	if strings.TrimSpace(raw.Action) == "" {
		return onboardingParse{}, false
	}
	return raw, true
}

func centsFromLLM(raw onboardingParse, original string) (int64, bool) {
	if raw.AmountReais > 0 {
		cents := int64(math.Round(raw.AmountReais * 100))
		if cents > 0 {
			return cents, true
		}
	}
	if cents, ok := parseMoney(raw.AmountText); ok && cents > 0 {
		return cents, true
	}
	if cents, ok := parseMoney(original); ok && cents > 0 {
		return cents, true
	}
	return 0, false
}

const (
	objectiveParseSystemPrompt = `Você é o MeControla no onboarding. O usuário respondeu à pergunta sobre o objetivo financeiro dele.
Classifique a mensagem:
- action="save" e preencha "objective" com o objetivo em poucas palavras quando o usuário expressa um objetivo (ex: quitar dívidas, viajar, juntar reserva).
- action="daily" quando for um comando de operação diária (registrar gasto/receita, consultar saldo), não um objetivo.
- action="clarify" quando a mensagem for vazia, sem sentido como objetivo ou off-topic.`

	budgetParseSystemPrompt = `Você é o MeControla no onboarding. O usuário respondeu quanto tem disponível no orçamento mensal.
Classifique a mensagem:
- action="save" e preencha "amount_reais" com o valor em reais como número (ex: "4 mil"->4000, "mil e quinhentos"->1500, "R$ 5.250,50"->5250.50).
- action="daily" quando for comando de operação diária, não uma renda.
- action="clarify" quando não houver valor reconhecível.`

	valueParseSystemPrompt = `Você é o MeControla no onboarding. O usuário informou quanto deseja destinar para uma categoria do planejamento.
Classifique a mensagem:
- action="save" e preencha "amount_reais" com o valor em reais como número (ex: "2 mil"->2000, "trezentos"->300, "R$ 1.250,90"->1250.90).
- action="daily" quando for comando de operação diária.
- action="clarify" quando não houver valor reconhecível.`

	cardsParseSystemPrompt = `Você é o MeControla no onboarding coletando cartões. Peça apenas apelido e dia de vencimento.
Classifique a mensagem:
- action="save" e preencha "nickname" e "due_day" (1 a 31) quando o usuário informar apelido e dia de vencimento.
- action="skip" quando o usuário disser que não usa cartão.
- action="add_another" quando o usuário apenas confirmar que usa cartão sem dar os dados ainda.
- action="daily" quando for comando de operação diária.
- action="clarify" quando faltar apelido ou dia de vencimento válido.`

	categoriesParseSystemPrompt = `Você é o MeControla no onboarding. Você apresentou as 5 categorias e perguntou se faz sentido.
Classifique a mensagem:
- action="confirm" quando o usuário concordar/entender.
- action="clarify" quando o usuário tiver dúvida, não entender ou responder algo que não seja confirmação.`

	objectiveParseSchema = `{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["save", "daily", "clarify"]},
    "objective": {"type": "string", "maxLength": 280}
  },
  "required": ["action", "objective"],
  "additionalProperties": false
}`

	moneyParseSchema = `{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["save", "daily", "clarify"]},
    "amount_reais": {"type": "number", "minimum": 0}
  },
  "required": ["action", "amount_reais"],
  "additionalProperties": false
}`

	cardsParseSchema = `{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["save", "skip", "add_another", "daily", "clarify"]},
    "nickname": {"type": "string", "maxLength": 60},
    "due_day": {"type": "integer", "minimum": 0, "maximum": 31}
  },
  "required": ["action", "nickname", "due_day"],
  "additionalProperties": false
}`

	categoriesParseSchema = `{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["confirm", "clarify"]}
  },
  "required": ["action"],
  "additionalProperties": false
}`
)
