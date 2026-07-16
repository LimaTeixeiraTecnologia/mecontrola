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
	reviewAwaitPersonalize
)

var errInvalidReviewAwaitKind = errors.New("onboarding: invalid review await kind")

func (k reviewAwaitKind) String() string {
	switch k {
	case reviewAwaitDistribution:
		return "distribution"
	case reviewAwaitConfirm:
		return "confirm"
	case reviewAwaitPersonalize:
		return "personalize"
	default:
		return "unknown"
	}
}

func (k reviewAwaitKind) IsValid() bool {
	return k >= reviewAwaitDistribution && k <= reviewAwaitPersonalize
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

type distributionIntentKind int

const (
	distributionIntentAccept distributionIntentKind = iota + 1
	distributionIntentPersonalize
	distributionIntentValues
)

var errInvalidDistributionIntent = errors.New("distribution: tipo de intencao invalido")

func (k distributionIntentKind) String() string {
	switch k {
	case distributionIntentAccept:
		return "accept"
	case distributionIntentPersonalize:
		return "personalize"
	case distributionIntentValues:
		return "values"
	default:
		return "unknown"
	}
}

func (k distributionIntentKind) IsValid() bool {
	return k >= distributionIntentAccept && k <= distributionIntentValues
}

func ParseDistributionIntentKind(s string) (distributionIntentKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "accept":
		return distributionIntentAccept, nil
	case "personalize":
		return distributionIntentPersonalize, nil
	case "values":
		return distributionIntentValues, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidDistributionIntent, s)
	}
}

type distributionBalanceKind int

const (
	distributionBalanced distributionBalanceKind = iota + 1
	distributionOver
	distributionUnder
)

var errInvalidDistributionBalance = errors.New("distribution: tipo de saldo invalido")

func (k distributionBalanceKind) String() string {
	switch k {
	case distributionBalanced:
		return "balanced"
	case distributionOver:
		return "over"
	case distributionUnder:
		return "under"
	default:
		return "unknown"
	}
}

func (k distributionBalanceKind) IsValid() bool {
	return k >= distributionBalanced && k <= distributionUnder
}

func ParseDistributionBalanceKind(s string) (distributionBalanceKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "balanced":
		return distributionBalanced, nil
	case "over":
		return distributionOver, nil
	case "under":
		return distributionUnder, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidDistributionBalance, s)
	}
}

const (
	distributionPercentToleranceAbs = 0.5
	distributionReaisToleranceCents = 5
)

type DistributionBalance struct {
	Status   distributionBalanceKind
	Unit     allocationInputKind
	Target   float64
	Sum      float64
	DeltaAbs float64
}

func DecideDistributionBalance(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) DistributionBalance {
	sum := sumAllocationValues(valuesBySlug)
	if kind == allocationInputReais {
		target := float64(monthlyBudgetCents) / 100
		deltaCents := math.Round(sum*100) - float64(monthlyBudgetCents)
		deltaAbs := math.Abs(deltaCents) / 100
		balance := DistributionBalance{Unit: kind, Target: target, Sum: sum, DeltaAbs: deltaAbs}
		switch {
		case math.Abs(deltaCents) <= distributionReaisToleranceCents:
			balance.Status = distributionBalanced
		case deltaCents > 0:
			balance.Status = distributionOver
		default:
			balance.Status = distributionUnder
		}
		return balance
	}

	target := 100.0
	delta := sum - target
	deltaAbs := math.Abs(delta)
	balance := DistributionBalance{Unit: allocationInputPercent, Target: target, Sum: sum, DeltaAbs: deltaAbs}
	switch {
	case deltaAbs <= distributionPercentToleranceAbs:
		balance.Status = distributionBalanced
	case delta > 0:
		balance.Status = distributionOver
	default:
		balance.Status = distributionUnder
	}
	return balance
}

type OnboardingState struct {
	Phase                  OnboardingPhase `json:"phase"`
	UserID                 string          `json:"userID"`
	PeerID                 string          `json:"peerID"`
	Goal                   string          `json:"goal"`
	GoalValueCents         int64           `json:"goalValueCents"`
	GoalValueAsked         bool            `json:"goalValueAsked"`
	TreatmentName          string          `json:"treatmentName"`
	TreatmentNameAsked     bool            `json:"treatmentNameAsked"`
	MonthlyBudgetCents     int64           `json:"monthlyBudgetCents"`
	ReviewAwait            reviewAwaitKind `json:"reviewAwait"`
	CardsDone              bool            `json:"cardsDone"`
	Allocations            map[string]int  `json:"allocations"`
	Recurrence             bool            `json:"recurrence"`
	RecurrenceMonths       int             `json:"recurrenceMonths"`
	ResumeText             string          `json:"resumeText"`
	FinalMessage           string          `json:"finalMessage"`
	GoalConfirmation       string          `json:"goalConfirmation"`
	RecurrenceConfirmation string          `json:"recurrenceConfirmation"`
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

type recurrenceIntentKind int

const (
	recurrenceIntentNegative recurrenceIntentKind = iota + 1
	recurrenceIntentPositive
	recurrenceIntentUnclear
)

var errInvalidRecurrenceIntent = errors.New("recurrence: tipo de intencao invalido")

func (k recurrenceIntentKind) String() string {
	switch k {
	case recurrenceIntentNegative:
		return "negative"
	case recurrenceIntentPositive:
		return "positive"
	case recurrenceIntentUnclear:
		return "unclear"
	default:
		return "unknown"
	}
}

func (k recurrenceIntentKind) IsValid() bool {
	return k >= recurrenceIntentNegative && k <= recurrenceIntentUnclear
}

func ParseRecurrenceIntentKind(s string) (recurrenceIntentKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "negative":
		return recurrenceIntentNegative, nil
	case "positive":
		return recurrenceIntentPositive, nil
	case "unclear":
		return recurrenceIntentUnclear, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidRecurrenceIntent, s)
	}
}

type recurrenceOutcomeKind int

const (
	recurrenceOutcomeNone recurrenceOutcomeKind = iota + 1
	recurrenceOutcomeDefault
	recurrenceOutcomeSpecific
	recurrenceOutcomeInvalid
	recurrenceOutcomeAmbiguous
)

func (k recurrenceOutcomeKind) String() string {
	switch k {
	case recurrenceOutcomeNone:
		return "no_recurrence"
	case recurrenceOutcomeDefault:
		return "default_12"
	case recurrenceOutcomeSpecific:
		return "specific_months"
	case recurrenceOutcomeInvalid:
		return "invalid_reprompt"
	case recurrenceOutcomeAmbiguous:
		return "ambiguous_reprompt"
	default:
		return "unknown"
	}
}

func (k recurrenceOutcomeKind) IsValid() bool {
	return k >= recurrenceOutcomeNone && k <= recurrenceOutcomeAmbiguous
}

type recurrenceDecision struct {
	Outcome recurrenceOutcomeKind
	Months  int
}

