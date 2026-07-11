package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const OnboardingWorkflowID = "onboarding-workflow"

const (
	stepWelcomeID       = "step-welcome"
	stepGoalID          = "step-goal"
	stepMonthlyBudgetID = "step-monthly-budget"
	stepCardsID         = "step-cards"
	stepBudgetReviewID  = "step-budget-review"
	stepActivationID    = "step-activation"
	stepRecurrenceID    = "step-recurrence"
	stepConclusionID    = "step-conclusion"
)

const (
	OnboardingStaleAfter  = 7 * 24 * time.Hour
	OnboardingReaperBatch = 100
)

var canonicalSlugs = []string{
	"expense.custo_fixo",
	"expense.conhecimento",
	"expense.prazeres",
	"expense.metas",
	"expense.liberdade_financeira",
}

var categoryLabels = map[string]string{
	"expense.custo_fixo":           "💰 Custo Fixo",
	"expense.conhecimento":         "🎓 Conhecimento",
	"expense.prazeres":             "🎉 Prazeres",
	"expense.metas":                "🎯 Metas",
	"expense.liberdade_financeira": "🏦 Liberdade Financeira",
}

var defaultDistributionBP = map[string]int{
	"expense.custo_fixo":           4000,
	"expense.conhecimento":         1000,
	"expense.prazeres":             1000,
	"expense.metas":                1000,
	"expense.liberdade_financeira": 3000,
}

type OnboardingPhase int

const (
	PhaseWelcome OnboardingPhase = iota + 1
	PhaseGoal
	PhaseMonthlyBudget
	PhaseBudgetReview
	PhaseActivation
	PhaseRecurrence
	PhaseCards
	PhaseConclusion
)

var errInvalidOnboardingPhase = errors.New("onboarding: invalid phase")

func (p OnboardingPhase) String() string {
	switch p {
	case PhaseWelcome:
		return "welcome"
	case PhaseGoal:
		return "goal"
	case PhaseMonthlyBudget:
		return "monthly_budget"
	case PhaseBudgetReview:
		return "budget_review"
	case PhaseActivation:
		return "activation"
	case PhaseRecurrence:
		return "recurrence"
	case PhaseCards:
		return "cards"
	case PhaseConclusion:
		return "conclusion"
	default:
		return "unknown"
	}
}

func (p OnboardingPhase) IsValid() bool {
	return p >= PhaseWelcome && p <= PhaseConclusion
}

func ParseOnboardingPhase(s string) (OnboardingPhase, error) {
	switch s {
	case "welcome":
		return PhaseWelcome, nil
	case "goal":
		return PhaseGoal, nil
	case "monthly_budget":
		return PhaseMonthlyBudget, nil
	case "budget_review":
		return PhaseBudgetReview, nil
	case "activation":
		return PhaseActivation, nil
	case "recurrence":
		return PhaseRecurrence, nil
	case "cards":
		return PhaseCards, nil
	case "conclusion":
		return PhaseConclusion, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidOnboardingPhase, s)
	}
}

type reviewAwaitKind int

const (
	reviewAwaitDistribution reviewAwaitKind = iota + 1
	reviewAwaitConfirm
)

var errInvalidReviewAwaitKind = errors.New("onboarding: invalid review await kind")

func (k reviewAwaitKind) String() string {
	switch k {
	case reviewAwaitDistribution:
		return "distribution"
	case reviewAwaitConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (k reviewAwaitKind) IsValid() bool {
	return k >= reviewAwaitDistribution && k <= reviewAwaitConfirm
}

type allocationInputKind int

const (
	allocationInputConfirm allocationInputKind = iota + 1
	allocationInputPercent
	allocationInputReais
)

var errInvalidAllocationInput = errors.New("distribution: tipo de entrada invalido")

var errAllocationConfirmWithValues = errors.New("recebi valores personalizados; me diga se quer aplicá-los em reais (R$) ou em porcentagem (%)")

func ParseAllocationInputKind(s string) (allocationInputKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "confirm":
		return allocationInputConfirm, nil
	case "percent":
		return allocationInputPercent, nil
	case "reais":
		return allocationInputReais, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidAllocationInput, s)
	}
}

