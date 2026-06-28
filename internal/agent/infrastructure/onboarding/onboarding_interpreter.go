package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
)

type onboardingInterpreter struct {
	interpreter appusecases.IntentInterpreter
	maxTokens   int
}

func NewOnboardingInterpreter(interpreter appusecases.IntentInterpreter, maxTokens int) agentwf.OnboardingInterpreter {
	if interpreter == nil {
		return nil
	}
	if maxTokens <= 0 {
		maxTokens = 512
	}
	return &onboardingInterpreter{interpreter: interpreter, maxTokens: maxTokens}
}

func (i *onboardingInterpreter) RenderWelcome(ctx context.Context) string {
	return i.render(ctx, welcomeSystemPrompt, welcomeCue)
}

func (i *onboardingInterpreter) RenderObjective(ctx context.Context) string {
	return i.render(ctx, objectiveSystemPrompt, objectiveCue)
}

func (i *onboardingInterpreter) RenderBudget(ctx context.Context) string {
	return i.render(ctx, budgetSystemPrompt, budgetCue)
}

func (i *onboardingInterpreter) RenderCards(ctx context.Context, loop int) string {
	if loop == 0 {
		return i.render(ctx, cardsSystemPrompt, cardsFirstCue)
	}
	return i.render(ctx, cardsSystemPrompt, cardsNextCue)
}

func (i *onboardingInterpreter) RenderCategories(ctx context.Context) string {
	return i.render(ctx, categoriesSystemPrompt, categoriesCue)
}

func (i *onboardingInterpreter) RenderValues(ctx context.Context, pending string) string {
	return i.render(ctx, valuesSystemPrompt, valuesCue(pending))
}

func (i *onboardingInterpreter) RenderSummary(ctx context.Context, state agentwf.SummaryState) string {
	return i.render(ctx, summarySystemPrompt, summaryCue(state))
}

func (i *onboardingInterpreter) RenderRetry(ctx context.Context, phase string) string {
	return i.render(ctx, retrySystemPrompt, retryCue(phase))
}

func (i *onboardingInterpreter) RenderDailyRedirect(ctx context.Context, phase string) string {
	return i.render(ctx, dailyRedirectSystemPrompt, dailyRedirectCue(phase))
}

func (i *onboardingInterpreter) RenderConclusion(ctx context.Context) string {
	return i.render(ctx, conclusionSystemPrompt, conclusionCue)
}

func (i *onboardingInterpreter) parseObjectiveDeterministic(text string) agentwf.ParsedObjective {
	trimmed := strings.TrimSpace(text)
	return agentwf.ParsedObjective{
		Objective:    trimmed,
		DailyCommand: isDailyCommandText(trimmed),
		Ambiguous:    trimmed == "",
	}
}

func (i *onboardingInterpreter) parseBudgetDeterministic(text string) agentwf.ParsedBudget {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandText(trimmed) {
		return agentwf.ParsedBudget{DailyCommand: true}
	}
	cents, ok := parseMoney(trimmed)
	if !ok {
		return agentwf.ParsedBudget{Ambiguous: true}
	}
	return agentwf.ParsedBudget{IncomeCents: cents}
}

func (i *onboardingInterpreter) parseCardsDeterministic(text string, loop int) agentwf.ParsedCards {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandText(trimmed) {
		return agentwf.ParsedCards{DailyCommand: true}
	}
	if isNegation(lower) {
		return agentwf.ParsedCards{Skip: true}
	}
	if loop == 0 && isConfirmation(lower) {
		return agentwf.ParsedCards{AddAnother: true}
	}
	nickname, dueDay, foundDay := extractCard(trimmed)
	if nickname == "" || !foundDay || dueDay < 1 || dueDay > 31 {
		return agentwf.ParsedCards{Ambiguous: true}
	}
	return agentwf.ParsedCards{Nickname: nickname, DueDay: dueDay}
}

func (i *onboardingInterpreter) parseValueDeterministic(text string) agentwf.ParsedValue {
	trimmed := strings.TrimSpace(text)
	if isDailyCommandText(trimmed) {
		return agentwf.ParsedValue{DailyCommand: true}
	}
	cents, ok := parseMoney(trimmed)
	if !ok {
		return agentwf.ParsedValue{Ambiguous: true}
	}
	return agentwf.ParsedValue{ValueCents: cents}
}

