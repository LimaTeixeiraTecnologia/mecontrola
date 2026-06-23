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
	s.Len(kinds, 21)
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
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
		intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindQueryCategory,
		intent.KindQueryGoal,
		intent.KindQueryCard,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindDeleteCard,
		intent.KindUnknown,
	}
	for _, k := range expected {
		s.True(set[k], "kind ausente: %v", k)
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

func (s *AgentWorkflowsSuite) TestNewWriteGuardIsNotNil() {
	a := &DailyLedgerAgent{}
	guard := a.newWriteGuard()
	s.NotNil(guard)
}