const (
	recurrenceDefaultMonths = 12
	recurrenceMinMonths     = 1
	recurrenceMaxMonths     = 12
)

func DecideRecurrence(intent recurrenceIntentKind, hasMonths bool, months int) recurrenceDecision {
	if hasMonths {
		if months >= recurrenceMinMonths && months <= recurrenceMaxMonths {
			return recurrenceDecision{Outcome: recurrenceOutcomeSpecific, Months: months}
		}
		return recurrenceDecision{Outcome: recurrenceOutcomeInvalid}
	}
	switch intent {
	case recurrenceIntentPositive:
		return recurrenceDecision{Outcome: recurrenceOutcomeDefault, Months: recurrenceDefaultMonths}
	case recurrenceIntentNegative:
		return recurrenceDecision{Outcome: recurrenceOutcomeNone}
	default:
		return recurrenceDecision{Outcome: recurrenceOutcomeAmbiguous}
	}
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

var errAllocationOutOfTolerance = errors.New("distribution: soma fora da tolerancia de fechamento")

func DecideAllocationsBP(kind allocationInputKind, valuesBySlug map[string]float64, monthlyBudgetCents int64) (map[string]int, error) {
	switch kind {
	case allocationInputConfirm:
		return decideAllocationsBPConfirm(valuesBySlug)
	case allocationInputPercent:
		return decideAllocationsBPPercent(valuesBySlug, monthlyBudgetCents)
	case allocationInputReais:
		return decideAllocationsBPReais(valuesBySlug, monthlyBudgetCents)
	default:
		return nil, errInvalidAllocationInput
	}
}

func decideAllocationsBPConfirm(valuesBySlug map[string]float64) (map[string]int, error) {
	if sumAllocationValues(valuesBySlug) > 0 {
		return nil, errAllocationConfirmWithValues
	}
	return maps.Clone(defaultDistributionBP), nil
}

func decideAllocationsBPPercent(valuesBySlug map[string]float64, monthlyBudgetCents int64) (map[string]int, error) {
	millis := make([]int64, len(canonicalSlugs))
	for i, slug := range canonicalSlugs {
		v := valuesBySlug[slug]
		if v < 0 {
			return nil, fmt.Errorf("o percentual de %s não pode ser negativo", categoryLabels[slug])
		}
		millis[i] = int64(math.Round(v * 1000))
	}
	balance := DecideDistributionBalance(allocationInputPercent, valuesBySlug, monthlyBudgetCents)
	if balance.Status != distributionBalanced {
		return nil, errAllocationOutOfTolerance
	}
	return finalizeAllocationsBP(millis, sumUnits(millis))
}

func decideAllocationsBPReais(valuesBySlug map[string]float64, monthlyBudgetCents int64) (map[string]int, error) {
	if monthlyBudgetCents <= 0 {
		return nil, errors.New("não consegui usar seu orçamento mensal para converter os valores")
	}
	cents := make([]int64, len(canonicalSlugs))
	for i, slug := range canonicalSlugs {
		v := valuesBySlug[slug]
		if v < 0 {
			return nil, fmt.Errorf("o valor de %s não pode ser negativo", categoryLabels[slug])
		}
		cents[i] = int64(math.Round(v * 100))
	}
	balance := DecideDistributionBalance(allocationInputReais, valuesBySlug, monthlyBudgetCents)
	if balance.Status != distributionBalanced {
		return nil, errAllocationOutOfTolerance
	}
	return finalizeAllocationsBP(cents, sumUnits(cents))
}

func sumUnits(units []int64) int64 {
	var total int64
	for _, u := range units {
		total += u
	}
	return total
}

func finalizeAllocationsBP(units []int64, total int64) (map[string]int, error) {
	if total <= 0 {
		return nil, errAllocationOutOfTolerance
	}
	bpSlice := centsToBasisPoints(units, total)
	bp := make(map[string]int, len(canonicalSlugs))
	for i, slug := range canonicalSlugs {
		bp[slug] = bpSlice[i]
	}
	if err := DecideDistribution(bp); err != nil {
		return nil, err
	}
	return bp, nil
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

type cardMissingField int

const (
	cardMissingNone cardMissingField = iota + 1
	cardMissingName
	cardMissingDueDay
	cardMissingNameAndDueDay
)

func classifyCardMissing(nickname, bank string, dueDay int) cardMissingField {
	hasName := strings.TrimSpace(nickname) != "" || strings.TrimSpace(bank) != ""
	hasDueDay := dueDay >= 1 && dueDay <= 31
	switch {
	case hasName && hasDueDay:
		return cardMissingNone
	case !hasName && !hasDueDay:
		return cardMissingNameAndDueDay
	case !hasName:
		return cardMissingName
	default:
		return cardMissingDueDay
	}
}

func normalizeCardExtract(extract cardExtract) cardExtract {
	nickname := strings.TrimSpace(extract.Nickname)
	bank := strings.TrimSpace(extract.Bank)
	switch {
	case nickname == "" && bank != "":
		extract.Nickname = bank
	case bank == "" && nickname != "":
		extract.Bank = nickname
	}
	return extract
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
	MixedUnit           bool    `json:"mixed_unit"`
}

type distributionIntentExtract struct {
	Action    string `json:"action"`
	MixedUnit bool   `json:"mixed_unit"`
}

type yesNoExtract struct {
	Confirmed bool `json:"confirmed"`
}

type recurrenceExtract struct {
	Intent    string `json:"intent"`
	HasMonths bool   `json:"hasMonths"`
	Months    int    `json:"months"`
}

type treatmentNameExtract struct {
	HasName bool   `json:"hasName"`
	Name    string `json:"name"`
}

var treatmentNameSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"hasName": map[string]any{"type": "boolean"},
		"name":    map[string]any{"type": "string"},
	},
	"required":             []any{"hasName", "name"},
	"additionalProperties": false,
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
		"mixed_unit":           map[string]any{"type": "boolean"},
	},
	"required":             []any{"action", "custo_fixo", "conhecimento", "prazeres", "metas", "liberdade_financeira", "mixed_unit"},
	"additionalProperties": false,
}

var distributionIntentSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"action":     map[string]any{"type": "string", "enum": []any{"accept", "personalize", "values"}},
		"mixed_unit": map[string]any{"type": "boolean"},
	},
	"required":             []any{"action", "mixed_unit"},
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

var recurrenceDecisionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent":    map[string]any{"type": "string", "enum": []any{"negative", "positive", "unclear"}},
		"hasMonths": map[string]any{"type": "boolean"},
		"months":    map[string]any{"type": "integer"},
	},
	"required":             []any{"intent", "hasMonths", "months"},
	"additionalProperties": false,
}

const welcomeCombinedPrompt = `🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar um celular novo, meta de R$ 5.000,00")`

const treatmentNameCapturePrompt = `🎉 Bem-vindo ao MeControla! 🎉

Estou aqui para te ajudar a organizar suas finanças e conquistar seus objetivos. 💪💰

Antes da gente começar, como você gostaria que eu te chamasse? 💚`

