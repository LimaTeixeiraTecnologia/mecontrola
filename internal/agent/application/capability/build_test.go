package capability

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestBuildCatalogDefinesCurrentUniqueKinds(t *testing.T) {
	t.Parallel()

	catalog, err := BuildCatalog()
	require.NoError(t, err)

	list := catalog.List()
	require.Len(t, list, 28)

	for _, spec := range list {
		require.Equal(t, []string{whatsAppChannel}, spec.Channels)
		if spec.Kind == intent.KindUnknown {
			require.Empty(t, spec.ToolName)
			require.Empty(t, spec.MetricsKey)
			require.Equal(t, workflowConversational, spec.WorkflowID)
			require.Equal(t, ModeRead, spec.Mode)
			continue
		}
		require.Equal(t, spec.Kind.String(), spec.ToolName)
		require.Equal(t, spec.ToolName, spec.MetricsKey)
	}
}

func TestBuildCatalogAppliesWorkflowDriftCorrections(t *testing.T) {
	t.Parallel()

	catalog, err := BuildCatalog()
	require.NoError(t, err)

	assertWorkflow(t, catalog, intent.KindQueryIncomeSummary, workflowTransactions)
	assertWorkflow(t, catalog, intent.KindBudgetRecurrence, workflowBudget)
	assertWorkflow(t, catalog, intent.KindDeleteTransactionByRef, workflowTransactions)
	assertWorkflow(t, catalog, intent.KindEditTransactionByRef, workflowTransactions)
}

func TestBuildCatalogMarksWritesAndDestructiveKinds(t *testing.T) {
	t.Parallel()

	catalog, err := BuildCatalog()
	require.NoError(t, err)

	recordExpense, ok := catalog.Lookup(intent.KindRecordExpense)
	require.True(t, ok)
	require.Equal(t, ModeWrite, recordExpense.Mode)
	require.False(t, recordExpense.RequiresConfirmation)
	require.True(t, recordExpense.SupportsSuspend)
	require.True(t, recordExpense.SupportsResume)

	configureBudget, ok := catalog.Lookup(intent.KindConfigureBudget)
	require.True(t, ok)
	require.Equal(t, ModeWrite, configureBudget.Mode)
	require.True(t, configureBudget.SupportsSuspend)
	require.True(t, configureBudget.SupportsResume)

	deleteByRef, ok := catalog.Lookup(intent.KindDeleteTransactionByRef)
	require.True(t, ok)
	require.Equal(t, ModeWrite, deleteByRef.Mode)
	require.True(t, deleteByRef.RequiresConfirmation)
	require.True(t, deleteByRef.SupportsSuspend)
	require.True(t, deleteByRef.SupportsResume)

	monthlySummary, ok := catalog.Lookup(intent.KindMonthlySummary)
	require.True(t, ok)
	require.Equal(t, ModeRead, monthlySummary.Mode)
	require.False(t, monthlySummary.RequiresConfirmation)
	require.False(t, monthlySummary.SupportsSuspend)
	require.False(t, monthlySummary.SupportsResume)
}

func assertWorkflow(t *testing.T, catalog *Catalog, kind intent.Kind, workflowID string) {
	t.Helper()

	spec, ok := catalog.Lookup(kind)
	require.True(t, ok)
	require.Equal(t, workflowID, spec.WorkflowID)
}
