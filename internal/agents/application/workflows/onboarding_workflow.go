package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	stepWelcomeID      = "step-welcome"
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

func DecideDistribution(allocs map[string]int) error {
	var errs []error
	total := 0
	for _, slug := range canonicalSlugs {
		v, ok := allocs[slug]
		if !ok {
			errs = append(errs, fmt.Errorf("distribution: categoria ausente: %s", slug))
			continue
		}
		total += v
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if total != 100 {
		return fmt.Errorf("distribution: soma deve ser 100, recebido %d", total)
	}
	return nil
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

type distributionExtract struct {
	CustoFixo           int `json:"custo_fixo"`
	Conhecimento        int `json:"conhecimento"`
	Prazeres            int `json:"prazeres"`
	Metas               int `json:"metas"`
	LiberdadeFinanceira int `json:"liberdade_financeira"`
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

var distributionSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"custo_fixo":           map[string]any{"type": "integer"},
		"conhecimento":         map[string]any{"type": "integer"},
		"prazeres":             map[string]any{"type": "integer"},
		"metas":                map[string]any{"type": "integer"},
		"liberdade_financeira": map[string]any{"type": "integer"},
	},
	"required":             []any{"custo_fixo", "conhecimento", "prazeres", "metas", "liberdade_financeira"},
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

func agentStream(ctx context.Context, a agent.Agent, prompt string) (string, error) {
	stream, err := a.Stream(ctx, agent.Request{
		AgentID:  a.ID(),
		Messages: []llm.Message{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	for range stream.Deltas() {
	}
	result, err := stream.Result(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

func BuildWelcomeStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseWelcome
			msg, err := agentStream(ctx, a, "Gere uma mensagem de boas-vindas calorosa para um novo usuario do MeControla, aplicativo de controle financeiro pessoal. Seja breve, animador e convide-o a comecar sua jornada financeira.")
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.welcome: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
		}
		state.ResumeText = ""
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func repromptInput(ctx context.Context, a agent.Agent, state OnboardingState, instruction, fallback string) workflow.StepOutput[OnboardingState] {
	msg, err := agentStream(ctx, a, instruction)
	if err != nil || strings.TrimSpace(msg) == "" {
		msg = fallback
	}
	return workflow.StepOutput[OnboardingState]{
		State:   state,
		Status:  workflow.StepStatusSuspended,
		Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
	}
}

func BuildGoalStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseGoal
			msg, err := agentStream(ctx, a, "Pergunte ao usuario qual e o seu principal objetivo financeiro para este mes. Seja direto e encorajador. De exemplos curtos.")
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.goal: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
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
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.goal: parse: %w", err)
		}
		var extract goalExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.goal: unmarshal: %w", err)
		}
		goal, err := DecideGoal(extract.Goal)
		if err != nil {
			return repromptInput(ctx, a, state,
				"O usuario nao informou um objetivo financeiro claro. Peca gentilmente que ele diga qual e o principal objetivo financeiro dele para este mes, com um exemplo curto.",
				"Não consegui identificar seu objetivo. Qual é o seu principal objetivo financeiro para este mês? Por exemplo: economizar R$ 500, quitar uma dívida ou montar uma reserva.",
			), nil
		}
		state.Goal = goal
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildIncomeStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseMonthlyIncome
			msg, err := agentStream(ctx, a, "Pergunte ao usuario qual e a sua renda mensal liquida em reais. Seja direto e amigavel.")
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.income: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
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
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.income: parse: %w", err)
		}
		var extract incomeExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.income: unmarshal: %w", err)
		}
		cents, err := DecideIncomeCents(extract.AmountBRL)
		if err != nil {
			return repromptInput(ctx, a, state,
				"O valor de renda informado nao foi valido. Peca novamente a renda mensal liquida em reais, com um exemplo curto.",
				"Não consegui identificar o valor. Qual é a sua renda mensal líquida em reais? Por exemplo: R$ 3.500,00.",
			), nil
		}
		state.IncomeCents = cents
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildCardsStep(a agent.Agent, cards interfaces.CardManager) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseCards
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.cards: parse_user_id: %w", err)
			}
			existingCards, err := cards.ListCards(ctx, userUUID)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.cards: list_cards: %w", err)
			}
			prompt := fmt.Sprintf(
				"O usuario tem %d cartao(oes) cadastrado(s). Pergunte se ele deseja adicionar um cartao de credito agora. Se sim, peca o apelido do cartao, o banco emissor e o dia de vencimento da fatura (numero entre 1 e 31).",
				len(existingCards),
			)
			msg, err := agentStream(ctx, a, prompt)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.cards: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
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
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.cards: parse: %w", err)
		}
		var extract cardExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.cards: unmarshal: %w", err)
		}
		if extract.WantsCard {
			if err := DecideCardEntry(extract.Nickname, extract.Bank, extract.DueDay); err != nil {
				return repromptInput(ctx, a, state,
					"O usuario quer adicionar um cartao mas nao informou o apelido, o banco e/ou o dia de vencimento validos. Peca o apelido do cartao, o banco emissor e o dia de vencimento da fatura (numero entre 1 e 31), com um exemplo curto.",
					"Para adicionar o cartão, me diga o apelido, o banco emissor e o dia de vencimento da fatura (um número entre 1 e 31). Por exemplo: \"Nubank, vencimento dia 10\". Se preferir não adicionar agora, responda \"não\".",
				), nil
			}
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.cards: parse_user_id_create: %w", err)
			}
			if _, err := cards.CreateCard(ctx, interfaces.NewCard{
				UserID:   userUUID,
				Nickname: extract.Nickname,
				Bank:     extract.Bank,
				DueDay:   extract.DueDay,
			}); err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.cards: create_card: %w", err)
			}
			state.CardNickname = extract.Nickname
			state.CardDueDay = extract.DueDay
		}
		state.CardsDone = true
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildMethodologyStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseMethodology
			msg, err := agentStream(ctx, a, "Explique brevemente a metodologia de distribuicao de orcamento do MeControla com 5 categorias: Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira. Depois peca ao usuario para definir o percentual de cada categoria (a soma deve ser 100%). Seja didatico.")
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.methodology: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
		}
		resumeText := state.ResumeText
		state.ResumeText = ""
		extracted, err := a.Execute(ctx, agent.Request{
			Messages: []llm.Message{
				{Role: "system", Content: "Extraia do texto do usuario os percentuais para cada categoria: custo_fixo, conhecimento, prazeres, metas, liberdade_financeira. Todos devem ser inteiros e somar 100."},
				{Role: "user", Content: resumeText},
			},
			Schema: &llm.Schema{Name: "distribution_extract", Strict: true, Schema: distributionSchema},
		})
		if err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.methodology: parse: %w", err)
		}
		var extract distributionExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.methodology: unmarshal: %w", err)
		}
		allocs := map[string]int{
			"expense.custo_fixo":           extract.CustoFixo,
			"expense.conhecimento":         extract.Conhecimento,
			"expense.prazeres":             extract.Prazeres,
			"expense.metas":                extract.Metas,
			"expense.liberdade_financeira": extract.LiberdadeFinanceira,
		}
		if distErr := DecideDistribution(allocs); distErr != nil {
			reaskMsg, streamErr := agentStream(ctx, a,
				fmt.Sprintf("O usuario informou percentuais invalidos para a distribuicao do orcamento. Problema: %s. Peca que ele informe novamente os percentuais para as 5 categorias, garantindo que a soma seja 100%%.", distErr.Error()))
			if streamErr != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.methodology: reask_stream: %w", streamErr)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: reaskMsg},
			}, nil
		}
		state.Allocations = allocs
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildDistributionStep(budgets interfaces.BudgetPlanner) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		state.Phase = PhaseDistribution
		userUUID, err := uuid.Parse(state.UserID)
		if err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.distribution: parse_user_id: %w", err)
		}
		loc, _ := time.LoadLocation("America/Sao_Paulo")
		competence := time.Now().In(loc).Format("2006-01")
		allocations := make([]interfaces.AllocationDraft, 0, len(state.Allocations))
		for slug, pct := range state.Allocations {
			allocations = append(allocations, interfaces.AllocationDraft{
				RootSlug:    slug,
				BasisPoints: pct * 100,
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
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.distribution: create_budget: %w", err)
			}
		case sumErr != nil:
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.distribution: get_monthly_summary: %w", sumErr)
		case summary.State == "active":
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
		default:
			if err := budgets.DeleteDraftBudget(ctx, userUUID, competence); err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.distribution: delete_draft_budget: %w", err)
			}
			if _, err := budgets.CreateBudget(ctx, draft); err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.distribution: create_budget: %w", err)
			}
		}
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildSummaryStep(a agent.Agent) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseSummary
			allocJSON, _ := json.Marshal(state.Allocations)
			prompt := fmt.Sprintf(
				"Gere um resumo do onboarding: objetivo='%s', renda=R$%.2f, cartao='%s' dia %d, distribuicao=%s. Peca confirmacao para ativar o orcamento.",
				state.Goal, float64(state.IncomeCents)/100.0, state.CardNickname, state.CardDueDay, string(allocJSON),
			)
			msg, err := agentStream(ctx, a, prompt)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.summary: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
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
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.summary: parse: %w", err)
		}
		var extract yesNoExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.summary: unmarshal: %w", err)
		}
		if !extract.Confirmed {
			reaskMsg, streamErr := agentStream(ctx, a, "O usuario nao confirmou o resumo. Pergunte se ele deseja revisar alguma informacao ou confirmar os dados para ativar o orcamento.")
			if streamErr != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.summary: reask_stream: %w", streamErr)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: reaskMsg},
			}, nil
		}
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
	}
}

