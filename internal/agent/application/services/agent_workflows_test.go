package services

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type AgentWorkflowsSuite struct {
	suite.Suite
}

func TestAgentWorkflowsSuite(t *testing.T) {
	suite.Run(t, new(AgentWorkflowsSuite))
}

func (s *AgentWorkflowsSuite) TestRoutableKindsHasNoDuplicates() {
	kinds := routableKinds()
	seen := make(map[intent.Kind]bool)
	for _, k := range kinds {
		s.False(seen[k], "kind duplicado: %v", k)
		seen[k] = true
	}
	s.Len(kinds, 20)
}

func (s *AgentWorkflowsSuite) TestRoutableKindsContainsExpectedKinds() {
	kinds := routableKinds()
	set := make(map[intent.Kind]bool, len(kinds))
	for _, k := range kinds {
		set[k] = true
	}

	expected := []intent.Kind{
		intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
		intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindQueryCategory,
		intent.KindQueryGoal,
		intent.KindQueryCard,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindBudgetRecurrence,
		intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindQueryIncomeSummary,
		intent.KindUnknown,
	}
	for _, k := range expected {
		s.True(set[k], "kind ausente: %v", k)
	}
}

func (s *AgentWorkflowsSuite) TestDestructiveKindsNotInRoutableKinds() {
	kinds := routableKinds()
	set := make(map[intent.Kind]bool, len(kinds))
	for _, k := range kinds {
		set[k] = true
	}

	destructive := []intent.Kind{
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindDeleteCard,
	}
	for _, k := range destructive {
		s.False(set[k], "kind destrutivo nao deve estar no registry: %v", k)
	}
}

func (s *AgentWorkflowsSuite) TestBuildRegistryCreates4Workflows() {
	a := &DailyLedgerAgent{}
	registry, err := a.buildRegistry()
	s.Require().NoError(err)
	s.Require().NotNil(registry)
	wfs := registry.Workflows()
	s.Len(wfs, 4)
	ids := make(map[string]bool, len(wfs))
	for _, wf := range wfs {
		ids[wf.ID()] = true
	}
	s.True(ids["transactions"])
	s.True(ids["budget"])
	s.True(ids["cards"])
	s.True(ids["conversational"])
}

func (s *AgentWorkflowsSuite) TestBuildRegistryResolvesAllRoutableKinds() {
	a := &DailyLedgerAgent{}
	registry, err := a.buildRegistry()
	s.Require().NoError(err)
	for _, k := range routableKinds() {
		_, ok := registry.Resolve(k)
		s.True(ok, "kind nao resolvido no registry: %v", k)
	}
}

func (s *AgentWorkflowsSuite) TestWarnBindingsCoversAllNonConversationalKinds() {
	routable := routableKinds()
	nonConversational := make([]intent.Kind, 0, len(routable))
	for _, k := range routable {
		if k != intent.KindUnknown {
			nonConversational = append(nonConversational, k)
		}
	}

	trackedKinds := warnMissingToolBindingsKinds()
	set := make(map[intent.Kind]bool, len(trackedKinds))
	for _, k := range trackedKinds {
		set[k] = true
	}

	for _, k := range nonConversational {
		s.True(set[k], "warnMissingToolBindings nao cobre kind roteavel: %v", k)
	}
}
