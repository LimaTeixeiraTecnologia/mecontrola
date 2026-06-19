package usecases

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
)

const (
	OnbPhaseWelcome      = "welcome"
	OnbPhaseMethodology1 = "methodology_1"
	OnbPhaseMethodology2 = "methodology_2"
	OnbPhaseMethodology3 = "methodology_3"
	OnbPhaseMethodology4 = "methodology_4"
	OnbPhaseMethodology5 = "methodology_5"
	OnbPhaseObjective    = "objective"
	OnbPhaseIncome       = "income"
	OnbPhaseCards        = "cards"
	OnbPhaseSplits       = "splits"
	OnbPhaseSummary      = "summary"
	OnbPhaseFirstTx      = "first_tx"
)

const (
	scriptWelcome = "👋 Oi! Eu sou o *MeControla*, seu parceiro pra organizar o dinheiro sem complicação.\n\nEm poucos minutos a gente deixa tudo no controle e você começa a realizar seus objetivos. Vamos começar? 🚀"

	scriptMethodology1 = "📊 Antes de tudo, deixa eu te mostrar como a gente organiza o dinheiro por aqui. É simples: tudo vive em *5 categorias* — nada de planilha complicada.\n\nVamos uma por uma:\n\n💰 *Custo Fixo* — contas que vêm todo mês, tipo aluguel, água, luz, telefone.\n\nFaz sentido? 😊"
	scriptMethodology2 = "🎓 *Conhecimento* — livros, cursos, estudos: tudo que te faz evoluir.\n\nFaz sentido? 😊"
	scriptMethodology3 = "🎉 *Prazeres* — lazer, jantares, diversão. A vida também é pra aproveitar!\n\nFaz sentido? 😊"
	scriptMethodology4 = "🎯 *Metas* — objetivos de curto e médio prazo, tipo uma viagem, comprar algo especial ou quitar uma dívida.\n\nFaz sentido? 😊"
	scriptMethodology5 = "🏦 *Liberdade Financeira* — investimentos e reserva de emergência, pra você dormir tranquilo.\n\nFaz sentido? 😊"

	scriptObjective    = "🎯 Show! Agora me conta: qual é o seu *objetivo principal*? O que você quer conquistar organizando o dinheiro? (ex: quitar dívidas, fazer uma viagem, comprar um carro, criar uma reserva)"
	scriptIncome       = "💰 Pra montar seu plano, qual é o seu *orçamento mensal* aproximado? (o quanto você tem por mês)"
	scriptCards        = "💳 Você usa *cartão de crédito*? Se sim, me diz o *apelido* e o *dia de vencimento* da fatura. Se não usar, é só dizer. 😊"
	scriptTransition   = "🚀 Seu plano tá pronto e estruturado! Falta só *um passo* pra você dominar o app.\n\n📝 Bora fazer seu *primeiro lançamento*? Me manda do jeito que você fala no dia a dia, tipo \"gastei 35 no mercado\" ou \"recebi 2500 de salário\"."
	scriptCardQuestion = "💳 Quer cadastrar um cartão? Me diz o apelido e o dia de vencimento — ou diga que não usa. 😊"
)

var onboardingDefaultSplit = []struct {
	slug string
	bp   int
}{
	{"expense.custo_fixo", 4000},
	{"expense.conhecimento", 1000},
	{"expense.prazeres", 1500},
	{"expense.metas", 2000},
	{"expense.liberdade_financeira", 1500},
}

func defaultSplitAmounts(budgetCents int64) []int64 {
	amounts := make([]int64, len(onboardingDefaultSplit))
	var assigned int64
	for i, e := range onboardingDefaultSplit {
		if i == len(onboardingDefaultSplit)-1 {
			amounts[i] = budgetCents - assigned
			continue
		}
		amounts[i] = budgetCents * int64(e.bp) / 10000
		assigned += amounts[i]
	}
	return amounts
}

func buildSplitsQuestion(budgetCents int64) string {
	amounts := defaultSplitAmounts(budgetCents)
	parts := make([]string, len(onboardingDefaultSplit))
	for i, e := range onboardingDefaultSplit {
		parts[i] = fmt.Sprintf("%s %d%% (R$ %s)", onboardingSlugEmoji(e.slug), e.bp/100, formatBRLCents(amounts[i]))
	}
	return "📊 Agora vamos distribuir seu orçamento entre as 5 categorias.\n\nMinha sugestão:\n" +
		strings.Join(parts, "\n") +
		"\n\nResponde *sim* pra usar essa distribuição, ou me manda os valores *em reais* do seu jeito (a soma precisa fechar R$ " + formatBRLCents(budgetCents) + ")."
}

