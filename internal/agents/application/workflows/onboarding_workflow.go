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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const OnboardingWorkflowID = "onboarding-workflow"

const (
	stepGoalID         = "step-goal"
	stepIncomeID       = "step-income"
	stepCardsID        = "step-cards"
	stepMethodologyID  = "step-methodology"
	stepDistributionID = "step-distribution"
	stepSummaryID      = "step-summary"
	stepConclusionID   = "step-conclusion"
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

var _defaultDistributionBP = map[string]int{
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
	PhaseMonthlyIncome
	PhaseCards
	PhaseMethodology
	PhaseDistribution
	PhaseSummary
	PhaseConclusion
)

var errInvalidOnboardingPhase = errors.New("onboarding: invalid phase")

func (p OnboardingPhase) String() string {
	switch p {
	case PhaseWelcome:
		return "welcome"
	case PhaseGoal:
		return "goal"
	case PhaseMonthlyIncome:
		return "monthly_income"
	case PhaseCards:
		return "cards"
	case PhaseMethodology:
		return "methodology"
	case PhaseDistribution:
		return "distribution"
	case PhaseSummary:
		return "summary"
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
	case "monthly_income":
		return PhaseMonthlyIncome, nil
	case "cards":
		return PhaseCards, nil
	case "methodology":
		return PhaseMethodology, nil
	case "distribution":
		return PhaseDistribution, nil
	case "summary":
		return PhaseSummary, nil
	case "conclusion":
		return PhaseConclusion, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidOnboardingPhase, s)
	}
}

type allocationInputKind int

const (
	allocationInputConfirm allocationInputKind = iota + 1
	allocationInputPercent
	allocationInputReais
)

var errInvalidAllocationInput = errors.New("distribution: tipo de entrada invalido")

func (k allocationInputKind) String() string {
	switch k {
	case allocationInputConfirm:
		return "confirm"
	case allocationInputPercent:
		return "percent"
	case allocationInputReais:
		return "reais"
	default:
		return "unknown"
	}
}

func (k allocationInputKind) IsValid() bool {
	return k >= allocationInputConfirm && k <= allocationInputReais
}

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
	Phase        OnboardingPhase `json:"phase"`
	UserID       string          `json:"userID"`
	PeerID       string          `json:"peerID"`
	Goal         string          `json:"goal"`
	IncomeCents  int64           `json:"incomeCents"`
	CardsDone    bool            `json:"cardsDone"`
	Allocations  map[string]int  `json:"allocations"`
	CardNickname string          `json:"cardNickname"`
	CardDueDay   int             `json:"cardDueDay"`
	Recurrence   bool            `json:"recurrence"`
	ResumeText   string          `json:"resumeText"`
	FinalMessage string          `json:"finalMessage"`
}

func DecideGoal(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", errors.New("goal: texto nao pode ser vazio")
	}
	return trimmed, nil
}