type OnboardingState struct {
	Phase              OnboardingPhase `json:"phase"`
	UserID             string          `json:"userID"`
	PeerID             string          `json:"peerID"`
	Goal               string          `json:"goal"`
	GoalValueCents     int64           `json:"goalValueCents"`
	GoalValueAsked     bool            `json:"goalValueAsked"`
	MonthlyBudgetCents int64           `json:"monthlyBudgetCents"`
	ReviewAwait        reviewAwaitKind `json:"reviewAwait"`
	CardsDone          bool            `json:"cardsDone"`
	Allocations        map[string]int  `json:"allocations"`
	Recurrence         bool            `json:"recurrence"`
	ResumeText         string          `json:"resumeText"`
	FinalMessage       string          `json:"finalMessage"`
}

func DecideGoal(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", errors.New("goal: texto nao pode ser vazio")
	}
	return trimmed, nil
}

func DecideGoalValueCents(hasAmount bool, amountBRL float64) (int64, bool) {
	if !hasAmount || amountBRL <= 0 {
		return 0, false
	}
	return int64(math.Round(amountBRL * 100)), true
}

func DecideMonthlyBudgetCents(amountBRL float64) (int64, error) {
	if amountBRL <= 0 {
		return 0, errors.New("monthly_budget: valor deve ser maior que zero")
	}
	return int64(math.Round(amountBRL * 100)), nil
}

func DecideDistribution(allocsBP map[string]int) error {
	var errs []error
	total := 0
	for _, slug := range canonicalSlugs {
		v, ok := allocsBP[slug]
		if !ok {
			errs = append(errs, fmt.Errorf("distribution: categoria ausente: %s", slug))
			continue
		}
		if v < 0 {
			errs = append(errs, fmt.Errorf("distribution: %s nao pode ser negativo", slug))
		}
		total += v
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if total != 10000 {
		return fmt.Errorf("distribution: soma dos basis points deve ser 10000, recebido %d", total)
	}
	return nil
}

func sumAllocationValues(valuesBySlug map[string]float64) float64 {
	var sum float64
	for _, slug := range canonicalSlugs {
		sum += valuesBySlug[slug]
	}
	return sum
}

func DecideAllocationKind(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) allocationInputKind {
	sum := sumAllocationValues(valuesBySlug)
	if sum <= 0 {
		return allocationInputConfirm
	}
	monthlyBudgetBRL := float64(monthlyBudgetCents) / 100
	if monthlyBudgetBRL > 0 && math.Abs(sum-monthlyBudgetBRL) <= 0.5 {
		return allocationInputReais
	}
	if math.Abs(sum-100) <= 0.5 {
		return allocationInputPercent
	}
	return kind
}

func DecideAllocationsBP(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) (map[string]int, error) {
	switch kind {
	case allocationInputConfirm:
		if sumAllocationValues(valuesBySlug) > 0 {
			return nil, errAllocationConfirmWithValues
		}
		return maps.Clone(defaultDistributionBP), nil
	case allocationInputPercent:
		bp := make(map[string]int, len(canonicalSlugs))
		totalPct := 0
		for _, slug := range canonicalSlugs {
			pct := int(math.Round(valuesBySlug[slug]))
			if pct < 0 {
				return nil, fmt.Errorf("o percentual de %s não pode ser negativo", categoryLabels[slug])
			}
			bp[slug] = pct * 100
			totalPct += pct
		}
		if totalPct != 100 {
			return nil, fmt.Errorf("a soma dos percentuais precisa ser 100%%, mas você informou %d%%", totalPct)
		}
		return bp, nil
	case allocationInputReais:
		if monthlyBudgetCents <= 0 {
			return nil, errors.New("não consegui usar seu orçamento mensal para converter os valores")
		}
		cents := make([]int64, len(canonicalSlugs))
		var sum int64
		for i, slug := range canonicalSlugs {
			v := valuesBySlug[slug]
			if v < 0 {
				return nil, fmt.Errorf("o valor de %s não pode ser negativo", categoryLabels[slug])
			}
			cents[i] = int64(math.Round(v * 100))
			sum += cents[i]
		}
		if sum != monthlyBudgetCents {
			return nil, fmt.Errorf("a soma dos valores (%s) precisa ser igual ao seu orçamento mensal (%s)", money.FromCents(sum).BRL(), money.FromCents(monthlyBudgetCents).BRL())
		}
		bpSlice := centsToBasisPoints(cents, monthlyBudgetCents)
		bp := make(map[string]int, len(canonicalSlugs))
		for i, slug := range canonicalSlugs {
			bp[slug] = bpSlice[i]
		}
		if err := DecideDistribution(bp); err != nil {
			return nil, err
		}
		return bp, nil
	default:
		return nil, errInvalidAllocationInput
	}
}

func centsToBasisPoints(cents []int64, totalCents int64) []int {
	bp := make([]int, len(cents))
	remainders := make([]int64, len(cents))
	assigned := 0
	for i, c := range cents {
		raw := c * 10000
		bp[i] = int(raw / totalCents)
		remainders[i] = raw % totalCents
		assigned += bp[i]
	}
	for leftover := 10000 - assigned; leftover > 0; leftover-- {
		best := -1
		var bestRem int64 = -1
		for i := range cents {
			if remainders[i] > bestRem {
				bestRem = remainders[i]
				best = i
			}
		}
		if best < 0 {
			break
		}
		bp[best]++
		remainders[best] = -1
	}
	return bp
}

func DecideCardEntry(nickname, bank string, dueDay int) error {
	var errs []error
	if strings.TrimSpace(nickname) == "" {
		errs = append(errs, errors.New("card_entry: nickname nao pode ser vazio"))
	}
	if strings.TrimSpace(bank) == "" {
		errs = append(errs, errors.New("card_entry: bank nao pode ser vazio"))
	}
	if dueDay < 1 || dueDay > 31 {
		errs = append(errs, fmt.Errorf("card_entry: dueDay deve estar entre 1 e 31, recebido %d", dueDay))
	}
	return errors.Join(errs...)
}

func allocationBPList(bpBySlug map[string]int) []interfaces.AllocationBP {
	out := make([]interfaces.AllocationBP, 0, len(canonicalSlugs))
	for _, slug := range canonicalSlugs {
		out = append(out, interfaces.AllocationBP{RootSlug: slug, BasisPoints: bpBySlug[slug]})
	}
	return out
}

func allocationCentsBySlug(items []interfaces.AllocationCents) map[string]interfaces.AllocationCents {
	out := make(map[string]interfaces.AllocationCents, len(items))
	for _, it := range items {
		out[it.RootSlug] = it
	}
	return out
}

func renderAllocationLines(items []interfaces.AllocationCents) string {
	bySlug := allocationCentsBySlug(items)
	var b strings.Builder
	for _, slug := range canonicalSlugs {
		it := bySlug[slug]
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], money.FromCents(it.PlannedCents).BRL(), it.BasisPoints/100)
	}
	return b.String()
}