const treatmentNameGoalPromptNoGreeting = `Vamos começar? Qual é o seu principal objetivo financeiro para este mês?
(por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva; se quiser, já pode me contar o valor da meta, tipo "comprar um celular novo, meta de R$ 5.000,00")`

const treatmentNameWMSectionHeading = "## Nome de Tratamento"

const treatmentNameSystemPrompt = "Extraia como o usuário quer ser chamado (nome/apelido de tratamento) a partir do texto. " +
	"Defina hasName=true quando o usuário indicar um nome, apelido ou forma de tratamento utilizável, cobrindo variações como nome direto, \"pode me chamar de X\", \"me chama de X\", \"prefiro X\", \"meu apelido é X\", \"só X mesmo\", \"X tá bom\". " +
	"Quando o usuário indicar explicitamente como quer ser chamado, use esse apelido em name; quando informar apenas um nome completo/composto sem indicar apelido, use o primeiro nome em name. " +
	"Defina hasName=false e name vazio quando o usuário recusar (ex.: \"não\", \"tanto faz\", \"prefiro não dizer\") ou responder diretamente sobre outro assunto sem mencionar nenhum nome/apelido utilizável — nunca invente um nome que não esteja no texto."

const goalReprompt = "Não consegui identificar seu objetivo. Qual é o seu principal objetivo financeiro para este mês? Por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva. Se souber, pode me contar também o valor da meta — mas isso é totalmente opcional."

const goalValueReprompt = "Legal! E você já tem uma ideia de quanto (em R$) representa essa meta? Pode responder com um número, por exemplo \"R$ 5.000,00\" ou \"5 mil\" — se preferir não informar agora, é só responder \"não\" que a gente segue em frente."

func goalConfirmationReprompt(goal string) string {
	return fmt.Sprintf(
		"Perfeito! Anotei seu objetivo: \"%s\" 🎯 Vamos juntos tornar isso realidade! 💪\n\n%s",
		goal,
		goalValueReprompt,
	)
}

const goalWithValueConfirmation = "Perfeito! Anotei seu objetivo: \"%s\" com meta de %s 🎯 Vamos juntos tornar isso realidade! 💪"

const goalValueLaterConfirmation = "Show! Meta de %s anotada. 🎯"

const monthlyBudgetPrompt = `📊 Antes de montar seu planejamento, deixa eu te mostrar como organizamos o dinheiro por aqui.

O dinheiro vive em apenas 5 categorias:

💰 Custo Fixo: contas essenciais e compromissos recorrentes, como moradia, mercado, transporte e saúde.
🎓 Conhecimento: cursos, livros, mentorias e aprendizados que aumentam sua capacidade de ganhar ou administrar dinheiro.
🎉 Prazeres: lazer, restaurantes, viagens, presentes e escolhas que trazem qualidade de vida.
🎯 Metas: objetivos com prazo e valor definidos, como quitar dívida, comprar algo importante ou montar uma reserva específica.
🏦 Liberdade Financeira: reserva de emergência, investimentos e aportes para independência financeira.

Qual é o seu orçamento mensal? (por exemplo: R$ 3.500,00)`

const monthlyBudgetReprompt = "Não consegui identificar o valor. Qual é o seu orçamento mensal? Por exemplo: R$ 3.500,00."

const cardsReprompt = "Para adicionar o cartão, me diga o apelido, o banco emissor e o dia de vencimento da fatura (um número entre 1 e 31).\n\n" +
	"Por exemplo:\n" +
	"• \"Roxinho, Nubank e vencimento dia 1\"\n" +
	"• \"Nubank e vencimento dia primeiro\" (sem apelido, o apelido do cartão fica igual ao banco)\n\n" +
	"Se preferir não adicionar agora, responda \"não\"."

const (
	cardsRepromptMissingName = "Para adicionar o cartão, preciso do apelido ou do banco emissor.\n\n" +
		"Por exemplo:\n" +
		"• \"Roxinho, Nubank\"\n" +
		"• \"Nubank\" (sem apelido, o apelido do cartão fica igual ao banco)\n\n" +
		"Se não quiser cadastrar agora, é só responder \"não\"."
	cardsRepromptMissingDueDay = "Para adicionar o cartão, preciso do dia de vencimento da fatura (um número entre 1 e 31).\n\n" +
		"Por exemplo:\n" +
		"• \"dia 1\"\n" +
		"• \"dia primeiro\"\n\n" +
		"Se não quiser cadastrar agora, é só responder \"não\"."
	cardsRepromptMissingBoth = "Para adicionar o cartão, preciso do apelido/banco emissor e do dia de vencimento da fatura (entre 1 e 31).\n\n" +
		"Por exemplo:\n" +
		"• \"Roxinho, Nubank e vencimento dia 1\"\n" +
		"• \"Nubank e vencimento dia primeiro\" (sem apelido, o apelido do cartão fica igual ao banco)\n\n" +
		"Se preferir não adicionar agora, responda \"não\"."
)

const cardCreatedSuccessOnboarding = "💳 Cartão registrado com sucesso ✅\nQuer registrar algum outro?"

func cardsRepromptFor(missing cardMissingField) string {
	switch missing {
	case cardMissingName:
		return cardsRepromptMissingName
	case cardMissingDueDay:
		return cardsRepromptMissingDueDay
	case cardMissingNameAndDueDay:
		return cardsRepromptMissingBoth
	default:
		return cardsReprompt
	}
}

const conclusionRecurrencePrompt = "📊 Quer que eu repita esse orçamento automaticamente todo mês, sem precisar configurar de novo? Você pode responder \"sim\" (repete por 12 meses), informar uma quantidade de 1 a 12 meses (ex.: \"só por 6 meses\"), ou \"não\" se preferir configurar de novo depois."

const recurrenceInvalidReprompt = "Consigo repetir esse orçamento automaticamente por um período entre 1 e 12 meses. Quantos meses você quer? Responda com um número de 1 a 12, \"sim\" para repetir por 12 meses, ou \"não\" para não repetir."

const recurrenceConfirmationNone = "Combinado, não vou repetir esse orçamento automaticamente. ✅"

const recurrenceConfirmationDefault = "Perfeito! Vou repetir esse orçamento automaticamente pelos próximos 12 meses. ✅"

const recurrenceConfirmationTemplate = "Perfeito! Vou repetir esse orçamento automaticamente por %s. ✅"

func monthsLabel(n int) string {
	if n == 1 {
		return "1 mês"
	}
	return fmt.Sprintf("%d meses", n)
}

func recurrenceConfirmationFor(months int) string {
	if months == recurrenceDefaultMonths {
		return recurrenceConfirmationDefault
	}
	return fmt.Sprintf(recurrenceConfirmationTemplate, monthsLabel(months))
}

