package postdeploy

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
)

type RegressionContractSuite struct {
	suite.Suite
}

func TestRegressionContractSuite(t *testing.T) {
	suite.Run(t, new(RegressionContractSuite))
}

func (s *RegressionContractSuite) TestRegisteredWorkflowIDsMatchProductionConstants() {
	realIDs := []string{
		workflows.BudgetCreationWorkflowID,
		workflows.CardCreateConfirmWorkflowID,
		workflows.DestructiveConfirmWorkflowID,
		workflows.OnboardingWorkflowID,
		workflows.PendingEntryWorkflowID,
	}

	s.Empty(MissingFrom(RegisteredWorkflows, realIDs), "workflow removido/renomeado sem história própria (RF-54)")
	s.Empty(MissingFrom(realIDs, RegisteredWorkflows), "novo workflow em produção não coberto pelo inventário de regressão")
}

func (s *RegressionContractSuite) TestMissingFromDetectsRemoval() {
	type args struct {
		expected []string
		actual   []string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(missing []string)
	}{
		{
			name: "deve detectar item removido",
			args: args{expected: []string{"a", "b", "c"}, actual: []string{"a", "c"}},
			expect: func(missing []string) {
				s.Equal([]string{"b"}, missing)
			},
		},
		{
			name: "não deve reportar nada quando conjuntos idênticos",
			args: args{expected: []string{"a", "b"}, actual: []string{"a", "b"}},
			expect: func(missing []string) {
				s.Empty(missing)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			missing := MissingFrom(scenario.args.expected, scenario.args.actual)
			scenario.expect(missing)
		})
	}
}

func (s *RegressionContractSuite) TestExtraInDetectsUndocumentedAddition() {
	type args struct {
		expected []string
		actual   []string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(extra []string)
	}{
		{
			name: "deve detectar item novo não documentado",
			args: args{expected: []string{"a", "b"}, actual: []string{"a", "b", "c"}},
			expect: func(extra []string) {
				s.Equal([]string{"c"}, extra)
			},
		},
		{
			name: "não deve reportar nada quando conjuntos idênticos",
			args: args{expected: []string{"a", "b"}, actual: []string{"a", "b"}},
			expect: func(extra []string) {
				s.Empty(extra)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			extra := ExtraIn(scenario.args.expected, scenario.args.actual)
			scenario.expect(extra)
		})
	}
}

func (s *RegressionContractSuite) TestRegisteredToolsCountMatchesInventory() {
	s.Len(RegisteredTools, 25, "inventário de tools deve refletir exatamente as tools ativas em module.go (RF-54)")
}

func (s *RegressionContractSuite) TestRegisteredScorersCountMatchesInventory() {
	s.Len(RegisteredScorers, 11, "3 scorers de continuidade (RF-29) + 9 comportamentais (RF-30)")
}

func (s *RegressionContractSuite) TestRegisteredScorerIDsMatchProductionConstructors() {
	realIDs := []string{
		scorers.NewFinancialToolCallAccuracyScorer().ID(),
		scorers.NewFinancialCompletenessScorer().ID(),
		scorers.NewCategorizationScorer(nil).ID(),
		scorers.NewNoEmptyAnswerScorer().ID(),
		scorers.NewWhatsAppFormatScorer().ID(),
		scorers.NewNoInternalTermsScorer().ID(),
		scorers.NewVerbatimRequiredScorer().ID(),
		scorers.NewNoDuplicateWriteScorer().ID(),
		scorers.NewNoHallucinationScorer().ID(),
		scorers.NewRequiredArgsScorer().ID(),
		scorers.NewMonthReferenceCorrectnessScorer().ID(),
	}

	s.Empty(MissingFrom(RegisteredScorers, realIDs), "scorer removido/renomeado sem história própria (RF-54)")
	s.Empty(MissingFrom(realIDs, RegisteredScorers), "novo scorer em produção não coberto pelo inventário de regressão")
	s.ElementsMatch(RegisteredScorers, realIDs, "os 11 scorers de BuildMeControlaScorers (RF-29+RF-30) devem bater 1:1 com o inventário")
}

func (s *RegressionContractSuite) TestCoveredExistingFlowsMatchesPRDEnumeration() {
	s.Len(CoveredExistingFlows, 18, "RF-56 enumera 18 fluxos existentes que devem continuar cobertos")
}