func BuildConclusionStep(a agent.Agent, budgets interfaces.BudgetPlanner, workingMem memory.WorkingMemory) func(context.Context, OnboardingState) (workflow.StepOutput[OnboardingState], error) {
	return func(ctx context.Context, state OnboardingState) (workflow.StepOutput[OnboardingState], error) {
		if state.ResumeText == "" {
			state.Phase = PhaseConclusion
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.conclusion: parse_user_id: %w", err)
			}
			loc, _ := time.LoadLocation("America/Sao_Paulo")
			competence := time.Now().In(loc).Format("2006-01")
			if err := budgets.ActivateBudget(ctx, userUUID, competence); err != nil && !errors.Is(err, interfaces.ErrBudgetAlreadyActive) {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.conclusion: activate_budget: %w", err)
			}
			msg, err := agentStream(ctx, a, "Informe ao usuario que o orcamento foi ativado com sucesso. Pergunte se ele deseja que o orcamento se repita automaticamente por 12 meses (responda sim ou nao).")
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.conclusion: stream: %w", err)
			}
			return workflow.StepOutput[OnboardingState]{
				State:   state,
				Status:  workflow.StepStatusSuspended,
				Suspend: &workflow.Suspension{Reason: workflow.SuspendAwaitingInput, Prompt: msg},
			}, nil
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
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.conclusion: parse: %w", err)
		}
		var extract yesNoExtract
		if err := json.Unmarshal(extracted.RawJSON, &extract); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.conclusion: unmarshal: %w", err)
		}
		if extract.Confirmed {
			userUUID, err := uuid.Parse(state.UserID)
			if err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.conclusion: parse_user_id_recurrence: %w", err)
			}
			loc, _ := time.LoadLocation("America/Sao_Paulo")
			competence := time.Now().In(loc).Format("2006-01")
			if err := budgets.CreateRecurrence(ctx, userUUID, competence, 12); err != nil {
				return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
					fmt.Errorf("agents.onboarding.conclusion: create_recurrence: %w", err)
			}
			state.Recurrence = true
		}
		if err := workingMem.Upsert(ctx, state.UserID, "## Objetivo Financeiro\n\n"+state.Goal); err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.conclusion: upsert_wm: %w", err)
		}
		finalMsg, err := agentStream(ctx, a, fmt.Sprintf(
			"Parabenize o usuario por completar o onboarding do MeControla! Objetivo registrado: '%s'. De exemplos praticos de como comecara a registrar gastos e receitas no app. Seja animador.",
			state.Goal,
		))
		if err != nil {
			return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusFailed},
				fmt.Errorf("agents.onboarding.conclusion: final_stream: %w", err)
		}
		state.FinalMessage = finalMsg
		return workflow.StepOutput[OnboardingState]{State: state, Status: workflow.StepStatusCompleted}, nil
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
			workflow.NewStepFunc(stepWelcomeID, wrap(BuildWelcomeStep(a))),
			workflow.NewStepFunc(stepGoalID, wrap(BuildGoalStep(a))),
			workflow.NewStepFunc(stepIncomeID, wrap(BuildIncomeStep(a))),
			workflow.NewStepFunc(stepCardsID, wrap(BuildCardsStep(a, cards))),
			workflow.NewStepFunc(stepMethodologyID, wrap(BuildMethodologyStep(a))),
			workflow.NewStepFunc(stepDistributionID, BuildDistributionStep(budgets)),
			workflow.NewStepFunc(stepSummaryID, wrap(BuildSummaryStep(a))),
			workflow.NewStepFunc(stepConclusionID, wrap(BuildConclusionStep(a, budgets, workingMem))),
		),
		Durable:     true,
		MaxAttempts: 3,
	}
}