func (i *onboardingInterpreter) ParseSummary(ctx context.Context, text string) (agentwf.ParsedSummary, error) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if isDailyCommandText(trimmed) {
		return agentwf.ParsedSummary{DailyCommand: true}, nil
	}
	if parsed, ok := i.parseSummaryWithLLM(ctx, text); ok {
		return parsed, nil
	}
	if isConfirmation(lower) {
		return agentwf.ParsedSummary{Confirm: true}, nil
	}
	if isNegation(lower) {
		return agentwf.ParsedSummary{Ambiguous: true}, nil
	}
	if strings.Contains(lower, "corrig") || strings.Contains(lower, "errado") || strings.Contains(lower, "altera") {
		return parseCorrection(trimmed), nil
	}
	return agentwf.ParsedSummary{}, nil
}

func (i *onboardingInterpreter) parseSummaryWithLLM(ctx context.Context, text string) (agentwf.ParsedSummary, bool) {
	if i.interpreter == nil {
		return agentwf.ParsedSummary{}, false
	}
	var schema map[string]any
	if json.Unmarshal([]byte(summaryIntentSchema), &schema) != nil {
		return agentwf.ParsedSummary{}, false
	}
	resp, err := i.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: summaryIntentSystemPrompt,
		UserMessage:  text,
		JSONSchema: &interfaces.JSONSchemaSpec{
			Name:   "summary_intent",
			Strict: true,
			Schema: schema,
		},
		MaxTokens: i.maxTokens,
	})
	if err != nil || len(resp.RawJSON) == 0 {
		return agentwf.ParsedSummary{}, false
	}
	var raw struct {
		Action   string `json:"action"`
		Target   string `json:"target"`
		NewValue string `json:"new_value"`
	}
	if json.Unmarshal(resp.RawJSON, &raw) != nil {
		return agentwf.ParsedSummary{}, false
	}
	switch strings.ToLower(raw.Action) {
	case "confirm":
		return agentwf.ParsedSummary{Confirm: true}, true
	case "cancel":
		return agentwf.ParsedSummary{Cancel: true}, true
	case "correct":
		return summaryCorrection(raw.Target, raw.NewValue)
	}
	return agentwf.ParsedSummary{}, false
}

func summaryCorrection(target, newValue string) (agentwf.ParsedSummary, bool) {
	switch strings.ToLower(target) {
	case "objective":
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetObjective, NewValue: newValue}, true
	case "budget", "renda":
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetBudget, NewValue: newValue}, true
	case "values", "categoria", "categorias", "distribuicao", "distribuição":
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetValues, NewValue: newValue}, true
	case "cards", "cartao", "cartão":
		nickname, dueDay, ok := extractCard(newValue)
		if ok {
			return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetCards, NewValue: fmt.Sprintf("%s %d", nickname, dueDay)}, true
		}
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetCards, NewValue: newValue}, true
	}
	return agentwf.ParsedSummary{}, false
}

func parseCorrection(text string) agentwf.ParsedSummary {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "orçamento"), strings.Contains(lower, "orcamento"), strings.Contains(lower, "renda"):
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetBudget, NewValue: extractMoneyText(text)}
	case strings.Contains(lower, "objetivo"), strings.Contains(lower, "meta"):
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetObjective, NewValue: extractObjectiveText(text)}
	case strings.Contains(lower, "valor"), strings.Contains(lower, "categoria"), strings.Contains(lower, "distribuição"), strings.Contains(lower, "distribuicao"):
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetValues, NewValue: extractMoneyText(text)}
	case strings.Contains(lower, "cartão"), strings.Contains(lower, "cartao"):
		nickname, dueDay, ok := extractCard(text)
		if ok {
			return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetCards, NewValue: fmt.Sprintf("%s %d", nickname, dueDay)}
		}
		return agentwf.ParsedSummary{Correct: true, Target: agentwf.CorrectionTargetCards, NewValue: text}
	default:
		return agentwf.ParsedSummary{Ambiguous: true}
	}
}

func extractMoneyText(text string) string {
	re := regexp.MustCompile(`[Rr]\$\s*[\d.]+,?\d*`)
	if match := re.FindString(text); match != "" {
		return match
	}
	re = regexp.MustCompile(`\d+[,.]?\d*`)
	if match := re.FindString(text); match != "" {
		return match
	}
	return text
}

func extractObjectiveText(text string) string {
	lower := strings.ToLower(text)
	markers := []string{"objetivo", "meta", "é", "eh", "="}
	for _, m := range markers {
		idx := strings.Index(lower, m)
		if idx >= 0 {
			start := idx + len(m)
			if start < len(text) {
				return strings.TrimSpace(text[start:])
			}
		}
	}
	return text
}