func DecideIncomeCents(amountBRL float64) (int64, error) {
	if amountBRL <= 0 {
		return 0, errors.New("income: valor deve ser maior que zero")
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

func DecideAllocationsBP(kind allocationInputKind, valuesBySlug map[string]float64, incomeCents int64) (map[string]int, error) {
	switch kind {
	case allocationInputConfirm:
		return maps.Clone(_defaultDistributionBP), nil
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
		if incomeCents <= 0 {
			return nil, errors.New("não consegui usar sua renda para converter os valores")
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
		if sum != incomeCents {
			return nil, fmt.Errorf("a soma dos valores (%s) precisa ser igual à sua renda (%s)", formatBRL(sum), formatBRL(incomeCents))
		}
		bpSlice := centsToBasisPoints(cents, incomeCents)
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

func formatBRL(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	subunit := cents % 100
	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, reais, subunit)
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
		fmt.Fprintf(&b, "%s: %s (%d%%)\n", categoryLabels[slug], formatBRL(it.PlannedCents), it.BasisPoints/100)
	}
	return b.String()
}

type goalExtract struct {
	Goal string `json:"goal"`
}

type incomeExtract struct {
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

var goalSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"goal": map[string]any{"type": "string"},
	},
	"required":             []any{"goal"},
	"additionalProperties": false,
}

var incomeSchema = map[string]any{
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

const _welcomeGoalPrompt = "🎉 Bem-vindo ao MeControla! 🎉\n\n" +
	"Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰\n\n" +
	"Vamos começar? Qual é o seu principal objetivo financeiro para este mês?\n" +
	"(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva)"

const _goalReprompt = "Não consegui identificar seu objetivo. Qual é o seu principal objetivo financeiro para este mês? Por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva."

const _incomePrompt = "Perfeito! Agora me conta: qual é a sua renda mensal líquida em reais? (por exemplo: R$ 3.500,00)"

const _incomeReprompt = "Não consegui identificar o valor. Qual é a sua renda mensal líquida em reais? Por exemplo: R$ 3.500,00."

const _cardsReprompt = "Para adicionar o cartão, me diga o apelido, o banco emissor e o dia de vencimento da fatura (um número entre 1 e 31). Por exemplo: \"Nubank, vencimento dia 10\". Se preferir não adicionar agora, responda \"não\"."

const _summaryReprompt = "Sem problema! Quando estiver tudo certo, responda \"sim\" para eu ativar o seu orçamento."

const _conclusionRecurrencePrompt = "🎉 Seu orçamento foi ativado com sucesso! Deseja que ele se repita automaticamente pelos próximos 12 meses? Responda \"sim\" ou \"não\"."

const _allocationInputSystemPrompt = "Você classifica a resposta do usuário sobre a distribuição do orçamento em 5 categorias: custo_fixo, conhecimento, prazeres, metas, liberdade_financeira. " +
	"Defina action='confirm' quando o usuário aceitar a sugestão sem informar novos valores (ex.: sim, aceito, pode confirmar, ok). " +
	"Defina action='percent' quando ele informar percentuais (números pequenos que somam cerca de 100). " +
	"Defina action='reais' quando ele informar valores em reais (R$). " +
	"Preencha cada categoria com o número informado pelo usuário; use 0 quando a categoria não for citada."

func cardsPrompt(existing int) string {
	if existing > 0 {
		return fmt.Sprintf("Você já tem %d cartão(ões) cadastrado(s). Deseja adicionar outro cartão de crédito agora? Se sim, informe o apelido, o banco emissor e o dia de vencimento da fatura (entre 1 e 31). Se não, responda \"não\".", existing)
	}
	return "Você deseja adicionar um cartão de crédito agora? Se sim, informe o apelido, o banco emissor e o dia de vencimento da fatura (entre 1 e 31). Se não, responda \"não\"."
}

func methodologyPrompt(items []interfaces.AllocationCents) string {
	var b strings.Builder
	b.WriteString("Agora vamos distribuir seu orçamento. O MeControla organiza tudo em 5 categorias. Esta é a sugestão com base na sua renda:\n\n")
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
	fmt.Fprintf(&b, "💵 Renda mensal: %s\n", formatBRL(state.IncomeCents))
	if state.CardNickname != "" {
		fmt.Fprintf(&b, "💳 Cartão: %s (vencimento dia %d)\n", state.CardNickname, state.CardDueDay)
	}
	b.WriteString("\nDistribuição:\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString("\nPosso ativar seu orçamento com esses dados? Responda \"sim\" para confirmar ou \"não\" para revisar.")
	return b.String()
}

func conclusionFinalMessage(goal string) string {
	return fmt.Sprintf(
		"Tudo pronto! 🚀 Seu objetivo \"%s\" está registrado.\n\n"+
			"Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: \"gastei R$ 50 no mercado\" ou \"recebi R$ 200 de freela\") que eu registro tudo pra você. Vamos juntos! 💪",
		goal,
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

func BuildGoalStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseGoal
			return suspendStep(state, _welcomeGoalPrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: "Extraia o objetivo financeiro principal do texto do usuario. Retorne como string concisa."},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "goal_extract", Strict: true, Schema: goalSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.goal: parse: %w", err))
		}
		var extract goalExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.goal: unmarshal: %w", err))
		}
		goal, err := DecideGoal(extract.Goal)
		if err != nil {
			return suspendStep(state, _goalReprompt), nil
		}
		state.Goal = goal
		return completeStep(state), nil
	}
}

func BuildIncomeStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseMonthlyIncome
			return suspendStep(state, _incomePrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: "Extraia o valor da renda mensal em reais (BRL) do texto do usuario. Retorne como numero decimal."},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "income_extract", Strict: true, Schema: incomeSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.income: parse: %w", err))
		}
		var extract incomeExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.income: unmarshal: %w", err))
		}
		cents, err := DecideIncomeCents(extract.AmountBRL)
		if err != nil {
			return suspendStep(state, _incomeReprompt), nil
		}
		state.IncomeCents = cents
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
				{Role: "system", Content: "Extraia do texto do usuario se ele quer adicionar um cartao (wantsCard), o apelido (nickname), o banco emissor (bank) e o dia de vencimento (dueDay, inteiro 1-31). Se nao quiser cartao, retorne wantsCard=false, nickname vazio, bank vazio e dueDay=0."},
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
		if extract.WantsCard {
			if err := DecideCardEntry(extract.Nickname, extract.Bank, extract.DueDay); err != nil {
				return suspendStep(state, _cardsReprompt), nil
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
			state.CardNickname = extract.Nickname
			state.CardDueDay = extract.DueDay
		}
		state.CardsDone = true
		return completeStep(state), nil
	}
}