type goalWithValueExtract struct {
	Goal      string  `json:"goal"`
	HasAmount bool    `json:"hasAmount"`
	AmountBRL float64 `json:"amountBRL"`
}

type goalValueExtract struct {
	HasAmount bool    `json:"hasAmount"`
	AmountBRL float64 `json:"amountBRL"`
}

type monthlyBudgetExtract struct {
	AmountBRL float64 `json:"amountBRL"`
}

type cardExtract struct {
	WantsCard bool   `json:"wantsCard"`
	Nickname  string `json:"nickname"`
	Bank      string `json:"bank"`
	DueDay    int    `json:"dueDay"`
}

type allocationInputExtract struct {
	Action              string  `json:"action"`
	CustoFixo           float64 `json:"custo_fixo"`
	Conhecimento        float64 `json:"conhecimento"`
	Prazeres            float64 `json:"prazeres"`
	Metas               float64 `json:"metas"`
	LiberdadeFinanceira float64 `json:"liberdade_financeira"`
}

type yesNoExtract struct {
	Confirmed bool `json:"confirmed"`
}

var goalWithValueSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"goal":      map[string]any{"type": "string"},
		"hasAmount": map[string]any{"type": "boolean"},
		"amountBRL": map[string]any{"type": "number"},
	},
	"required":             []any{"goal", "hasAmount", "amountBRL"},
	"additionalProperties": false,
}

var goalValueSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"hasAmount": map[string]any{"type": "boolean"},
		"amountBRL": map[string]any{"type": "number"},
	},
	"required":             []any{"hasAmount", "amountBRL"},
	"additionalProperties": false,
}

var monthlyBudgetSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"amountBRL": map[string]any{"type": "number"},
	},
	"required":             []any{"amountBRL"},
	"additionalProperties": false,
}

var cardSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"wantsCard": map[string]any{"type": "boolean"},
		"nickname":  map[string]any{"type": "string"},
		"bank":      map[string]any{"type": "string"},
		"dueDay":    map[string]any{"type": "integer"},
	},
	"required":             []any{"wantsCard", "nickname", "bank", "dueDay"},
	"additionalProperties": false,
}

var allocationInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action":               map[string]any{"type": "string", "enum": []any{"confirm", "percent", "reais"}},
		"custo_fixo":           map[string]any{"type": "number"},
		"conhecimento":         map[string]any{"type": "number"},
		"prazeres":             map[string]any{"type": "number"},
		"metas":                map[string]any{"type": "number"},
		"liberdade_financeira": map[string]any{"type": "number"},
	},
	"required":             []any{"action", "custo_fixo", "conhecimento", "prazeres", "metas", "liberdade_financeira"},
	"additionalProperties": false,
}

var recurrenceSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"confirmed": map[string]any{"type": "boolean"},
	},
	"required":             []any{"confirmed"},
	"additionalProperties": false,
}

const welcomePrompt = "🎉 Bem-vindo ao MeControla! 🎉\n\n" +
	"Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰"

const welcomeGoalPrompt = "Vamos começar? Qual é o seu principal objetivo financeiro para este mês?\n" +
	"(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo \"comprar uma casa, meta de R$ 400.000,00\")"

const goalReprompt = "Não consegui identificar seu objetivo. Qual é o seu principal objetivo financeiro para este mês? Por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva. Se souber, pode me contar também o valor da meta — mas isso é totalmente opcional."

const goalValueReprompt = "Legal! E você já tem uma ideia de quanto (em R$) representa essa meta? Pode responder com um número, por exemplo \"R$ 400.000,00\" ou \"400 mil\" — se preferir não informar agora, é só responder \"não\" que a gente segue em frente."

const monthlyBudgetPrompt = "Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui. Tudo vive em apenas 5 categorias: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira.\n\n" +
	"Qual é o seu orçamento mensal? (por exemplo: R$ 3.500,00)"

const monthlyBudgetReprompt = "Não consegui identificar o valor. Qual é o seu orçamento mensal? Por exemplo: R$ 3.500,00."

const cardsReprompt = "Para adicionar o cartão, me diga o apelido, o banco emissor e o dia de vencimento da fatura (um número entre 1 e 31). Por exemplo: \"Nubank, vencimento dia 10\". Se preferir não adicionar agora, responda \"não\"."

const conclusionRecurrencePrompt = "Quer que eu repita esse orçamento automaticamente pelos próximos 12 meses, sem precisar configurar de novo? Responda \"sim\" ou \"não\"."

const allocationInputSystemPrompt = "Você classifica a resposta do usuário sobre a distribuição do orçamento em 5 categorias: custo_fixo, conhecimento, prazeres, metas, liberdade_financeira. " +
	"Defina action='confirm' SOMENTE quando o usuário aceitar a sugestão sem informar nenhum valor novo (ex.: sim, aceito, pode confirmar, ok); nunca use 'confirm' quando o texto contiver números para as categorias. " +
	"Defina action='reais' quando o usuário informar valores em reais — valores acompanhados de R$/reais ou números grandes cuja soma se aproxima do orçamento mensal (ex.: 2500, 500, 2000). " +
	"Defina action='percent' quando ele informar percentuais — números pequenos, acompanhados de % ou cuja soma se aproxima de 100. " +
	"Em caso de dúvida entre 'reais' e 'percent', escolha 'reais' se a soma dos números se aproximar do orçamento mensal e 'percent' se a soma se aproximar de 100; jamais coaja valores em reais para percentuais ou vice-versa. " +
	"Preencha cada categoria com o número informado pelo usuário; use 0 quando a categoria não for citada."

const summaryConfirmSystemPrompt = "O usuario esta confirmando se deseja ativar o orcamento com os dados apresentados. Extraia se confirmou (true) ou nao (false)."

const goalWithValueSystemPrompt = "Extraia o objetivo financeiro principal do texto do usuário (campo goal, string concisa) e, se houver, o valor em reais associado a essa meta. " +
	"Defina hasAmount=true somente quando o texto mencionar explicitamente um valor monetário para a meta; caso contrário hasAmount=false e amountBRL=0. " +
	"Converta o valor mencionado para um número em reais (amountBRL), sempre em ponto decimal, nunca com símbolo de moeda ou separador de milhar. Exemplos de conversão: " +
	"\"R$ 400.000,00\" -> amountBRL=400000; " +
	"\"400000\" -> amountBRL=400000; " +
	"\"10 mil reais\" -> amountBRL=10000; " +
	"\"400 mil\" -> amountBRL=400000; " +
	"\"1,5 milhão\" -> amountBRL=1500000. " +
	"Se o usuário não mencionar nenhum valor, ou disser que não sabe, ou recusar informar, defina hasAmount=false e amountBRL=0 — nunca invente um valor que não esteja no texto."

const monthlyBudgetSystemPrompt = "Extraia o valor do orcamento mensal em reais (BRL) do texto do usuario. Retorne como numero decimal."

