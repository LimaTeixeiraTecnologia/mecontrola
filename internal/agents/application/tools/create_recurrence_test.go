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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var testRecurrenceSubID = uuid.MustParse("00000000-0000-0000-0000-000000000011")

type fakeRecurrenceRegistrar struct {
	result usecases.RegisterResult
	err    error
	called bool
	lastAt usecases.CreateRecurrenceCommand
}

func (f *fakeRecurrenceRegistrar) CreateRecurrence(_ context.Context, cmd usecases.CreateRecurrenceCommand) (usecases.RegisterResult, error) {
	f.called = true
	f.lastAt = cmd
	return f.result, f.err
}

func recurrenceInput() CreateRecurrenceInput {
	return CreateRecurrenceInput{
		Direction:     "outcome",
		PaymentMethod: "debit_card",
		AmountCents:   15000,
		Description:   "Aluguel",
		CategoryID:    testCategoryID.String(),
		SubcategoryID: testRecurrenceSubID.String(),
		Frequency:     "monthly",
		DayOfMonth:    5,
	}
}

func TestBuildCreateRecurrenceTool_OpensPendingNoSyncWrite_CA16(t *testing.T) {
	registrar := &fakeRecurrenceRegistrar{result: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: "Confirma a criação desta recorrência?"}}

	handle := BuildCreateRecurrenceTool(registrar, imocks.NewCardManager(t))
	assert.Equal(t, "create_recurrence", handle.ID())
	assert.NotEmpty(t, handle.Description())

	argsJSON, _ := json.Marshal(recurrenceInput())
	out, verbatimText, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.NoError(t, err)

	var result CreateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, agent.ToolOutcomeClarify.String(), result.Outcome)
	assert.Empty(t, result.ResourceID)
	assert.True(t, registrar.called)
	assert.Equal(t, testCategoryID, registrar.lastAt.CategoryID)
	assert.Equal(t, testRecurrenceSubID, registrar.lastAt.SubcategoryID)
	assert.Equal(t, "monthly", registrar.lastAt.Frequency)
	assert.Equal(t, 5, registrar.lastAt.DayOfMonth)
	assert.Equal(t, "Confirma a criação desta recorrência?", verbatimText)
}

func TestBuildCreateRecurrenceTool_MissingSubcategory_DelegatesResolution(t *testing.T) {
	registrar := &fakeRecurrenceRegistrar{result: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify, Message: "Em qual categoria isso se encaixa? 📂"}}

	handle := BuildCreateRecurrenceTool(registrar, imocks.NewCardManager(t))
	in := recurrenceInput()
	in.CategoryID = ""
	in.SubcategoryID = ""
	in.CategoryText = "custos fixos"
	argsJSON, _ := json.Marshal(in)
	_, _, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.NoError(t, err)
	assert.True(t, registrar.called)
	assert.Equal(t, uuid.Nil, registrar.lastAt.CategoryID)
	assert.Equal(t, uuid.Nil, registrar.lastAt.SubcategoryID)
	assert.Equal(t, "custos fixos", registrar.lastAt.CategoryText)
}

func TestBuildCreateRecurrenceTool_InvalidUserID(t *testing.T) {
	registrar := &fakeRecurrenceRegistrar{result: usecases.RegisterResult{Outcome: agent.ToolOutcomeClarify}}

	handle := BuildCreateRecurrenceTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(recurrenceInput())
	invalidCtx := agent.WithToolInvocationContext(context.Background(), "not-a-uuid", "wamid-recur", 1)
	_, _, err := handle.Invoke(invalidCtx, argsJSON)
	require.Error(t, err)
}

func TestBuildCreateRecurrenceTool_RegistrarError(t *testing.T) {
	registrar := &fakeRecurrenceRegistrar{err: errors.New("start error")}

	handle := BuildCreateRecurrenceTool(registrar, imocks.NewCardManager(t))
	argsJSON, _ := json.Marshal(recurrenceInput())
	_, _, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.Error(t, err)
}

func TestBuildCreateRecurrenceTool_CardNotFoundAsksClarify(t *testing.T) {
	cardID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	registrar := &fakeRecurrenceRegistrar{}

	cards := imocks.NewCardManager(t)
	cards.EXPECT().GetCard(mock.Anything, cardID, testUserID).Return(interfaces.Card{}, interfaces.ErrCardNotFound).Once()
	handle := BuildCreateRecurrenceTool(registrar, cards)

	in := recurrenceInput()
	in.CardID = cardID.String()
	argsJSON, _ := json.Marshal(in)
	out, verbatimText, err := handle.Invoke(identityCtx("wamid-recur", 1), argsJSON)
	require.NoError(t, err)
	assert.NotEmpty(t, verbatimText)

	var result CreateRecurrenceOutput
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "clarify", result.Outcome)
	assert.Empty(t, result.ResourceID)
	assert.False(t, registrar.called)
}