func (i *onboardingInterpreter) render(ctx context.Context, system, user string) string {
	return user
}

func parseMoney(text string) (int64, bool) {
	return agentwf.ParseMoneyCents(text)
}

func extractCard(text string) (string, int, bool) {
	trimmed := strings.TrimSpace(text)
	dayRe := regexp.MustCompile(`(\d{1,2})`)
	dayMatches := dayRe.FindStringSubmatch(trimmed)
	if len(dayMatches) < 2 {
		return "", 0, false
	}
	day, _ := strconv.Atoi(dayMatches[1])
	nickname := strings.TrimSpace(dayRe.ReplaceAllString(trimmed, ""))
	if nickname == "" {
		return "", 0, false
	}
	return nickname, day, true
}

func isDailyCommandText(text string) bool {
	lower := strings.ToLower(text)
	cues := []string{
		"gastei", "gasto", "comprei", "paguei", "recebi",
		"quanto", "qual meu", "meu saldo", "como estou", "resumo do mês",
		"mercado", "lanche", "pix", "conta", "transferi", "extrato", "depósito", "deposito",
	}
	for _, cue := range cues {
		if strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}

func isConfirmation(text string) bool {
	switch text {
	case "sim", "yes", "vamos", "começar", "bora", "ok", "certo", "quero", "acho que sim":
		return true
	default:
		return false
	}
}

func isNegation(text string) bool {
	switch text {
	case "não", "nao", "no", "nunca", "não uso", "nao uso", "não tenho", "nao tenho":
		return true
	default:
		return false
	}
}

func valuesCue(pending string) string {
	switch pending {
	case "fixed_cost":
		return "Quanto você gasta por mês com custos fixos? (ex: aluguel, contas) 💰"
	case "knowledge":
		return "Quanto você investe em conhecimento? (ex: cursos, livros) 🎓"
	case "pleasures":
		return "Quanto você reserva para prazeres? (ex: lazer, restaurantes) 🎉"
	case "goals":
		return "Quanto você destina para metas? (ex: viagem, carro) 🎯"
	case "financial_freedom":
		return "Quanto você guarda para liberdade financeira? 🏦"
	default:
		return "Quanto você quer destinar?"
	}
}

func summaryCue(state agentwf.SummaryState) string {
	var b strings.Builder
	b.WriteString("✅ Planejamento criado!\n\n")
	b.WriteString("🎯 Objetivo:\n")
	b.WriteString(state.Objective)
	b.WriteString("\n\n💰 Orçamento:\n")
	b.WriteString(formatBRL(state.IncomeCents))
	b.WriteString("\n\n📊 Distribuição\n")
	for _, slug := range []string{"fixed_cost", "knowledge", "pleasures", "goals", "financial_freedom"} {
		amount := state.Values[slug]
		b.WriteString("\n")
		b.WriteString(categoryEmoji(slug))
		b.WriteString(" ")
		b.WriteString(categoryLabel(slug))
		b.WriteString("\n")
		b.WriteString(formatBRL(amount))
		b.WriteString(" (")
		b.WriteString(formatPercent(amount, state.IncomeCents))
		b.WriteString("%)\n")
	}
	b.WriteString("\nEstá tudo certo? 😊")
	return b.String()
}

func categoryLabel(slug string) string {
	switch slug {
	case "fixed_cost":
		return "Custo Fixo"
	case "knowledge":
		return "Conhecimento"
	case "pleasures":
		return "Prazeres"
	case "goals":
		return "Metas"
	case "financial_freedom":
		return "Liberdade Financeira"
	default:
		return slug
	}
}

func categoryEmoji(slug string) string {
	switch slug {
	case "fixed_cost":
		return "💰"
	case "knowledge":
		return "🎓"
	case "pleasures":
		return "🎉"
	case "goals":
		return "🎯"
	case "financial_freedom":
		return "🏦"
	default:
		return "•"
	}
}

func retryCue(phase string) string {
	switch phase {
	case "welcome":
		return "Para começar, é só dizer *sim* 😊"
	case "objective":
		return "Não entendi seu objetivo. Pode me contar de novo com poucas palavras? 😊"
	case "budget":
		return "Não entendi o valor. Quanto você recebe por mês? Me diga só o número. 💰"
	case "cards":
		return "Pra cadastrar o cartão preciso do apelido e do dia de vencimento (1 a 31). Pode me passar? 💳"
	case "categories":
		return "As categorias ajudam a organizar seu dinheiro. Vamos seguir?"
	case "values":
		return "Não entendi o valor. Pode me mandar só o número em reais?"
	case "summary":
		return "Pode me dizer se está tudo certo ou o que você gostaria de ajustar?"
	default:
		return "Não entendi. Pode repetir? 😊"
	}
}

func dailyRedirectCue(phase string) string {
	_ = phase
	return "Antes de registrar movimentações, vamos terminar seu planejamento? É rapidinho 🚀"
}

const (
	summaryIntentSystemPrompt = `Você é o MeControla. O usuário está revisando o resumo do planejamento e foi perguntado "Está tudo certo?".
Classifique a mensagem:
- action="confirm" quando concorda que está tudo certo (ex: "sim", "está tudo certo", "tá ok", "perfeito", "pode confirmar", "fechado").
- action="correct" quando quer mudar um campo: preencha "target" (objective|budget|cards|values) e "new_value".
- action="cancel" quando quer cancelar ou recomeçar.
- action="none" quando não estiver claro.
Campos: objective (objetivo), budget (renda/orçamento), cards (cartão), values (distribuição/valores).`

	summaryIntentSchema = `{
  "type": "object",
  "properties": {
    "action": {"type": "string", "enum": ["confirm", "correct", "cancel", "none"]},
    "target": {"type": "string", "enum": ["objective", "budget", "cards", "values", ""]},
    "new_value": {"type": "string"}
  },
  "required": ["action", "target", "new_value"],
  "additionalProperties": false
}`
)

const (
	welcomeSystemPrompt       = "Você é o MeControla, um assistente financeiro acolhedor. Envie a mensagem de boas-vindas do onboarding e pergunte 'Vamos começar?'. Use tom leve, emojis oficiais."
	objectiveSystemPrompt     = "Você é o MeControla. Pergunte o objetivo financeiro do usuário de forma acolhedora, depois que ele já aceitou começar."
	budgetSystemPrompt        = "Você é o MeControla. Pergunte o valor disponível do orçamento mensal do usuário."
	cardsSystemPrompt         = "Você é o MeControla. Pergunte sobre cartões: apelido e dia de vencimento. Não peça fechamento, limite, banco nem bandeira."
	categoriesSystemPrompt    = "Você é o MeControla. Apresente as 5 categorias oficiais: 💰 Custo Fixo, 🎓 Conhecimento, 🎉 Prazeres, 🎯 Metas, 🏦 Liberdade Financeira. Pergunte se faz sentido."
	valuesSystemPrompt        = "Você é o MeControla. Pergunte o valor em reais para a categoria indicada."
	retrySystemPrompt         = "Você é o MeControla. Responda de forma gentil pedindo para o usuário repetir ou esclarecer."
	dailyRedirectSystemPrompt = "Você é o MeControla. O usuário mandou um comando diário no meio do onboarding. Peça gentilmente para concluir o setup primeiro."
	welcomeCue                = "Bem-vindo ao MeControla! 🎉 Vou te ajudar a criar seu primeiro planejamento financeiro em poucos minutos. Vamos começar? 🚀"
	objectiveCue              = "Qual é o seu principal objetivo financeiro agora? (ex: quitar dívidas, viajar, juntar reserva) 🎯"
	budgetCue                 = "Quanto você tem disponível por mês para montar esse planejamento? 💰"
	cardsFirstCue             = "Você usa cartão de crédito? Se sim, me diz o apelido e o dia de vencimento da fatura. Se não usa, é só dizer 'não uso'. 💳"
	cardsNextCue              = "Deseja adicionar outro cartão? Me diz o apelido e o dia de vencimento, ou diga 'não' para seguir. 💳"
	categoriesCue             = "No MeControla usamos 5 categorias:\n💰 Custo Fixo\n🎓 Conhecimento\n🎉 Prazeres\n🎯 Metas\n🏦 Liberdade Financeira\n\nFaz sentido?"
	summarySystemPrompt       = "Você é o MeControla. Apresente o resumo do planejamento do usuário de forma clara, com valor e percentual por categoria, e pergunte se está tudo certo."
	conclusionSystemPrompt    = "Você é o MeControla. Celebre a conclusão do onboarding e ensine como registrar movimentações no dia a dia com exemplos simples."
	conclusionCue             = "Seu planejamento está pronto! 🎉\n\nAgora é só usar no dia a dia:\n• *Mercado 120 pix* — registra uma despesa\n• *Como estou esse mês?* — veja seu resumo\n\nConta comigo sempre que precisar! 🚀"
)