func defaultSplitToolCall(budgetCents int64) interfaces.ToolCall {
	amounts := defaultSplitAmounts(budgetCents)
	allocations := make([]any, len(onboardingDefaultSplit))
	for i, e := range onboardingDefaultSplit {
		allocations[i] = map[string]any{"root_slug": e.slug, "amount_cents": amounts[i]}
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
	b.WriteString("A pessoa está na etapa atual descrita abaixo. Se a mensagem dela contiver o dado pedido, CHAME a ferramenta disponível com os valores corretos. Se for uma dúvida ou conversa, responda em TEXTO curto e gentil (sem ferramenta) e não invente dados.\n\n")
	switch phase {
	case OnbPhaseObjective:
		b.WriteString("Etapa: objetivo principal. Chame save_onboarding_objective com o objetivo informado.")
	case OnbPhaseIncome:
		b.WriteString("Etapa: orçamento mensal. Converta o valor para centavos (R$ 5.000 = 500000, 5 mil = 500000) e chame save_onboarding_income.")
	case OnbPhaseCards:
		b.WriteString("Etapa: cartão de crédito. Use o apelido EXATO que a pessoa escreveu (não abrevie). Só chame save_onboarding_card se a pessoa informar TANTO o apelido QUANTO o dia de vencimento (1 a 31). Se faltar o dia de vencimento, NÃO chame a ferramenta: pergunte em texto o dia de vencimento da fatura. Nunca invente o dia.")
	case OnbPhaseSplits:
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

func onboardingPhaseTool(phase string) string {
	switch phase {
	case OnbPhaseObjective:
		return ToolSaveOnboardingObjective
	case OnbPhaseIncome:
		return ToolSaveOnboardingIncome
	case OnbPhaseCards:
		return ToolSaveOnboardingCard
	case OnbPhaseSplits:
		return ToolSaveOnboardingBudgetSplits
	case OnbPhaseFirstTx:
		return recordTransactionToolName
	default:
		return ""
	}
}

const recordTransactionToolName = "record_transaction"

func onboardingSummary(snapshot OnboardingSnapshot) string {
	var b strings.Builder
	b.WriteString("📋 *Seu plano:*\n")
	if strings.TrimSpace(snapshot.Objective) != "" {
		fmt.Fprintf(&b, "🎯 Objetivo: %s\n", snapshot.Objective)
	}
	fmt.Fprintf(&b, "💰 Orçamento: R$ %s\n", formatBRLCents(snapshot.IncomeCents))
	if len(snapshot.Cards) > 0 {
		cards := make([]string, 0, len(snapshot.Cards))
		for _, c := range snapshot.Cards {
			cards = append(cards, c.Name+" (vence dia "+strconv.Itoa(c.DueDay)+")")
		}
		fmt.Fprintf(&b, "💳 Cartões: %s\n", strings.Join(cards, ", "))
	}
	if len(snapshot.Splits) > 0 {
		parts := make([]string, 0, len(snapshot.Splits))
		for _, s := range snapshot.Splits {
			parts = append(parts, onboardingSlugEmoji(s.Slug)+strconv.Itoa(s.Percent)+"%")
		}
		fmt.Fprintf(&b, "📊 Distribuição: %s\n", strings.Join(parts, " · "))
	}
	b.WriteString("\nTá tudo certo? 😊")
	return b.String()
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
	if cents < 0 {
		cents = -cents
	}
	reais := cents / 100
	s := strconv.FormatInt(reais, 10)
	if len(s) > 3 {
		rem := len(s) % 3
		var out []byte
		if rem > 0 {
			out = append(out, s[:rem]...)
			out = append(out, '.')
		}
		for i := rem; i < len(s); i += 3 {
			out = append(out, s[i:i+3]...)
			if i+3 < len(s) {
				out = append(out, '.')
			}
		}
		s = string(out)
	}
	return fmt.Sprintf("%s,%02d", s, cents%100)
}
