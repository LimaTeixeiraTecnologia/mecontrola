package usecases

import (
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/onboardingv2draft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
)

const (
	OnbPhaseObjective     = "objective"
	OnbPhaseBudget        = "budget"
	OnbPhaseCards         = "cards"
	OnbPhaseFinancialPlan = "financial_plan"
	OnbPhaseFirstTx       = "first_tx"
)

const (
	scriptWelcome = "👋 Oi! Eu sou o *MeControla*, seu parceiro financeiro.\n\n" +
		"📊 Aqui no MeControla todo dinheiro é organizado em apenas *5 categorias*:\n\n" +
		"💰 *Custo Fixo*\nContas que acontecem todos os meses.\n\n" +
		"🎓 *Conhecimento*\nCursos, livros e tudo que faz você evoluir.\n\n" +
		"🎉 *Prazeres*\nLazer, diversão e experiências.\n\n" +
		"🎯 *Metas*\nObjetivos específicos que você deseja realizar.\n\n" +
		"🏦 *Liberdade Financeira*\nReserva financeira e investimentos.\n\n" +
		"Pronto 😊\n\n" +
		"Agora vamos montar seu plano.\n\n" +
		"🔵 *Etapa 1/4 — Objetivo*\n" +
		"Qual é o seu objetivo principal? (ex: quitar dívidas, fazer uma viagem, criar uma reserva)"

	scriptBudget = "🔵 *Etapa 2/4 — Orçamento*\n\n" +
		"Pra montar seu plano, qual é o seu *orçamento mensal*? (quanto você tem por mês)"

	scriptCards = "🔵 *Etapa 3/4 — Cartões*\n\n" +
		"💳 Você usa *cartão de crédito*?\n\n" +
		"Se usar, me responda assim:\n\n" +
		"Nubank 13 / Inter 5 / Itaú 10\n\n" +
		"(apelido + dia de fechamento)\n\n" +
		"Se não usar, basta responder: *Não uso*"

	scriptCardQuestion = "💳 Qual é o apelido e o dia de fechamento? (ex: Nubank 13)"

	scriptFirstTx = "🚀 Agora é só começar! Me manda seu *primeiro lançamento* do jeito que você fala:\n\n" +
		"\"gastei 35 no mercado\" ou \"recebi 2500 de salário\""
)

const (
	onboardingWelcomeSystemPrompt = "Você é o MeControla, parceiro financeiro no WhatsApp. Esta é a PRIMEIRA mensagem para um usuário recém-ativado. " +
		"Escreva UMA mensagem curta, calorosa e acolhedora em PT-BR que: (1) se apresente como parceiro financeiro; " +
		"(2) diga em uma frase que organiza gastos e receitas direto no WhatsApp; " +
		"(3) termine perguntando qual é o objetivo financeiro principal da pessoa (ex: quitar dívidas, fazer uma viagem, criar uma reserva). " +
		"Formatação WhatsApp: para negrito use UM asterisco (*assim*), nunca dois. Não use listas longas, não peça valores ainda e não invente dados."
	onboardingWelcomeCue = "Inicie o onboarding e me pergunte meu objetivo financeiro principal."
)

func buildAutoSplitPreview(splits []onboardingv2draft.SplitEntry) string {
	var b strings.Builder
	b.WriteString("📊 Aqui está uma sugestão de distribuição para o seu orçamento:\n\n")
	for _, s := range splits {
		fmt.Fprintf(&b, "%s *%s*: R$ %s\n", onboardingSlugEmoji(s.RootSlug), onboardingSlugName(s.RootSlug), formatBRLCents(s.AmountCents))
	}
	return strings.TrimRight(b.String(), "\n")
}

func buildFinancialPlanMessage(snapshot OnboardingSnapshot, splits []onboardingv2draft.SplitEntry) string {
	var b strings.Builder
	b.WriteString("🔵 *Etapa 4/4 — Plano Financeiro*\n\n")
	if snapshot.Objective != "" {
		fmt.Fprintf(&b, "🎯 *Objetivo*: %s\n", snapshot.Objective)
	}
	if snapshot.IncomeCents > 0 {
		fmt.Fprintf(&b, "💰 *Orçamento*: R$ %s/mês\n", formatBRLCents(snapshot.IncomeCents))
	}
	if len(snapshot.Cards) > 0 {
		cardParts := make([]string, 0, len(snapshot.Cards))
		for _, c := range snapshot.Cards {
			cardParts = append(cardParts, fmt.Sprintf("%s (fecha dia %d)", c.Name, c.ClosingDay))
		}
		fmt.Fprintf(&b, "💳 *Cartões*: %s\n", strings.Join(cardParts, ", "))
	}
	b.WriteString("\n📋 *Distribuição Final*:\n\n")
	for _, s := range splits {
		fmt.Fprintf(&b, "%s *%s*: R$ %s\n", onboardingSlugEmoji(s.RootSlug), onboardingSlugName(s.RootSlug), formatBRLCents(s.AmountCents))
	}
	b.WriteString("\nResponde *sim* pra confirmar, ou me diz como quer distribuir.")
	return b.String()
}