const allocationInputSystemPrompt = "Você classifica a resposta do usuário sobre a distribuição do orçamento em 5 categorias: custo_fixo, conhecimento, prazeres, metas, liberdade_financeira. " +
	"Defina action='confirm' SOMENTE quando o usuário aceitar a sugestão sem informar nenhum valor novo (ex.: sim, aceito, pode confirmar, ok); nunca use 'confirm' quando o texto contiver números para as categorias. " +
	"Defina action='reais' quando o usuário informar valores em reais — valores acompanhados de R$/reais ou números grandes cuja soma se aproxima do orçamento mensal (ex.: 2500, 500, 2000). " +
	"Defina action='percent' quando ele informar percentuais — números pequenos, acompanhados de % ou cuja soma se aproxima de 100. " +
	"Em caso de dúvida entre 'reais' e 'percent', escolha 'reais' se a soma dos números se aproximar do orçamento mensal e 'percent' se a soma se aproximar de 100; jamais coaja valores em reais para percentuais ou vice-versa. " +
	"Preencha cada categoria com o número informado pelo usuário; use 0 quando a categoria não for citada. " +
	"Converta valores por extenso para número, sempre em ponto decimal, nunca com símbolo de moeda ou separador de milhar. Exemplos de conversão: " +
	"\"mil reais\" -> 1000; " +
	"\"quinhentos\" -> 500; " +
	"\"dois mil e quinhentos\" -> 2500; " +
	"\"dez mil\" -> 10000; " +
	"\"quarenta por cento\" -> 40. " +
	"Defina mixed_unit=true quando a MESMA mensagem misturar unidades diferentes entre as categorias informadas (ex.: 'custo fixo 40%, prazeres R$ 300' mistura porcentagem e reais); caso contrário mixed_unit=false. " +
	"Quando mixed_unit=true, ainda assim preencha action e os valores como de costume, pois a ambiguidade de unidade é tratada separadamente."

const distributionIntentSystemPrompt = "Você classifica a intenção do usuário sobre o passo de distribuição do orçamento em 5 categorias. " +
	"Retorne action e mixed_unit; NÃO extraia valores por categoria aqui. " +
	"Precedência obrigatória entre as três intenções: 'values' > 'personalize' > 'accept' — se a mensagem contém números utilizáveis para pelo menos uma categoria (em reais, porcentagem ou por extenso), a ação é SEMPRE 'values', mesmo que o texto também contenha a palavra 'não' ou 'nao' (ex.: 'não, quero 40% em custo fixo e o resto dividido' -> action='values', pois há número; 'não, prefiro escolher os valores' sem número nenhum -> action='personalize'). " +
	"Defina action='accept' somente quando o usuário aceitar a sugestão sem recusar e sem informar nenhum valor novo (ex.: 'sim', 'aceito', 'pode confirmar', 'ok', 'topo'). " +
	"Defina action='personalize' quando o usuário recusar a sugestão ou pedir para escolher/personalizar sem informar nenhum número usável (ex.: 'não', 'nao', 'quero personalizar', 'prefiro escolher', 'quero mudar os valores'). " +
	"Defina action='values' quando o usuário informar valores usáveis para pelo menos uma categoria, em reais ('R$ 1.000,00', '1000'), em porcentagem ('40%', '40') ou por extenso ('mil reais' -> 1000, 'quinhentos' -> 500, 'dois mil e quinhentos' -> 2500, 'dez mil' -> 10000, 'quarenta por cento' -> 40). " +
	"Defina mixed_unit=true quando a MESMA mensagem misturar unidades diferentes entre as categorias informadas (ex.: 'custo fixo 40%, prazeres R$ 300' mistura porcentagem e reais); caso contrário mixed_unit=false. " +
	"Quando mixed_unit=true, ainda assim retorne o action mais adequado (normalmente 'values'), pois a ambiguidade de unidade é tratada separadamente da intenção."

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

const monthlyBudgetSystemPrompt = "Extraia o valor do orcamento mensal em reais (BRL) do texto do usuario. Retorne como numero decimal. " +
	"Converta o valor mencionado para um número em reais (amountBRL), sempre em ponto decimal, nunca com símbolo de moeda ou separador de milhar. Exemplos de conversão: " +
	"\"R$ 3.500,00\" -> 3500; " +
	"\"3500\" -> 3500; " +
	"\"mil e quinhentos reais\" -> 1500; " +
	"\"mil reais\" -> 1000; " +
	"\"dois mil e quinhentos\" -> 2500; " +
	"\"dez mil\" -> 10000."

const cardsSystemPrompt = "Extraia do texto do usuario se ele quer adicionar um 💳 (wantsCard), o apelido (nickname), o banco emissor (bank) e o dia de vencimento (dueDay, inteiro 1-31). Se nao quiser 💳, retorne wantsCard=false, nickname vazio, bank vazio e dueDay=0."

const recurrenceDecisionSystemPrompt = "Você extrai a resposta do usuário sobre repetir o orçamento automaticamente pelos próximos meses. " +
	"Retorne intent, hasMonths e months; NÃO decida prioridade nem limites aqui — isso é resolvido por outra função. " +
	"Defina intent='positive' quando o usuário aceitar a recorrência (ex.: 'sim', 'pode', 'quero', 'confirmo', 'topo'), mesmo que também informe uma quantidade de meses. " +
	"Defina intent='negative' quando o usuário recusar (ex.: 'não', 'nao', 'não quero', 'não precisa'). " +
	"Defina intent='unclear' quando não houver intenção reconhecível na mensagem (ex.: 'talvez', 'sei lá', emoji isolado, texto sem relação com a pergunta). " +
	"Defina hasMonths=true e preencha months sempre que o usuário mencionar uma quantidade de meses, numérica ou por extenso, mesmo que fora do intervalo permitido — não corrija nem limite o valor aqui, apenas extraia o número informado. " +
	"Converta números por extenso para inteiro. Exemplos de conversão: " +
	"\"um\" -> 1; \"dois\" -> 2; \"três\" -> 3; \"quatro\" -> 4; \"cinco\" -> 5; \"seis\" -> 6; \"sete\" -> 7; \"oito\" -> 8; \"nove\" -> 9; \"dez\" -> 10; \"onze\" -> 11; \"doze\" -> 12. " +
	"Se o usuário não mencionar nenhuma quantidade, defina hasMonths=false e months=0."

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
		return fmt.Sprintf(
			"Você já tem %d cartão 💳 cadastrado(s). Deseja cadastrar **outro** cartão agora?\n\n"+
				"Por exemplo:\n"+
				"• \"Roxinho, Nubank e vencimento dia 1\"\n"+
				"• \"Nubank e vencimento dia primeiro\" (sem apelido, o apelido fica igual ao banco)\n\n"+
				"Se não, responda \"não\".",
			existing,
		)
	}
	return "O cartão 💳 é opcional. Você deseja cadastrar um cartão agora?\n\n" +
		"Por exemplo:\n" +
		"• \"Roxinho, Nubank e vencimento dia 1\"\n" +
		"• \"Nubank e vencimento dia primeiro\" (sem apelido, o apelido fica igual ao banco)\n\n" +
		"Se não quiser agora, é só responder \"não\" e seguir sem cartão."
}