const cardsSystemPrompt = "Extraia do texto do usuario se ele quer adicionar um cartao (wantsCard), o apelido (nickname), o banco emissor (bank) e o dia de vencimento (dueDay, inteiro 1-31). Se nao quiser cartao, retorne wantsCard=false, nickname vazio, bank vazio e dueDay=0."

const recurrenceSystemPrompt = "O usuario esta respondendo se deseja repetir o orcamento automaticamente pelos proximos 12 meses. Extraia se confirmou (true) ou nao (false)."

const goalValueSystemPrompt = "Extraia, se houver, o valor em reais que o usuário informou para a meta financeira dele. " +
	"Defina hasAmount=true somente quando o texto mencionar explicitamente um valor monetário; caso contrário hasAmount=false e amountBRL=0. " +
	"Converta o valor mencionado para um número em reais (amountBRL), sempre em ponto decimal, nunca com símbolo de moeda ou separador de milhar. Exemplos de conversão: " +
	"\"R$ 400.000,00\" -> amountBRL=400000; " +
	"\"400000\" -> amountBRL=400000; " +
	"\"10 mil reais\" -> amountBRL=10000; " +
	"\"400 mil\" -> amountBRL=400000; " +
	"\"1,5 milhão\" -> amountBRL=1500000. " +
	"Se o usuário recusar (ex.: \"não\", \"não sei\", \"prefiro não dizer\") ou não mencionar nenhum valor, defina hasAmount=false e amountBRL=0 — nunca invente um valor que não esteja no texto."

func cardsPrompt(existing int) string {
	if existing > 0 {
		return fmt.Sprintf("Você já tem %d cartão(ões) cadastrado(s). Deseja adicionar outro cartão de crédito agora? Se sim, informe o apelido, o banco emissor e o dia de vencimento da fatura (entre 1 e 31). Se não, responda \"não\".", existing)
	}
	return "Você deseja adicionar um cartão de crédito agora? Se sim, informe o apelido, o banco emissor e o dia de vencimento da fatura (entre 1 e 31). Se não, responda \"não\"."
}

func methodologyPrompt(items []interfaces.AllocationCents) string {
	var b strings.Builder
	b.WriteString("Agora vamos distribuir seu orçamento. O MeControla organiza tudo em 5 categorias. Esta é a sugestão com base no seu orçamento mensal:\n\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString("\nAceita esta sugestão? Responda \"sim\" para confirmar, ou envie novos valores para cada categoria — pode ser em reais (R$) ou em porcentagem (%).")
	return b.String()
}

func methodologyReprompt(reason string, items []interfaces.AllocationCents) string {
	return "Ops, não consegui aplicar essa distribuição: " + reason + "\n\n" + methodologyPrompt(items)
}

func summaryPrompt(state OnboardingState, items []interfaces.AllocationCents) string {
	var b strings.Builder
	b.WriteString("Vamos revisar tudo antes de ativar seu orçamento:\n\n")
	fmt.Fprintf(&b, "🎯 Objetivo: %s\n", state.Goal)
	fmt.Fprintf(&b, "💵 Orçamento mensal: %s\n", money.FromCents(state.MonthlyBudgetCents).BRL())
	b.WriteString("\nDistribuição:\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString("\nPosso ativar seu orçamento com esses dados? Responda \"sim\" para confirmar ou \"não\" para revisar a distribuição.")
	return b.String()
}

func conclusionFinalMessage(goal string, valueCents int64) string {
	objetivo := fmt.Sprintf("Seu objetivo \"%s\"", goal)
	if valueCents > 0 {
		objetivo = fmt.Sprintf("Seu objetivo \"%s\" (meta de %s)", goal, money.FromCents(valueCents).BRL())
	}
	return fmt.Sprintf(
		"Tudo pronto! 🚀 %s está registrado.\n\n"+
			"Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: \"gastei R$ 50 no mercado\" ou \"recebi R$ 200 de freela\") que eu registro tudo pra você. Vamos juntos! 💪",
		objetivo,
	)
}

func suspendStep(state OnboardingState, prompt string) workflow.StepOutput[OnboardingState] {
	return workflow.StepOutput[OnboardingState]{
		State:   state,
		Status:  workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: prompt},
	}
}

func completeStep(state OnboardingState) workflow.StepOutput[OnboardingState] {
	return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}
}

func failStep(state OnboardingState, err error) (workflow.StepOutput[OnboardingState], error) {
	return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed}, err
}