func BuildMethodologyStep(a agent.Agent, budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		preview, previewErr := budgets.SuggestAllocation(ctx, state.IncomeCents, allocationBPList(_defaultDistributionBP))
		if previewErr != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.methodology: suggest_allocation: %w", previewErr))
		}
		if state.ResumeText == "" {
			state.Phase = PhaseMethodology
			return suspendStep(state, methodologyPrompt(preview)), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: _allocationInputSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "allocation_input", Strict: true, Schema: allocationInputSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.methodology: parse: %w", err))
		}
		var input allocationInputExtract
		if err := json.Unmarshal(extracted.RawJSON, &input); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.methodology: unmarshal: %w", err))
		}
		kind, kindErr := ParseAllocationInputKind(input.Action)
		if kindErr != nil {
			return suspendStep(state, methodologyReprompt("não entendi sua resposta.", preview)), nil
		}
		values := map[string]float64{
			"expense.custo_fixo":           input.CustoFixo,
			"expense.conhecimento":         input.Conhecimento,
			"expense.prazeres":             input.Prazeres,
			"expense.metas":                input.Metas,
			"expense.liberdade_financeira": input.LiberdadeFinanceira,
		}
		bp, decErr := DecideAllocationsBP(kind, values, state.IncomeCents)
		if decErr != nil {
			return suspendStep(state, methodologyReprompt(decErr.Error(), preview)), nil
		}
		state.Allocations = bp
		return completeStep(state), nil
	}
}