func methodologyPrompt(items []interfaces.AllocationCents) string {
	var b strings.Builder
	b.WriteString("Agora vamos distribuir seu orçamento. O MeControla organiza tudo em 5 categorias. Esta é a sugestão com base no seu orçamento mensal:\n\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString("\nAceita esta sugestão? Responda \"sim\" para confirmar, envie novos valores para cada categoria — pode ser em reais (R$) ou em porcentagem (%) — ou responda \"não\" para personalizar e me dizer você mesmo quanto quer em cada categoria.")
	return b.String()
}

func formatDistributionValue(unit allocationInputKind, v float64) string {
	if unit == allocationInputReais {
		return money.FromCents(int64(math.Round(v * 100))).BRL()
	}
	return fmt.Sprintf("%.0f%%", v)
}

func renderEchoedValues(unit allocationInputKind, valuesBySlug map[string]float64) string {
	var b strings.Builder
	for _, slug := range canonicalSlugs {
		fmt.Fprintf(&b, "%s: %s\n", categoryLabels[slug], formatDistributionValue(unit, valuesBySlug[slug]))
	}
	return b.String()
}

func renderBalanceMessage(balance DistributionBalance, valuesBySlug map[string]float64) string {
	var b strings.Builder
	targetLabel := "100%"
	if balance.Unit == allocationInputReais {
		targetLabel = money.FromCents(int64(math.Round(balance.Target * 100))).BRL()
	}
	switch balance.Status {
	case distributionOver:
		fmt.Fprintf(&b, "A soma que você enviou passou %s do que precisamos fechar (%s).\n\n", formatDistributionValue(balance.Unit, balance.DeltaAbs), targetLabel)
	case distributionUnder:
		fmt.Fprintf(&b, "A soma que você enviou ainda falta %s para fechar o total (%s).\n\n", formatDistributionValue(balance.Unit, balance.DeltaAbs), targetLabel)
	default:
		fmt.Fprintf(&b, "A distribuição precisa fechar %s no total.\n\n", targetLabel)
	}
	b.WriteString("Veja o que entendi de cada categoria:\n")
	b.WriteString(renderEchoedValues(balance.Unit, valuesBySlug))
	b.WriteString("\nMe envie os valores novamente, por categoria, para fechar exatamente o total.")
	return b.String()
}

func methodologyReprompt(reason string, items []interfaces.AllocationCents) string {
	return "Ops, não consegui aplicar essa distribuição: " + reason + "\n\n" + methodologyPrompt(items)
}

func personalizePrompt(monthlyBudgetCents int64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Combinado! Vamos personalizar sua distribuição. Seu orçamento mensal é %s e ele precisa ser todo distribuído entre as 5 categorias:\n\n", money.FromCents(monthlyBudgetCents).BRL())
	for _, slug := range canonicalSlugs {
		fmt.Fprintf(&b, "%s\n", categoryLabels[slug])
	}
	b.WriteString("\nMe diga quanto vai para cada categoria — pode ser em reais (R$) ou em porcentagem (%), sempre somando o total do orçamento (ou 100%). Se alguma categoria não fizer sentido para você agora, pode colocar ZERO nela, sem problema.")
	return b.String()
}

func personalizeReprompt(monthlyBudgetCents int64) string {
	return "Não consegui identificar valores para as categorias. " + personalizePrompt(monthlyBudgetCents)
}

const distributionMixedUnitPrompt = "Entendi valores misturando reais (R$) e porcentagem (%) na mesma resposta. Use uma única unidade para todas as categorias — ou só em reais (R$), ou só em porcentagem (%) — e me envie de novo."

func zeroedCategoriesWarning(items []interfaces.AllocationCents) string {
	bySlug := allocationCentsBySlug(items)
	var zeroed []string
	for _, slug := range canonicalSlugs {
		if bySlug[slug].BasisPoints == 0 {
			zeroed = append(zeroed, categoryLabels[slug])
		}
	}
	if len(zeroed) == 0 {
		return ""
	}
	return "\n⚠️ Estas categorias ficarão zeradas (R$ 0,00 / 0%): " + strings.Join(zeroed, ", ") + ".\n"
}

func summaryPrompt(state OnboardingState, items []interfaces.AllocationCents) string {
	var b strings.Builder
	b.WriteString("Vamos revisar tudo antes de ativar seu orçamento:\n\n")
	fmt.Fprintf(&b, "🎯 Objetivo: %s\n", state.Goal)
	fmt.Fprintf(&b, "💵 Orçamento mensal: %s\n", money.FromCents(state.MonthlyBudgetCents).BRL())
	b.WriteString("\nDistribuição:\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString(zeroedCategoriesWarning(items))
	b.WriteString("\nPosso ativar seu orçamento com esses dados? Responda \"sim\" para confirmar ou \"não\" para revisar a distribuição.")
	return b.String()
}

func conclusionFinalMessage() string {
	return "Tudo pronto! 🚀\n\n" +
		"Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: \"gastei R$ 50 no mercado\" ou \"recebi R$ 200 de freela\") que eu registro tudo pra você. Vamos juntos! 💪"
}

func cardSummaryLine(card interfaces.Card) string {
	if card.Nickname == card.Bank {
		return fmt.Sprintf("- %s — vencimento dia %d", card.Bank, card.DueDay)
	}
	return fmt.Sprintf("- %s (%s) — vencimento dia %d", card.Nickname, card.Bank, card.DueDay)
}

func renderCardsSummary(cards []interfaces.Card) string {
	if len(cards) == 0 {
		return "Nenhum cartão cadastrado."
	}
	var b strings.Builder
	for i, card := range cards {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(cardSummaryLine(card))
	}
	return b.String()
}

func recurrenceSummaryLine(recurrence bool, months int) string {
	if !recurrence {
		return "🔁 Recorrência: desligada"
	}
	effectiveMonths := months
	if effectiveMonths <= 0 {
		effectiveMonths = recurrenceDefaultMonths
	}
	return fmt.Sprintf("🔁 Recorrência: ligada (repete pelos próximos %s)", monthsLabel(effectiveMonths))
}

func conclusionSummaryMessage(state OnboardingState, items []interfaces.AllocationCents, cards []interfaces.Card) string {
	var b strings.Builder
	b.WriteString("Resumo de Onboarding\n\n")
	if state.GoalValueCents > 0 {
		fmt.Fprintf(&b, "🎯 Objetivo: %s (meta de %s)\n", state.Goal, money.FromCents(state.GoalValueCents).BRL())
	} else {
		fmt.Fprintf(&b, "🎯 Objetivo: %s\n", state.Goal)
	}
	fmt.Fprintf(&b, "💵 Orçamento mensal: %s\n\n", money.FromCents(state.MonthlyBudgetCents).BRL())
	b.WriteString("Distribuição:\n")
	b.WriteString(renderAllocationLines(items))
	b.WriteString("\nCartões:\n")
	b.WriteString(renderCardsSummary(cards))
	b.WriteString("\n\n")
	b.WriteString(recurrenceSummaryLine(state.Recurrence, state.RecurrenceMonths))
	b.WriteString("\n\n")
	b.WriteString(conclusionFinalMessage())
	return b.String()
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

const treatmentNameOutcomeMetric = "agents_onboarding_treatment_name_total"

const (
	treatmentNameOutcomeCaptured   = "captured"
	treatmentNameOutcomeSkipped    = "skipped"
	treatmentNameOutcomeParseError = "parse_error"
)

func newTreatmentNameOutcomeCounter(o11y observability.Observability) observability.Counter {
	return o11y.Metrics().Counter(
		treatmentNameOutcomeMetric,
		"Total de resultados da captura do nome de tratamento no onboarding",
		"1",
	)
}

func recordTreatmentNameOutcome(ctx context.Context, tn observability.Counter, outcome string) {
	if tn == nil {
		return
	}
	tn.Add(ctx, 1, observability.String("outcome", outcome))
}

func BuildWelcomeStep(a agent.Agent, tn observability.Counter) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseWelcome
		if state.ResumeText == "" {
			state.TreatmentNameAsked = true
			return suspendStep(state, treatmentNameCapturePrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: treatmentNameSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "treatment_name_extract", Strict: true, Schema: treatmentNameSchema},
		})
		if err != nil {
			recordTreatmentNameOutcome(ctx, tn, treatmentNameOutcomeParseError)
			return completeStep(state), nil
		}
		var extract treatmentNameExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			recordTreatmentNameOutcome(ctx, tn, treatmentNameOutcomeParseError)
			return completeStep(state), nil
		}
		name, ok := DecideTreatmentName(extract.HasName, extract.Name)
		if !ok {
			recordTreatmentNameOutcome(ctx, tn, treatmentNameOutcomeSkipped)
			return completeStep(state), nil
		}
		state.TreatmentName = name
		recordTreatmentNameOutcome(ctx, tn, treatmentNameOutcomeCaptured)
		return completeStep(state), nil
	}
}

func BuildGoalStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseGoal
			return suspendStep(state, treatmentNameGoalPromptNoGreeting), nil
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
				return suspendStep(state, goalConfirmationReprompt(state.Goal)), nil
			}
			if state.GoalValueCents > 0 {
				state.GoalConfirmation = fmt.Sprintf(goalWithValueConfirmation, state.Goal, money.FromCents(state.GoalValueCents).BRL())
			}
			return completeStep(state), nil
		}

		if !state.GoalValueAsked {
			state.GoalValueAsked = true
			return suspendStep(state, goalConfirmationReprompt(state.Goal)), nil
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
			state.GoalConfirmation = fmt.Sprintf(goalValueLaterConfirmation, money.FromCents(state.GoalValueCents).BRL())
		}
		return completeStep(state), nil
	}
}

const monthlyBudgetOutcomeMetric = "agents_onboarding_monthly_budget_total"

const (
	monthlyBudgetOutcomeParsedOK   = "parsed_ok"
	monthlyBudgetOutcomeReprompt   = "reprompt"
	monthlyBudgetOutcomeParseError = "parse_error"
)

func newMonthlyBudgetOutcomeCounter(o11y observability.Observability) observability.Counter {
	return o11y.Metrics().Counter(
		monthlyBudgetOutcomeMetric,
		"Total de resultados do passo de orcamento mensal do onboarding",
		"1",
	)
}

func recordMonthlyBudgetOutcome(ctx context.Context, mb observability.Counter, outcome string) {
	if mb == nil {
		return
	}
	mb.Add(ctx, 1, observability.String("outcome", outcome))
}

func BuildMonthlyBudgetStep(a agent.Agent, mb observability.Counter) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseMonthlyBudget
			if state.GoalConfirmation != "" {
				prompt := state.GoalConfirmation + "\n\n" + monthlyBudgetPrompt
				state.GoalConfirmation = ""
				return suspendStep(state, prompt), nil
			}
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
			recordMonthlyBudgetOutcome(ctx, mb, monthlyBudgetOutcomeParseError)
			return failStep(state, fmt.Errorf("agents.onboarding.monthly_budget: parse: %w", err))
		}
		var extract monthlyBudgetExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			recordMonthlyBudgetOutcome(ctx, mb, monthlyBudgetOutcomeParseError)
			return failStep(state, fmt.Errorf("agents.onboarding.monthly_budget: unmarshal: %w", err))
		}
		cents, err := DecideMonthlyBudgetCents(extract.AmountBRL)
		if err != nil {
			recordMonthlyBudgetOutcome(ctx, mb, monthlyBudgetOutcomeReprompt)
			return suspendStep(state, monthlyBudgetReprompt), nil
		}
		state.MonthlyBudgetCents = cents
		recordMonthlyBudgetOutcome(ctx, mb, monthlyBudgetOutcomeParsedOK)
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
			prompt := cardsPrompt(len(existingCards))
			if state.RecurrenceConfirmation != "" {
				prompt = state.RecurrenceConfirmation + "\n\n" + prompt
				state.RecurrenceConfirmation = ""
			}
			return suspendStep(state, prompt), nil
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
		extract = normalizeCardExtract(extract)
		if !extract.WantsCard {
			state.CardsDone = true
			return completeStep(state), nil
		}
		missing := classifyCardMissing(extract.Nickname, extract.Bank, extract.DueDay)
		if missing != cardMissingNone {
			return suspendStep(state, cardsRepromptFor(missing)), nil
		}
		if err := DecideCardEntry(extract.Nickname, extract.Bank, extract.DueDay); err != nil {
			return suspendStep(state, cardsRepromptFor(cardMissingNameAndDueDay)), nil
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
		if _, err := cards.ListCards(ctx, userUUID); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.cards: list_cards_after_create: %w", err))
		}
		return suspendStep(state, cardCreatedSuccessOnboarding), nil
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

const distributionOutcomeMetric = "agents_onboarding_distribution_total"

const (
	distributionOutcomePersonalizeEntered = "personalize_entered"
	distributionOutcomeAcceptedDefault    = "accepted_default"
	distributionOutcomeAcceptedValues     = "accepted_values"
	distributionOutcomeOver               = "over"
	distributionOutcomeUnder              = "under"
	distributionOutcomeMixedUnit          = "mixed_unit"
	distributionOutcomeToleranceAbsorbed  = "tolerance_absorbed"
)

func newDistributionOutcomeCounter(o11y observability.Observability) observability.Counter {
	return o11y.Metrics().Counter(
		distributionOutcomeMetric,
		"Total de resultados do passo de distribuicao personalizada do onboarding",
		"1",
	)
}

func recordDistributionOutcome(ctx context.Context, dist observability.Counter, outcome string) {
	if dist == nil {
		return
	}
	dist.Add(ctx, 1, observability.String("outcome", outcome))
}

