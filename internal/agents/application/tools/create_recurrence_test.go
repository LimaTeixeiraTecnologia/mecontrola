package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	imocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var testRecurrenceID = uuid.MustParse("00000000-0000-0000-0000-000000000010")

func TestBuildCreateRecurrenceToolSuccess(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	writer := mocks.NewIdempotentWriter(t)

	recurrences.EXPECT().
		CreateRecurrence(mock.Anything, mock.AnythingOfType("interfaces.RawRecurrence")).
		Return(interfaces.EntryRef{ID: testRecurrenceID, Kind: interfaces.EntryKindRecurringTemplate}, nil).Once()

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-recur", 1, "create_recurrence", "recurring_template", mock.AnythingOfType("usecases.WriteFn")).
		RunAndReturn(func(ctx context.Context, userID uuid.UUID, wamid string, itemSeq int, operation, resourceKind string, write usecases.WriteFn) (usecases.IdempotentWriteResult, error) {
			id, _, err := write(ctx)
			if err != nil {
				return usecases.IdempotentWriteResult{}, err
			}
			return usecases.IdempotentWriteResult{ResourceID: id, Outcome: agent.ToolOutcomeRouted}, nil
		}).Once()

	handle := BuildCreateRecurrenceTool(recurrences, writer)
	assert.Equal(t, "create_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(CreateRecurrenceInput{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    testCategoryID.String(),
		Frequency:     "monthly",
		DayOfMonth:    5,
	})
	out, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.NoError(t, err)

	var result CreateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, testRecurrenceID.String(), result.ResourceID)
	assert.Equal(t, "recurring_template", result.Kind)
	assert.False(t, result.IsReplay)
	assert.Equal(t, agent.ToolOutcomeRouted.String(), result.Outcome)
}

func TestBuildCreateRecurrenceToolReplay(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	writer := mocks.NewIdempotentWriter(t)

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-recur", 1, "create_recurrence", "recurring_template", mock.AnythingOfType("usecases.WriteFn")).
		Return(usecases.IdempotentWriteResult{ResourceID: testRecurrenceID, Outcome: agent.ToolOutcomeReplay}, nil).Once()

	handle := BuildCreateRecurrenceTool(recurrences, writer)
	argsJSON, _ := json.Marshal(CreateRecurrenceInput{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    testCategoryID.String(),
		Frequency:     "monthly",
		DayOfMonth:    5,
	})
	out, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.NoError(t, err)

	var result CreateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.True(t, result.IsReplay)
	assert.Equal(t, testRecurrenceID.String(), result.ResourceID)
}

func TestBuildCreateRecurrenceToolInvalidUserID(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	writer := mocks.NewIdempotentWriter(t)

	handle := BuildCreateRecurrenceTool(recurrences, writer)
	argsJSON, _ := json.Marshal(CreateRecurrenceInput{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    testCategoryID.String(),
		Frequency:     "monthly",
		DayOfMonth:    5,
	})
	invalidCtx := agent.WithToolInvocationContext(context.Background(), "not-a-uuid", "wamid-recur", 1)
	_, err := handle.Invoke(invalidCtx, argsJSON)
	require.Error(t, err)
}

func TestBuildCreateRecurrenceToolWriterError(t *testing.T) {
	recurrences := imocks.NewRecurrenceManager(t)
	writer := mocks.NewIdempotentWriter(t)

	writer.EXPECT().Execute(mock.Anything, mock.AnythingOfType("uuid.UUID"), "wamid-recur", 1, "create_recurrence", "recurring_template", mock.AnythingOfType("usecases.WriteFn")).
		Return(usecases.IdempotentWriteResult{}, errors.New("write error")).Once()

	handle := BuildCreateRecurrenceTool(recurrences, writer)
	argsJSON, _ := json.Marshal(CreateRecurrenceInput{
		Direction:     "outcome",
		PaymentMethod: "debit",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    testCategoryID.String(),
		Frequency:     "monthly",
		DayOfMonth:    5,
	})
	_, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.Error(t, err)
}
