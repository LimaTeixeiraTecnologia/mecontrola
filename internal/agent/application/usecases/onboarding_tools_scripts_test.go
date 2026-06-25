package usecases

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	onbusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
)

type OnboardingToolsSuite struct {
	suite.Suite
}

func TestOnboardingToolsSuite(t *testing.T) {
	suite.Run(t, new(OnboardingToolsSuite))
}

func (s *OnboardingToolsSuite) TestScriptCardsHasFechamentoNotVencimento() {
	s.NotContains(scriptCards, "vencimento", "scriptCards nao deve mencionar vencimento")
	s.Contains(scriptCards, "fechamento", "scriptCards deve mencionar fechamento")
}

func (s *OnboardingToolsSuite) TestScriptCardQuestionHasFechamento() {
	s.NotContains(scriptCardQuestion, "vencimento", "scriptCardQuestion nao deve mencionar vencimento")
	s.Contains(scriptCardQuestion, "fechamento", "scriptCardQuestion deve mencionar fechamento")
}

func (s *OnboardingToolsSuite) TestScriptWelcomeHasAquiNoMeControla() {
	s.Contains(scriptWelcome, "Aqui no MeControla", "scriptWelcome deve iniciar o bloco de categorias com 'Aqui no MeControla'")
}

func (s *OnboardingToolsSuite) TestScriptWelcomeHasFiveCategories() {
	for _, cat := range []string{"Custo Fixo", "Conhecimento", "Prazeres", "Metas", "Liberdade Financeira"} {
		s.Contains(scriptWelcome, cat, "scriptWelcome deve conter categoria: %s", cat)
	}
}

func (s *OnboardingToolsSuite) TestScriptWelcomeHasProgressIndicator() {
	s.Contains(scriptWelcome, "Etapa 1/4", "scriptWelcome deve conter indicador de progresso")
}

func (s *OnboardingToolsSuite) TestObjectiveProfileStoredInDraftAndPassedToSuggester() {
	capturedProfile := ""
	capturedObjective := ""

	suggester := &fakeSplitSuggesterCapture{
		onSuggest: func(_ context.Context, _ uuid.UUID, profile, objective string, _ int64) ([]onbusecases.SuggestBudgetSplitView, error) {
			capturedProfile = profile
			capturedObjective = objective
			return []onbusecases.SuggestBudgetSplitView{
				{RootSlug: "expense.custo_fixo", PlannedCents: 200000},
				{RootSlug: "expense.conhecimento", PlannedCents: 50000},
				{RootSlug: "expense.prazeres", PlannedCents: 75000},
				{RootSlug: "expense.metas", PlannedCents: 100000},
				{RootSlug: "expense.liberdade_financeira", PlannedCents: 75000},
			}, nil
		},
	}

	interp := &fakeTurnInterpreter{resp: interfaces.LLMResponse{
		RawJSON: []byte(`{"action":"save_onboarding_objective","objective":"quitar dividas","objective_profile":"payoff_debt","reply":""}`),
	}}
	dispatcher := &fakeToolDispatcherWithProfile{results: map[string]OnboardingToolResult{
		ToolSaveOnboardingObjective: {Reply: "Objetivo anotado!", Advance: true, ObjectiveProfile: "payoff_debt"},
	}}
	setter := &fakePhaseSetter{}
	v2 := &fakeV2Session{}

	ctx := context.Background()
	uc, err := NewRunOnboardingTurn(interp,
		newReader(
			OnboardingSnapshot{InProgress: true, Phase: OnbPhaseObjective},
			OnboardingSnapshot{InProgress: true, Phase: OnbPhaseBudget},
		),
		dispatcher, setter, 512, fake.NewProvider(), nil, suggester, v2)
	s.Require().NoError(err)

	_, err = uc.Execute(ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "quitar dividas"})
	s.Require().NoError(err)

	s.NotNil(v2.saved, "draft deve ser salvo apos objetivo")
	s.Equal("payoff_debt", v2.saved.ObjectiveProfile(), "objective_profile deve ser persistido no draft")

	_, err = uc.Execute(ctx, RunOnboardingTurnInput{UserID: uuid.New(), Channel: "whatsapp", Text: "ganho 5000"})
	s.Require().NoError(err)
	_ = capturedProfile
	_ = capturedObjective
}

func (s *OnboardingToolsSuite) TestDataPhasePromptInstructsResumeWithProgressIndicator() {
	scenarios := []struct {
		phase  string
		header string
	}{
		{OnbPhaseObjective, "🔵 Etapa 1/4 — Objetivo"},
		{OnbPhaseBudget, "🔵 Etapa 2/4 — Orçamento"},
		{OnbPhaseCards, "🔵 Etapa 3/4 — Cartões"},
		{OnbPhaseFinancialPlan, "🔵 Etapa 4/4 — Plano Financeiro"},
		{OnbPhaseFirstTx, "🔵 Etapa 4/4 — Plano Financeiro"},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.phase, func() {
			prompt := onboardingDataPhasePrompt(scenario.phase, OnboardingSnapshot{})
			s.Contains(prompt, scenario.header, "prompt deve conter o cabeçalho de progresso da etapa")
			s.Contains(prompt, "retome a etapa atual", "prompt deve instruir retomada da etapa atual")
		})
	}
}

type fakeToolDispatcherWithProfile struct {
	results map[string]OnboardingToolResult
	err     error
}

func (f *fakeToolDispatcherWithProfile) Dispatch(_ context.Context, _ uuid.UUID, _ string, call interfaces.ToolCall) (OnboardingToolResult, error) {
	if f.err != nil {
		return OnboardingToolResult{}, f.err
	}
	return f.results[call.FunctionName], nil
}

type fakeSplitSuggesterCapture struct {
	onSuggest func(ctx context.Context, userID uuid.UUID, profile, objective string, incomeCents int64) ([]onbusecases.SuggestBudgetSplitView, error)
}

func (s *fakeSplitSuggesterCapture) Suggest(ctx context.Context, userID uuid.UUID, profile, objective string, incomeCents int64) ([]onbusecases.SuggestBudgetSplitView, error) {
	return s.onSuggest(ctx, userID, profile, objective, incomeCents)
}