func BuildBudgetReviewStep(a agent.Agent, budgets interfaces.BudgetPlanner, dist observability.Counter) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
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
			return handleReviewAwaitDistribution(ctx, a, budgets, dist, state)
		case reviewAwaitPersonalize:
			return handleReviewAwaitPersonalize(ctx, a, budgets, dist, state)
		case reviewAwaitConfirm:
			return handleReviewAwaitConfirm(ctx, a, budgets, state)
		default:
			return failStep(state, fmt.Errorf("agents.onboarding.budget_review: %w", errInvalidReviewAwaitKind))
		}
	}
}

func classifyDistributionIntent(ctx context.Context, a agent.Agent, resumeText string) (distributionIntentKind, bool, error) {
	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: distributionIntentSystemPrompt},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "distribution_intent", Strict: true, Schema: distributionIntentSchema},
	})
	if err != nil {
		return 0, false, fmt.Errorf("classify_intent: %w", err)
	}
	var intent distributionIntentExtract
	if err := json.Unmarshal(extracted.RawJSON, &intent); err != nil {
		return 0, false, fmt.Errorf("unmarshal_intent: %w", err)
	}
	kind, _ := ParseDistributionIntentKind(intent.Action)
	return kind, intent.MixedUnit, nil
}

func extractAllocationValues(ctx context.Context, a agent.Agent, resumeText string) (allocationInputKind, map[string]float64, bool, error) {
	extracted, err := a.Execute(ctx, agent.Request{
		Messages: []llm.Message{
			{Role: "system", Content: allocationInputSystemPrompt},
			{Role: "user", Content: resumeText},
		},
		Schema: &llm.Schema{Name: "allocation_input", Strict: true, Schema: allocationInputSchema},
	})
	if err != nil {
		return 0, nil, false, fmt.Errorf("parse_allocation: %w", err)
	}
	var input allocationInputExtract
	if err := json.Unmarshal(extracted.RawJSON, &input); err != nil {
		return 0, nil, false, fmt.Errorf("unmarshal_allocation: %w", err)
	}
	kind, kindErr := ParseAllocationInputKind(input.Action)
	if kindErr != nil {
		return 0, nil, false, fmt.Errorf("parse_allocation_kind: %w", kindErr)
	}
	values := map[string]float64{
		"expense.custo_fixo":           input.CustoFixo,
		"expense.conhecimento":         input.Conhecimento,
		"expense.prazeres":             input.Prazeres,
		"expense.metas":                input.Metas,
		"expense.liberdade_financeira": input.LiberdadeFinanceira,
	}
	return kind, values, input.MixedUnit, nil
}

func activateAllocationValues(ctx context.Context, budgets interfaces.BudgetPlanner, dist observability.Counter, state OnboardingState, kind allocationInputKind, values map[string]float64) (workflow.StepOutput[OnboardingState], error) {
	resolvedKind := DecideAllocationKind(kind, values, state.MonthlyBudgetCents)
	balance := DecideDistributionBalance(resolvedKind, values, state.MonthlyBudgetCents)
	if balance.Status != distributionBalanced {
		state.ReviewAwait = reviewAwaitDistribution
		if balance.Status == distributionOver {
			recordDistributionOutcome(ctx, dist, distributionOutcomeOver)
		} else {
			recordDistributionOutcome(ctx, dist, distributionOutcomeUnder)
		}
		return suspendStep(state, renderBalanceMessage(balance, values)), nil
	}
	bp, decErr := DecideAllocationsBP(resolvedKind, values, state.MonthlyBudgetCents)
	if decErr != nil {
		state.ReviewAwait = reviewAwaitDistribution
		if errors.Is(decErr, errAllocationOutOfTolerance) {
			if balance.Status == distributionOver {
				recordDistributionOutcome(ctx, dist, distributionOutcomeOver)
			} else {
				recordDistributionOutcome(ctx, dist, distributionOutcomeUnder)
			}
			return suspendStep(state, renderBalanceMessage(balance, values)), nil
		}
		return suspendStep(state, "Ops, não consegui aplicar essa distribuição: "+decErr.Error()), nil
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
	if balance.DeltaAbs > 0 {
		recordDistributionOutcome(ctx, dist, distributionOutcomeToleranceAbsorbed)
	} else {
		recordDistributionOutcome(ctx, dist, distributionOutcomeAcceptedValues)
	}
	return suspendStep(state, summaryPrompt(state, summaryPreview)), nil
}

func handleReviewAwaitDistribution(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, dist observability.Counter, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	resumeText := state.ResumeText
	state.ResumeText = ""
	preview, previewErr := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(defaultDistributionBP))
	if previewErr != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation: %w", previewErr))
	}
	intent, mixedUnit, intentErr := classifyDistributionIntent(ctx, a, resumeText)
	if intentErr != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: %w", intentErr))
	}
	if mixedUnit {
		state.ReviewAwait = reviewAwaitDistribution
		recordDistributionOutcome(ctx, dist, distributionOutcomeMixedUnit)
		return suspendStep(state, distributionMixedUnitPrompt), nil
	}
	switch intent {
	case distributionIntentAccept:
		state.Allocations = maps.Clone(defaultDistributionBP)
		if err := applyDraftBudget(ctx, budgets, state); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.budget_review: apply_draft_budget: %w", err))
		}
		summaryPreview, err := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(state.Allocations))
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.budget_review: suggest_allocation_current: %w", err))
		}
		state.ReviewAwait = reviewAwaitConfirm
		recordDistributionOutcome(ctx, dist, distributionOutcomeAcceptedDefault)
		return suspendStep(state, summaryPrompt(state, summaryPreview)), nil
	case distributionIntentPersonalize:
		state.ReviewAwait = reviewAwaitPersonalize
		recordDistributionOutcome(ctx, dist, distributionOutcomePersonalizeEntered)
		return suspendStep(state, personalizePrompt(state.MonthlyBudgetCents)), nil
	case distributionIntentValues:
		kind, values, _, extractErr := extractAllocationValues(ctx, a, resumeText)
		if extractErr != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.budget_review: %w", extractErr))
		}
		return activateAllocationValues(ctx, budgets, dist, state, kind, values)
	default:
		state.ReviewAwait = reviewAwaitDistribution
		return suspendStep(state, methodologyReprompt("não entendi sua resposta.", preview)), nil
	}
}

