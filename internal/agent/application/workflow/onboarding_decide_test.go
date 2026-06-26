package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type OnboardingDecideSuite struct {
	suite.Suite
}

func TestOnboardingDecideSuite(t *testing.T) {
	suite.Run(t, new(OnboardingDecideSuite))
}

func (s *OnboardingDecideSuite) TestDecisionOutcomeStringAndValid() {
	s.Equal("advance", workflow.OutcomeAdvance.String())
	s.Equal("clarify", workflow.OutcomeClarify.String())
	s.Equal("deferred", workflow.OutcomeDeferred.String())
	s.Equal("confirm", workflow.OutcomeConfirm.String())
	s.Equal("cancel", workflow.OutcomeCancel.String())
	s.Equal("correct", workflow.OutcomeCorrect.String())
	s.Equal("reprompt", workflow.OutcomeReprompt.String())
	s.Equal("unknown", workflow.DecisionOutcome(0).String())
	s.True(workflow.OutcomeAdvance.IsValid())
	s.True(workflow.OutcomeReprompt.IsValid())
	s.False(workflow.DecisionOutcome(0).IsValid())
	s.False(workflow.DecisionOutcome(99).IsValid())
}

func (s *OnboardingDecideSuite) TestDecideObjective() {
	scenarios := []struct {
		name   string
		parsed workflow.ParsedObjective
		expect workflow.DecisionOutcome
	}{
		{name: "advance", parsed: workflow.ParsedObjective{Objective: "quitar dividas"}, expect: workflow.OutcomeAdvance},
		{name: "clarify vazio", parsed: workflow.ParsedObjective{}, expect: workflow.OutcomeClarify},
		{name: "clarify ambiguo", parsed: workflow.ParsedObjective{Objective: "nao sei", Ambiguous: true}, expect: workflow.OutcomeClarify},
		{name: "deferred comando diario", parsed: workflow.ParsedObjective{DailyCommand: true}, expect: workflow.OutcomeDeferred},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, workflow.DecideObjective(scenario.parsed))
		})
	}
}

func (s *OnboardingDecideSuite) TestDecideBudget() {
	scenarios := []struct {
		name   string
		parsed workflow.ParsedBudget
		expect workflow.DecisionOutcome
	}{
		{name: "advance", parsed: workflow.ParsedBudget{IncomeCents: 500000}, expect: workflow.OutcomeAdvance},
		{name: "clarify zero", parsed: workflow.ParsedBudget{IncomeCents: 0}, expect: workflow.OutcomeClarify},
		{name: "clarify negativo", parsed: workflow.ParsedBudget{IncomeCents: -1}, expect: workflow.OutcomeClarify},
		{name: "clarify ambiguo", parsed: workflow.ParsedBudget{Ambiguous: true}, expect: workflow.OutcomeClarify},
		{name: "deferred comando diario", parsed: workflow.ParsedBudget{DailyCommand: true, IncomeCents: 500000}, expect: workflow.OutcomeDeferred},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, workflow.DecideBudget(scenario.parsed))
		})
	}
}

func (s *OnboardingDecideSuite) TestDecideCards() {
	scenarios := []struct {
		name   string
		parsed workflow.ParsedCards
		expect workflow.DecisionOutcome
	}{
		{name: "advance", parsed: workflow.ParsedCards{Nickname: "Nubank", DueDay: 15}, expect: workflow.OutcomeAdvance},
		{name: "advance skip", parsed: workflow.ParsedCards{Skip: true}, expect: workflow.OutcomeAdvance},
		{name: "advance add another", parsed: workflow.ParsedCards{AddAnother: true}, expect: workflow.OutcomeAdvance},
		{name: "clarify vazio", parsed: workflow.ParsedCards{}, expect: workflow.OutcomeClarify},
		{name: "clarify dia invalido", parsed: workflow.ParsedCards{Nickname: "Nubank", DueDay: 32}, expect: workflow.OutcomeClarify},
		{name: "clarify apelido vazio", parsed: workflow.ParsedCards{DueDay: 15}, expect: workflow.OutcomeClarify},
		{name: "clarify ambiguo", parsed: workflow.ParsedCards{Ambiguous: true}, expect: workflow.OutcomeClarify},
		{name: "deferred comando diario", parsed: workflow.ParsedCards{DailyCommand: true}, expect: workflow.OutcomeDeferred},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, workflow.DecideCards(scenario.parsed))
		})
	}
}