func BuildDistributionStep(budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseDistribution
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.distribution: parse_user_id: %w", err))
		}
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		competence := time.Now().In(loc).Format("2006-01")
		allocations := make([]interfaces.AllocationDraft, 0, len(state.Allocations))
		for _, slug := range canonicalSlugs {
			allocations = append(allocations, interfaces.AllocationDraft{
				RootSlug:    slug,
				BasisPoints: state.Allocations[slug],
			})
		}
		draft := interfaces.DraftBudget{
			UserID:      userUUID,
			Competence:  competence,
			TotalCents:  state.IncomeCents,
			Allocations: allocations,
		}
		summary, sumErr := budgets.GetMonthlySummary(ctx, userUUID, competence)
		switch {
		case errors.Is(sumErr, interfaces.ErrBudgetNotFound):
			if _, err := budgets.CreateBudget(ctx, draft); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.distribution: create_budget: %w", err))
			}
		case sumErr != nil:
			return failStep(state, fmt.Errorf("agents.onboarding.distribution: get_monthly_summary: %w", sumErr))
		case summary.State == "active":
			return completeStep(state), nil
		default:
			if err := budgets.DeleteDraftBudget(ctx, userUUID, competence); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.distribution: delete_draft_budget: %w", err))
			}
			if _, err := budgets.CreateBudget(ctx, draft); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.distribution: create_budget: %w", err))
			}
		}
		return completeStep(state), nil
	}
}

func BuildSummaryStep(a agent.Agent, budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseSummary
			preview, err := budgets.SuggestAllocation(ctx, state.IncomeCents, allocationBPList(state.Allocations))
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.summary: suggest_allocation: %w", err))
			}
			return suspendStep(state, summaryPrompt(state, preview)), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: "O usuario esta confirmando se deseja ativar o orcamento com os dados apresentados. Extraia se confirmou (true) ou nao (false)."},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "summary_confirm", Strict: true, Schema: recurrenceSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.summary: parse: %w", err))
		}
		var extract yesNoExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.summary: unmarshal: %w", err))
		}
		if !extract.Confirmed {
			return suspendStep(state, _summaryReprompt), nil
		}
		return completeStep(state), nil
	}
}

func BuildConclusionStep(a agent.Agent, budgets interfaces.BudgetPlanner, workingMem memory.WorkingMemory) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseConclusion
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.conclusion: parse_user_id: %w", err))
			}
			loc, _ := time.LoadLocation("America/Sao_Paulo")
			competence := time.Now().In(loc).Format("2006-01")
			if err := budgets.ActivateBudget(ctx, userUUID, competence); err != nil && !errors.Is(err, interfaces.ErrBudgetAlreadyActive) {
				return failStep(state, fmt.Errorf("agents.onboarding.conclusion: activate_budget: %w", err))
			}
			return suspendStep(state, _conclusionRecurrencePrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: "O usuario esta respondendo se deseja recorrencia automatica do orcamento por 12 meses. Extraia se confirmou (true) ou nao (false)."},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "recurrence_extract", Strict: true, Schema: recurrenceSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: parse: %w", err))
		}
		var extract yesNoExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: unmarshal: %w", err))
		}
		if extract.Confirmed {
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.conclusion: parse_user_id_recurrence: %w", err))
			}
			loc, _ := time.LoadLocation("America/Sao_Paulo")
			competence := time.Now().In(loc).Format("2006-01")
			if err := budgets.CreateRecurrence(ctx, userUUID, competence, 12); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.conclusion: create_recurrence: %w", err))
			}
			state.Recurrence = true
		}
		if err := workingMem.Upsert(ctx, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: upsert_wm: %w", err))
		}
		state.FinalMessage = conclusionFinalMessage(state.Goal)
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
			workflow.NewStepFunc(stepGoalID, wrap(BuildGoalStep(a))),
			workflow.NewStepFunc(stepIncomeID, wrap(BuildIncomeStep(a))),
			workflow.NewStepFunc(stepCardsID, wrap(BuildCardsStep(a, cards))),
			workflow.NewStepFunc(stepMethodologyID, wrap(BuildMethodologyStep(a, budgets))),
			workflow.NewStepFunc(stepDistributionID, BuildDistributionStep(budgets)),
			workflow.NewStepFunc(stepSummaryID, wrap(BuildSummaryStep(a, budgets))),
			workflow.NewStepFunc(stepConclusionID, wrap(BuildConclusionStep(a, budgets, workingMem))),
		),
		Durable:     true,
		MaxAttempts: 3,
	}
}