func BuildWelcomeStep() func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(_ context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseWelcome
			return suspendStep(state, welcomePrompt), nil
		}
		state.ResumeText = ""
		return completeStep(state), nil
	}
}

func BuildGoalStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseGoal
			return suspendStep(state, welcomeGoalPrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""

		if state.Goal == "" {
			extracted, err := a.Execute(ctx, agent.Request{
				Messages: []llm.Message{
					{Role: "system", Content: goalWithValueSystemPrompt},
					{Role: "user", Content: resumeText},
				},
				Schema: &llm.Schema{Name: "goal_with_value_extract", Strict: true, Schema: goalWithValueSchema},
			})
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.goal: parse: %w", err))
			}
			var extract goalWithValueExtract
			if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.goal: unmarshal: %w", err))
			}
			goal, err := DecideGoal(extract.Goal)
			if err != nil {
				return suspendStep(state, goalReprompt), nil
			}
			state.Goal = goal
			if cents, ok := DecideGoalValueCents(extract.HasAmount, extract.AmountBRL); ok {
				state.GoalValueCents = cents
			}
			if state.GoalValueCents == 0 && !state.GoalValueAsked {
				state.GoalValueAsked = true
				return suspendStep(state, goalValueReprompt), nil
			}
			return completeStep(state), nil
		}

		if !state.GoalValueAsked {
			state.GoalValueAsked = true
			return suspendStep(state, goalValueReprompt), nil
		}

		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: goalValueSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "goal_value_extract", Strict: true, Schema: goalValueSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.goal_value: parse: %w", err))
		}
		var extract goalValueExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.goal_value: unmarshal: %w", err))
		}
		if cents, ok := DecideGoalValueCents(extract.HasAmount, extract.AmountBRL); ok {
			state.GoalValueCents = cents
		}
		return completeStep(state), nil
	}
}

func BuildMonthlyBudgetStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseMonthlyBudget
			return suspendStep(state, monthlyBudgetPrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: monthlyBudgetSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "monthly_budget_extract", Strict: true, Schema: monthlyBudgetSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.monthly_budget: parse: %w", err))
		}
		var extract monthlyBudgetExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.monthly_budget: unmarshal: %w", err))
		}
		cents, err := DecideMonthlyBudgetCents(extract.AmountBRL)
		if err != nil {
			return suspendStep(state, monthlyBudgetReprompt), nil
		}
		state.MonthlyBudgetCents = cents
		return completeStep(state), nil
	}
}

func BuildCardsStep(a agent.Agent, cards interfaces.CardManager) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseCards
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.cards: parse_user_id: %w", err))
			}
			existingCards, err := cards.ListCards(ctx, userUUID)
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.cards: list_cards: %w", err))
			}
			return suspendStep(state, cardsPrompt(len(existingCards))), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: cardsSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "card_extract", Strict: true, Schema: cardSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: parse: %w", err))
		}
		var extract cardExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: unmarshal: %w", err))
		}
		if !extract.WantsCard {
			state.CardsDone = true
			return completeStep(state), nil
		}
		if err := DecideCardEntry(extract.Nickname, extract.Bank, extract.DueDay); err != nil {
			return suspendStep(state, cardsReprompt), nil
		}
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: parse_user_id_create: %w", err))
		}
		if _, err := cards.CreateCard(ctx, interfaces.NewCard{
			UserID:   userUUID,
			Nickname: extract.Nickname,
			Bank:     extract.Bank,
			DueDay:   extract.DueDay,
		}); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: create_card: %w", err))
		}
		existingCards, err := cards.ListCards(ctx, userUUID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: list_cards_after_create: %w", err))
		}
		return suspendStep(state, cardsPrompt(len(existingCards))), nil
	}
}

func competenceLocation(loc *time.Location, err error) *time.Location {
	if err != nil || loc == nil {
		return time.UTC
	}
	return loc
}