func (s *OnboardingDecideSuite) TestDecideValues() {
	makeValues := func(fixed, knowledge, pleasures, goals, freedom int64) map[string]int64 {
		return map[string]int64{
			valueobjects.CategoryKindFixedCost.String():        fixed,
			valueobjects.CategoryKindKnowledge.String():        knowledge,
			valueobjects.CategoryKindPleasures.String():        pleasures,
			valueobjects.CategoryKindGoals.String():            goals,
			valueobjects.CategoryKindFinancialFreedom.String(): freedom,
		}
	}

	scenarios := []struct {
		name   string
		state  workflow.ValuesState
		expect workflow.DecisionOutcome
	}{
		{
			name:   "advance soma igual",
			state:  workflow.ValuesState{Values: makeValues(200000, 50000, 75000, 100000, 75000), IncomeCents: 500000},
			expect: workflow.OutcomeAdvance,
		},
		{
			name:   "clarify soma diferente",
			state:  workflow.ValuesState{Values: makeValues(300000, 50000, 75000, 100000, 75000), IncomeCents: 500000},
			expect: workflow.OutcomeClarify,
		},
		{
			name: "clarify menos de 5",
			state: func() workflow.ValuesState {
				v := makeValues(200000, 50000, 75000, 100000, 75000)
				delete(v, valueobjects.CategoryKindFinancialFreedom.String())
				return workflow.ValuesState{Values: v, IncomeCents: 425000}
			}(),
			expect: workflow.OutcomeClarify,
		},
		{
			name:   "clarify valor negativo",
			state:  workflow.ValuesState{Values: makeValues(-1, 50000, 75000, 100000, 75000), IncomeCents: 300000},
			expect: workflow.OutcomeClarify,
		},
		{
			name:   "clarify chaves arbitrarias",
			state:  workflow.ValuesState{Values: map[string]int64{"a": 100000, "b": 100000, "c": 100000, "d": 100000, "e": 100000}, IncomeCents: 500000},
			expect: workflow.OutcomeClarify,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, workflow.DecideValues(scenario.state))
		})
	}
}

func (s *OnboardingDecideSuite) TestDecideSummary() {
	scenarios := []struct {
		name   string
		parsed workflow.ParsedSummary
		expect workflow.DecisionOutcome
	}{
		{name: "confirm", parsed: workflow.ParsedSummary{Confirm: true}, expect: workflow.OutcomeConfirm},
		{name: "cancel", parsed: workflow.ParsedSummary{Cancel: true}, expect: workflow.OutcomeCancel},
		{name: "correct", parsed: workflow.ParsedSummary{Correct: true, Target: workflow.CorrectionTargetBudget, NewValue: "5000"}, expect: workflow.OutcomeCorrect},
		{name: "correct sem target", parsed: workflow.ParsedSummary{Correct: true}, expect: workflow.OutcomeClarify},
		{name: "reprompt", parsed: workflow.ParsedSummary{}, expect: workflow.OutcomeReprompt},
		{name: "clarify ambiguo", parsed: workflow.ParsedSummary{Ambiguous: true}, expect: workflow.OutcomeClarify},
		{name: "deferred comando diario", parsed: workflow.ParsedSummary{DailyCommand: true}, expect: workflow.OutcomeDeferred},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, workflow.DecideSummary(scenario.parsed))
		})
	}
}

func (s *OnboardingDecideSuite) TestBuildValuesState() {
	original := map[string]int64{"fixed_cost": 100}
	state := workflow.BuildValuesState(original, 200)
	original["fixed_cost"] = 999
	s.Equal(int64(100), state.Values["fixed_cost"])
	s.Equal(int64(200), state.IncomeCents)
}

func (s *OnboardingDecideSuite) TestCategorySlug() {
	s.Equal("fixed_cost", workflow.CategorySlug(valueobjects.CategoryKindFixedCost))
}
