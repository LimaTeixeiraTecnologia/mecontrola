package capability

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestNewCatalogAggregatesValidationErrors(t *testing.T) {
	t.Parallel()

	_, err := NewCatalog(
		CapabilitySpec{
			ID:         "expense",
			Kind:       intent.KindRecordExpense,
			WorkflowID: "transactions",
			Mode:       ModeRead,
		},
		CapabilitySpec{
			ID:         "expense",
			Kind:       intent.KindRecordExpense,
			WorkflowID: "",
			Mode:       0,
		},
	)

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCapabilityDuplicateID))
	require.True(t, errors.Is(err, ErrCapabilityDuplicateKind))
	require.True(t, errors.Is(err, ErrCapabilityModeInvalid))
	require.True(t, errors.Is(err, ErrCapabilityWorkflowIDEmpty))
	require.ErrorContains(t, err, `id="expense"`)
	require.ErrorContains(t, err, `kind="record_expense"`)
	require.ErrorContains(t, err, "field=workflow_id")
	require.ErrorContains(t, err, "field=mode")
}

func TestCatalogLookupHitAndMiss(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog(t)

	spec, ok := catalog.Lookup(intent.KindRecordExpense)
	require.True(t, ok)
	require.Equal(t, "expense", spec.ID)

	_, ok = catalog.Lookup(intent.KindQueryGoal)
	require.False(t, ok)
}

func TestCatalogListReturnsDefensiveCopyInStableOrder(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog(t)

	list := catalog.List()
	require.Len(t, list, 2)
	require.Equal(t, "expense", list[0].ID)
	require.Equal(t, "cards", list[1].ID)

	list[0].ID = "mutated"
	list[0].Channels[0] = "sms"

	second := catalog.List()
	require.Equal(t, "expense", second[0].ID)
	require.Equal(t, []string{"whatsapp"}, second[0].Channels)
}

func TestCatalogClassifyUsesFallbackForUnknownKind(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog(t)

	workflow, tool := catalog.Classify(intent.KindListCards)
	require.Equal(t, "cards", workflow)
	require.Equal(t, "list_cards", tool)

	workflow, tool = catalog.Classify(intent.KindQueryGoal)
	require.Equal(t, workflowConversational, workflow)
	require.Empty(t, tool)
}

func newTestCatalog(t *testing.T) *Catalog {
	t.Helper()

	catalog, err := NewCatalog(
		CapabilitySpec{
			ID:          "expense",
			Description: "record expense",
			Kind:        intent.KindRecordExpense,
			WorkflowID:  "transactions",
			ToolName:    "record_expense",
			Mode:        ModeWrite,
			Channels:    []string{"whatsapp"},
			MetricsKey:  "record_expense",
		},
		CapabilitySpec{
			ID:                   "cards",
			Description:          "list cards",
			Kind:                 intent.KindListCards,
			WorkflowID:           "cards",
			ToolName:             "list_cards",
			Mode:                 ModeRead,
			RequiresConfirmation: false,
			SupportsSuspend:      false,
			SupportsResume:       false,
			Channels:             []string{"whatsapp"},
			MetricsKey:           "list_cards",
		},
	)
	require.NoError(t, err)
	return catalog
}