func applyDraftBudget(ctx context.Context, budgets interfaces.BudgetPlanner, state OnboardingState) error {
	userUUID, err := uuid.Parse(state.UserID)
	if err != nil {
		return fmt.Errorf("parse_user_id: %w", err)
	}
	loc := competenceLocation(time.LoadLocation("America/Sao_Paulo"))
	competence := time.Now().In(loc).Format("2006-01")
	allocations := make([]interfaces.AllocationDraft, 0, len(canonicalSlugs))
	for _, slug := range canonicalSlugs {
		allocations = append(allocations, interfaces.AllocationDraft{
			RootSlug:    slug,
			BasisPoints: state.Allocations[slug],
		})
	}
	draft := interfaces.DraftBudget{
		UserID:      userUUID,
		Competence:  competence,
		TotalCents:  state.MonthlyBudgetCents,
		Allocations: allocations,
	}
	summary, sumErr := budgets.GetMonthlySummary(ctx, userUUID, competence)
	switch {
	case errors.Is(sumErr, interfaces.ErrBudgetNotFound):
		if _, err := budgets.CreateBudget(ctx, draft); err != nil {
			return fmt.Errorf("create_budget: %w", err)
		}
	case sumErr != nil:
		return fmt.Errorf("get_monthly_summary: %w", sumErr)
	case summary.State == "active":
		return nil
	default:
		if err := budgets.DeleteDraftBudget(ctx, userUUID, competence); err != nil {
			return fmt.Errorf("delete_draft_budget: %w", err)
		}
		if _, err := budgets.CreateBudget(ctx, draft); err != nil {
			return fmt.Errorf("create_budget: %w", err)
		}
	}
	return nil
}

func BuildBudgetReviewStep(a agent.Agent, budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseBudgetReview

		if state.ResumeText == "" {
			preview, previewErr := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(defaultDistributionBP))
			if previewErr != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation: %w", previewErr))
			}
			state.ReviewAwait = reviewAwaitDistribution
			return suspendStep(state, methodologyPrompt(preview)), nil
		}

		switch state.ReviewAwait {
		case reviewAwaitDistribution:
			return handleReviewAwaitDistribution(ctx, a, budgets, state)
		case reviewAwaitConfirm:
			return handleReviewAwaitConfirm(ctx, a, budgets, state)
		default:
			return failStep(state, fmt.Errorf("agents.onboarding.budget_review: %w", errInvalidReviewAwaitKind))
		}
	}
}

func handleReviewAwaitDistribution(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	resumeText := state.ResumeText
	state.ResumeText = ""
	preview, previewErr := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(defaultDistributionBP))
	if previewErr != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation: %w", previewErr))
	}
	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: allocationInputSystemPrompt},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "allocation_input", Strict: true, Schema: allocationInputSchema},
	})
	if err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: parse_allocation: %w", err))
	}
	var input allocationInputExtract
	if err := json.Unmarshal(extracted.RawJSON, &input); err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: unmarshal_allocation: %w", err))
	}
	kind, kindErr := ParseAllocationInputKind(input.Action)
	if kindErr != nil {
		state.ReviewAwait = reviewAwaitDistribution
		return suspendStep(state, methodologyReprompt("não entendi sua resposta.", preview)), nil
	}
	values := map[string]float64{
		"expense.custo_fixo":           input.CustoFixo,
		"expense.conhecimento":         input.Conhecimento,
		"expense.prazeres":             input.Prazeres,
		"expense.metas":                input.Metas,
		"expense.liberdade_financeira": input.LiberdadeFinanceira,
	}
	kind = DecideAllocationKind(kind, values, state.MonthlyBudgetCents)
	bp, decErr := DecideAllocationsBP(kind, values, state.MonthlyBudgetCents)
	if decErr != nil {
		state.ReviewAwait = reviewAwaitDistribution
		return suspendStep(state, methodologyReprompt(decErr.Error(), preview)), nil
	}
	state.Allocations = bp
	if err := applyDraftBudget(ctx, budgets, state); err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: apply_draft_budget: %w", err))
	}
	summaryPreview, err := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(state.Allocations))
	if err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation_current: %w", err))
	}
	state.ReviewAwait = reviewAwaitConfirm
	return suspendStep(state, summaryPrompt(state, summaryPreview)), nil
}

func handleReviewAwaitConfirm(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	resumeText := state.ResumeText
	state.ResumeText = ""
	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: summaryConfirmSystemPrompt},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "summary_confirm", Strict: true, Schema: recurrenceSchema},
	})
	if err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: parse_confirm: %w", err))
	}
	var extract yesNoExtract
	if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: unmarshal_confirm: %w", err))
	}
	if extract.Confirmed {
		state.ReviewAwait = 0
		return completeStep(state), nil
	}
	preview, previewErr := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(defaultDistributionBP))
	if previewErr != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation: %w", previewErr))
	}
	state.ReviewAwait = reviewAwaitDistribution
	return suspendStep(state, methodologyPrompt(preview)), nil
}