func buildAutoSplitToolCall(splits []onboardingv2draft.SplitEntry) interfaces.ToolCall {
	allocations := make([]any, len(splits))
	for i, s := range splits {
		allocations[i] = map[string]any{"root_slug": s.RootSlug, "amount_cents": s.AmountCents}
	}
	return interfaces.ToolCall{
		FunctionName:  ToolSaveOnboardingBudgetSplits,
		ArgumentsJSON: map[string]any{"allocations": allocations},
	}
}

func sanitizeWhatsAppText(text string) string {
	out := strings.TrimSpace(text)
	for strings.Contains(out, "**") {
		out = strings.ReplaceAll(out, "**", "*")
	}
	return out
}

func onboardingTextHasDigit(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

var onboardingNegations = []string{
	"nao", "não", "n", "no", "agora nao", "agora não", "negativo", "nope",
	"so esse", "só esse", "somente esse", "sem cartao", "sem cartão", "nenhum",
	"nao uso", "não uso", "nao tenho", "não tenho", "ja chega", "já chega", "pode seguir",
}

var onboardingInterrogatives = []string{
	"o que", "que ", "qual", "quais", "por que", "porque", "porquê", "pq",
	"como", "quando", "quanto", "quanta", "onde", "quem", "sera", "será",
}

func looksLikeOnboardingQuestion(normalized string) bool {
	if strings.Contains(normalized, "?") {
		return true
	}
	for _, q := range onboardingInterrogatives {
		if strings.HasPrefix(normalized, q) {
			return true
		}
	}
	return false
}

func shouldAdvanceScriptedPhase(text string) bool {
	normalized := normalizeOnboardingText(text)
	if normalized == "" {
		return false
	}
	if looksLikeOnboardingQuestion(normalized) {
		return false
	}
	if matchesCues(normalized, onboardingNegations) {
		return false
	}
	return true
}

func matchesOnboardingNegation(text string) bool {
	normalized := normalizeOnboardingText(text)
	if normalized == "" {
		return false
	}
	return matchesCues(normalized, onboardingNegations)
}

func normalizeOnboardingText(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func isOnboardingSeparator(r rune) bool {
	if r >= 0x80 {
		return false
	}
	if r >= 'a' && r <= 'z' {
		return false
	}
	if r >= '0' && r <= '9' {
		return false
	}
	return true
}

func matchesCues(normalized string, cues []string) bool {
	words := strings.FieldsFunc(normalized, isOnboardingSeparator)
	wordSet := make(map[string]struct{}, len(words))
	for _, w := range words {
		wordSet[w] = struct{}{}
	}
	for _, cue := range cues {
		if normalized == cue {
			return true
		}
		if strings.Contains(cue, " ") {
			if strings.Contains(normalized, cue) {
				return true
			}
			continue
		}
		if _, ok := wordSet[cue]; ok {
			return true
		}
	}
	return false
}

func onboardingToolByName(name string) (interfaces.ToolSpec, bool) {
	for _, tool := range OnboardingToolCatalog() {
		if tool.Name == name {
			return tool, true
		}
	}
	return interfaces.ToolSpec{}, false
}

func onboardingDataPhasePrompt(phase string, snapshot OnboardingSnapshot) string {
	var b strings.Builder
	b.WriteString("Você é o MeControla, assistente de onboarding financeiro via WhatsApp. Tom acolhedor e curto.\n")
	b.WriteString("Formatação WhatsApp: para negrito use UM asterisco (*assim*), NUNCA dois (**assim**). Não use Markdown.\n")
	b.WriteString("A pessoa está na etapa atual descrita abaixo. Se a mensagem dela contiver o dado pedido, CHAME a ferramenta disponível com os valores corretos. Se for uma dúvida ou conversa, responda em TEXTO curto e gentil (sem ferramenta) e não invente dados.\n")
	fmt.Fprintf(&b, "Quando for uma dúvida ou conversa (sem chamar ferramenta), SEMPRE retome a etapa atual: responda breve e gentil e termine repetindo o cabeçalho exato \"%s\" seguido de refazer a pergunta da etapa.\n\n", onboardingPhaseHeader(phase))
	switch phase {
	case OnbPhaseObjective:
		b.WriteString("Etapa: objetivo principal. Chame save_onboarding_objective com o objetivo informado.")
	case OnbPhaseBudget:
		b.WriteString("Etapa: orçamento mensal. Converta o valor para centavos (R$ 5.000 = 500000, 5 mil = 500000) e chame save_onboarding_income.")
	case OnbPhaseCards:
		b.WriteString("Etapa: cartões de crédito. O usuário pode enviar vários cartões em uma única mensagem (ex: 'Nubank 13 / Inter 5' ou um por linha). Para CADA cartão que tiver apelido E dia de fechamento, chame save_onboarding_card separadamente usando o apelido EXATO que a pessoa escreveu. Se faltar o dia de fechamento de algum cartão, pergunte em texto — nunca invente o dia.")
	case OnbPhaseFinancialPlan:
		b.WriteString("Etapa: distribuição do orçamento EM REAIS por categoria. Converta cada valor para centavos e mapeie os nomes para root_slug: Custo Fixo->expense.custo_fixo, Conhecimento->expense.conhecimento, Prazeres->expense.prazeres, Metas->expense.metas, Liberdade Financeira->expense.liberdade_financeira. Envie as 5 categorias (0 para as não citadas) e chame save_onboarding_budget_splits.")
	case OnbPhaseFirstTx:
		b.WriteString("Etapa: primeiro lançamento. Extraia direção (income/outcome), valor em centavos e estabelecimento, e chame record_transaction.")
	}
	if strings.TrimSpace(snapshot.Objective) != "" {
		fmt.Fprintf(&b, "\n\nContexto: objetivo=%s", snapshot.Objective)
	}
	if snapshot.IncomeCents > 0 {
		fmt.Fprintf(&b, " orçamento_centavos=%d", snapshot.IncomeCents)
	}
	return b.String()
}

const (
	onbHeaderObjective     = "🔵 Etapa 1/4 — Objetivo"
	onbHeaderBudget        = "🔵 Etapa 2/4 — Orçamento"
	onbHeaderCards         = "🔵 Etapa 3/4 — Cartões"
	onbHeaderFinancialPlan = "🔵 Etapa 4/4 — Plano Financeiro"
)

func onboardingPhaseHeader(phase string) string {
	switch phase {
	case OnbPhaseObjective:
		return onbHeaderObjective
	case OnbPhaseBudget:
		return onbHeaderBudget
	case OnbPhaseCards:
		return onbHeaderCards
	case OnbPhaseFinancialPlan:
		return onbHeaderFinancialPlan
	case OnbPhaseFirstTx:
		return onbHeaderFinancialPlan
	default:
		return onbHeaderObjective
	}
}

func onboardingPhaseTool(phase string) string {
	switch phase {
	case OnbPhaseObjective:
		return ToolSaveOnboardingObjective
	case OnbPhaseBudget:
		return ToolSaveOnboardingIncome
	case OnbPhaseCards:
		return ToolSaveOnboardingCard
	case OnbPhaseFinancialPlan:
		return ToolSaveOnboardingBudgetSplits
	case OnbPhaseFirstTx:
		return recordTransactionToolName
	default:
		return ""
	}
}

const recordTransactionToolName = "record_transaction"

func onboardingSlugName(slug string) string {
	switch slug {
	case "expense.custo_fixo":
		return "Custo Fixo"
	case "expense.conhecimento":
		return "Conhecimento"
	case "expense.prazeres":
		return "Prazeres"
	case "expense.metas":
		return "Metas"
	case "expense.liberdade_financeira":
		return "Liberdade Financeira"
	default:
		return slug
	}
}

func onboardingSlugEmoji(slug string) string {
	switch slug {
	case "expense.custo_fixo":
		return "💰"
	case "expense.conhecimento":
		return "🎓"
	case "expense.prazeres":
		return "🎉"
	case "expense.metas":
		return "🎯"
	case "expense.liberdade_financeira":
		return "🏦"
	default:
		return ""
	}
}

func formatBRLCents(cents int64) string {
	return money.FromCents(cents).Amount()
}

func joinReplies(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return strings.Join(out, "\n\n")
}

func outcomeForAdvance(advance bool) string {
	if advance {
		return "advance"
	}
	return "stay"
}