func handleReviewAwaitPersonalize(ctx context.Context, a agent.Agent, budgets interfaces.BudgetPlanner, dist observability.Counter, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	resumeText := state.ResumeText
	state.ResumeText = ""
	kind, values, mixedUnit, extractErr := extractAllocationValues(ctx, a, resumeText)
	if extractErr != nil {
		return failStep(state, fmt.Errorf("agents.onboarding.budget_review: %w", extractErr))
	}
	if mixedUnit {
		state.ReviewAwait = reviewAwaitPersonalize
		recordDistributionOutcome(ctx, dist, distributionOutcomeMixedUnit)
		return suspendStep(state, distributionMixedUnitPrompt), nil
	}
	if kind == allocationInputConfirm && sumAllocationValues(values) <= 0 {
		state.ReviewAwait = reviewAwaitPersonalize
		return suspendStep(state, personalizeReprompt(state.MonthlyBudgetCents)), nil
	}
	out, err := activateAllocationValues(ctx, budgets, dist, state, kind, values)
	if err != nil {
		return out, err
	}
	if out.Status == workflow.StepStatusSuspended && out.State.ReviewAwait == reviewAwaitDistribution {
		out.State.ReviewAwait = reviewAwaitPersonalize
	}
	return out, nil
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

const recurrenceOutcomeMetric = "agents_onboarding_recurrence_total"

func newRecurrenceOutcomeCounter(o11y observability.Observability) observability.Counter {
	return o11y.Metrics().Counter(
		recurrenceOutcomeMetric,
		"Total de resultados do passo de recorrencia do onboarding",
		"1",
	)
}

func recordRecurrenceOutcome(ctx context.Context, rec observability.Counter, outcome string) {
	if rec == nil {
		return
	}
	rec.Add(ctx, 1, observability.String("outcome", outcome))
}

func BuildRecurrenceStep(a agent.Agent, budgets interfaces.BudgetPlanner, rec observability.Counter) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseRecurrence
			return suspendStep(state, conclusionRecurrencePrompt), nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: recurrenceDecisionSystemPrompt},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "recurrence_decision", Strict: true, Schema: recurrenceDecisionSchema},
		})
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: parse: %w", err))
		}
		var extract recurrenceExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: unmarshal: %w", err))
		}
		intent, _ := ParseRecurrenceIntentKind(extract.Intent)
		decision := DecideRecurrence(intent, extract.HasMonths, extract.Months)
		switch decision.Outcome {
		case recurrenceOutcomeInvalid:
			recordRecurrenceOutcome(ctx, rec, decision.Outcome.String())
			return suspendStep(state, recurrenceInvalidReprompt), nil
		case recurrenceOutcomeAmbiguous:
			recordRecurrenceOutcome(ctx, rec, decision.Outcome.String())
			return suspendStep(state, conclusionRecurrencePrompt), nil
		case recurrenceOutcomeNone:
			state.RecurrenceConfirmation = recurrenceConfirmationNone
			recordRecurrenceOutcome(ctx, rec, decision.Outcome.String())
			return completeStep(state), nil
		case recurrenceOutcomeDefault, recurrenceOutcomeSpecific:
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.recurrence: parse_user_id: %w", err))
			}
			loc := competenceLocation(time.LoadLocation("America/Sao_Paulo"))
			competence := time.Now().In(loc).Format("2006-01")
			if err := budgets.CreateRecurrence(ctx, userUUID, competence, decision.Months); err != nil {
				return failStep(state, fmt.Errorf("agents.onboarding.recurrence: create_recurrence: %w", err))
			}
			state.Recurrence = true
			state.RecurrenceMonths = decision.Months
			state.RecurrenceConfirmation = recurrenceConfirmationFor(decision.Months)
			recordRecurrenceOutcome(ctx, rec, decision.Outcome.String())
			return completeStep(state), nil
		default:
			return failStep(state, fmt.Errorf("agents.onboarding.recurrence: unknown_outcome: %d", decision.Outcome))
		}
	}
}

func conclusionWorkingMemoryContent(state OnboardingState) string {
	var b strings.Builder
	if state.TreatmentName != "" {
		fmt.Fprintf(&b, "%s\n\n%s\n\n", treatmentNameWMSectionHeading, state.TreatmentName)
	}
	fmt.Fprintf(&b, "## Objetivo Financeiro\n\n%s", state.Goal)
	return b.String()
}

func BuildConclusionStep(
	workingMem memory.WorkingMemory,
	budgets interfaces.BudgetPlanner,
	cards interfaces.CardManager,
) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseConclusion
		if err := workingMem.Upsert(ctx, state.UserID, conclusionWorkingMemoryContent(state)); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: upsert_wm: %w", err))
		}
		metadata := map[string]any{"objetivo_financeiro": state.Goal}
		if state.GoalValueCents > 0 {
			metadata["objetivo_financeiro_valor_centavos"] = state.GoalValueCents
		}
		if state.TreatmentName != "" {
			metadata["nome_tratamento"] = state.TreatmentName
		}
		if err := workingMem.UpsertMetadata(ctx, state.UserID, metadata); err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: upsert_metadata: %w", err))
		}
		items, err := budgets.SuggestAllocation(ctx, state.MonthlyBudgetCents, allocationBPList(state.Allocations))
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: suggest_allocation: %w", err))
		}
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: parse_user_id: %w", err))
		}
		userCards, err := cards.ListCards(ctx, userUUID)
		if err != nil {
			return failStep(state, fmt.Errorf("agents.onboarding.conclusion: list_cards: %w", err))
		}
		state.FinalMessage = conclusionSummaryMessage(state, items, userCards)
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
	o11y observability.Observability,
) workflow.Definition[OnboardingState] {
	wrap := func(fn func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error)) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		return wrapStepWithMessages(fn, threads, messages)
	}
	var dist observability.Counter
	var mb observability.Counter
	var rec observability.Counter
	var tn observability.Counter
	if o11y != nil {
		dist = newDistributionOutcomeCounter(o11y)
		mb = newMonthlyBudgetOutcomeCounter(o11y)
		rec = newRecurrenceOutcomeCounter(o11y)
		tn = newTreatmentNameOutcomeCounter(o11y)
	}
	return workflow.Definition[OnboardingState]{
		ID: OnboardingWorkflowID,
		Root: workflow.Sequence("root",
			workflow.NewStepFunc(stepWelcomeID, wrap(BuildWelcomeStep(a, tn))),
			workflow.NewStepFunc(stepGoalID, wrap(BuildGoalStep(a))),
			workflow.NewStepFunc(stepMonthlyBudgetID, wrap(BuildMonthlyBudgetStep(a, mb))),
			workflow.NewStepFunc(stepBudgetReviewID, wrap(BuildBudgetReviewStep(a, budgets, dist))),
			workflow.NewStepFunc(stepActivationID, wrap(BuildActivationStep(budgets))),
			workflow.NewStepFunc(stepRecurrenceID, wrap(BuildRecurrenceStep(a, budgets, rec))),
			workflow.NewStepFunc(stepCardsID, wrap(BuildCardsStep(a, cards))),
			workflow.NewStepFunc(stepConclusionID, wrap(BuildConclusionStep(workingMem, budgets, cards))),
		),
		Durable:     true,
		MaxAttempts: 3,
	}
}

func BuildOnboardingReaper(store workflow.Store, o11y observability.Observability) *workflow.StaleSuspendedReaper {
	return workflow.NewStaleSuspendedReaper(store, OnboardingWorkflowID, OnboardingStaleAfter, OnboardingReaperBatch, o11y)
}