func BuildActivationStep(budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseActivation
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.activation: parse_user_id: %w", err))
		}
		loc := competenceLocation(time.LoadLocation("America/Sao_Paulo"))
		competence := time.Now().In(loc).Format("2006-01")
		if err := budgets.ActivateBudget(ctx, userUUID, competence); err != nil && !errors.Is(err, interfaces.ErrBudgetAlreadyActive) {
			return failStep(state, fmt.Errorf("agents.onboarding.activation: activate_budget: %w", err))
		}
		return completeStep(state), nil
	}
}

func BuildRecurrenceStep(a agent.Agent, budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseRecurrence
			return suspendStep(state, conclusionRecurrencePrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: recurrenceSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "recurrence_confirm", Strict: true, Schema: recurrenceSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: parse: %w", err))
		}
		var extract yesNoExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: unmarshal: %w", err))
		}
		if !extract.Confirmed {
			return completeStep(state), nil
		}
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: parse_user_id: %w", err))
		}
		loc := competenceLocation(time.LoadLocation("America/Sao_Paulo"))
		competence := time.Now().In(loc).Format("2006-01")
		if err := budgets.CreateRecurrence(ctx, userUUID, competence, 12); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: create_recurrence: %w", err))
		}
		state.Recurrence = true
		return completeStep(state), nil
	}
}

func BuildConclusionStep(workingMem memory.WorkingMemory) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseConclusion
		if err := workingMem.Upsert(ctx, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: upsert_wm: %w", err))
		}
		metadata := map[string]any{"objetivo_financeiro": state.Goal}
		if state.GoalValueCents > 0 {
			metadata["objetivo_financeiro_valor_centavos"] = state.GoalValueCents
		}
		if err := workingMem.UpsertMetadata(ctx, state.UserID, metadata); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: upsert_metadata: %w", err))
		}
		state.FinalMessage = conclusionFinalMessage(state.Goal, state.GoalValueCents)
		return completeStep(state), nil
	}
}

func appendOnboardingMsg(ctx context.Context, threads memory.ThreadGateway, messages memory.MessageStore, state OnboardingState, role memory.MessageRole, content string) {
	if state.PeerID == "" || content == "" {
		return
	}
	thread, err := threads.GetOrCreate(ctx, state.UserID, state.PeerID)
	if err != nil {
		return
	}
	_ = messages.Append(ctx, thread.ID, memory.Message{
		ID:         uuid.New(),
		ResourceID: state.UserID,
		Role:       role,
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	})
}

func wrapStepWithMessages(
	fn func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error),
	threads memory.ThreadGateway,
	messages memory.MessageStore,
) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		inbound := state.ResumeText
		if inbound != "" {
			appendOnboardingMsg(ctx, threads, messages, state, memory.RoleUser, inbound)
		}
		out, err := fn(ctx, state)
		if err == nil && out.Status == workflow.StepStatusSuspended && out.Suspend != nil && out.Suspend.Prompt != "" {
			appendOnboardingMsg(ctx, threads, messages, out.State, memory.RoleAssistant, out.Suspend.Prompt)
		}
		return out, err
	}
}

func BuildOnboardingWorkflow(
	a agent.Agent,
	cards interfaces.CardManager,
	budgets interfaces.BudgetPlanner,
	workingMem memory.WorkingMemory,
	threads memory.ThreadGateway,
	messages memory.MessageStore,
) workflow.Definition[OnboardingState] {
	wrap := func(fn func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error)) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		return wrapStepWithMessages(fn, threads, messages)
	}
	return workflow.Definition[OnboardingState]{
		ID: OnboardingWorkflowID,
		Root: workflow.Sequence("root",
			workflow.NewStepFunc(stepWelcomeID, wrap(BuildWelcomeStep())),
			workflow.NewStepFunc(stepGoalID, wrap(BuildGoalStep(a))),
			workflow.NewStepFunc(stepMonthlyBudgetID, wrap(BuildMonthlyBudgetStep(a))),
			workflow.NewStepFunc(stepBudgetReviewID, wrap(BuildBudgetReviewStep(a, budgets))),
			workflow.NewStepFunc(stepActivationID, wrap(BuildActivationStep(budgets))),
			workflow.NewStepFunc(stepRecurrenceID, wrap(BuildRecurrenceStep(a, budgets))),
			workflow.NewStepFunc(stepCardsID, wrap(BuildCardsStep(a, cards))),
			workflow.NewStepFunc(stepConclusionID, wrap(BuildConclusionStep(workingMem))),
		),
		Durable:     true,
		MaxAttempts: 3,
	}
}

func BuildOnboardingReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, OnboardingWorkflowID, OnboardingStaleAfter, OnboardingReaperBatch, o11y)
}
