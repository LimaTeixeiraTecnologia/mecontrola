package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func buildCreateTransactionCmd(t *testing.T) commands.CreateTransaction {
	t.Helper()
	userID := valueobjects.UserIDFromUUID(uuid.New())
	direction, err := valueobjects.ParseDirection("income")
	require.NoError(t, err)
	pm, err := valueobjects.ParsePaymentMethodForCreate("pix")
	require.NoError(t, err)
	amount, err := valueobjects.NewMoney(1000)
	require.NoError(t, err)
	desc, err := valueobjects.NewDescription("Test transaction")
	require.NoError(t, err)
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	return commands.CreateTransaction{
		UserID:        userID,
		Direction:     direction,
		PaymentMethod: pm,
		Amount:        amount,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		OccurredAt:    time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
	}
}

func TestTransactionWorkflow_DecideCreate_PopulatesCategory(t *testing.T) {
	sut := services.TransactionWorkflow{}
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	txID := uuid.New()
	eventID := uuid.New()

	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	subID := valueobjects.SubcategoryIDFromUUID(uuid.New())
	cmd := buildCreateTransactionCmd(t)
	cmd.CategoryID = catID
	cmd.SubcategoryID = option.Some(subID)

	decision := sut.DecideCreate(cmd, txID, eventID, now)

	evt, ok := decision.Event.(entities.TransactionCreated)
	require.True(t, ok)
	assert.Equal(t, catID.UUID(), evt.CategoryID)
	assert.Equal(t, subID.UUID(), evt.SubcategoryID)
}

func TestTransactionWorkflow_DecideCreate_NoSubcategory_UsesNil(t *testing.T) {
	sut := services.TransactionWorkflow{}
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	txID := uuid.New()
	eventID := uuid.New()

	cmd := buildCreateTransactionCmd(t)
	cmd.SubcategoryID = option.None[valueobjects.SubcategoryID]()

	decision := sut.DecideCreate(cmd, txID, eventID, now)

	evt, ok := decision.Event.(entities.TransactionCreated)
	require.True(t, ok)
	assert.Equal(t, uuid.Nil, evt.SubcategoryID)
	assert.Equal(t, cmd.CategoryID.UUID(), evt.CategoryID)
}

func TestTransactionWorkflow_DecideCreate(t *testing.T) {
	sut := services.TransactionWorkflow{}
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	txID := uuid.New()
	eventID := uuid.New()

	cmd := buildCreateTransactionCmd(t)
	decision := sut.DecideCreate(cmd, txID, eventID, now)

	assert.Equal(t, txID, decision.Transaction.ID())
	assert.Equal(t, "2024-03", decision.Transaction.RefMonth().String())
	assert.Equal(t, int64(1), decision.Transaction.Version())

	evt, ok := decision.Event.(entities.TransactionCreated)
	require.True(t, ok, "evento deve ser TransactionCreated")
	assert.Equal(t, eventID, evt.EventID)
	assert.Equal(t, txID, evt.AggregateID)
	assert.Equal(t, "2024-03", evt.RefMonth.String())
	assert.Equal(t, int64(1000), evt.AmountCents)
}

func TestTransactionWorkflow_DecideUpdate_SameRefMonth(t *testing.T) {
	sut := services.TransactionWorkflow{}
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	txID := uuid.New()
	eventID := uuid.New()

	cmd := buildCreateTransactionCmd(t)
	createDecision := sut.DecideCreate(cmd, txID, eventID, now)

	updateEventID := uuid.New()
	updateNow := now.Add(time.Hour)

	pm, _ := valueobjects.ParsePaymentMethodForCreate("ted")
	amount, _ := valueobjects.NewMoney(2000)
	desc, _ := valueobjects.NewDescription("Updated transaction")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())

	updateCmd := commands.UpdateTransaction{
		TransactionID: txID,
		UserID:        cmd.UserID,
		Direction:     cmd.Direction,
		PaymentMethod: pm,
		Amount:        amount,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		OccurredAt:    time.Date(2024, 3, 20, 12, 0, 0, 0, time.UTC),
		Version:       1,
	}

	decision := sut.DecideUpdate(createDecision.Transaction, updateCmd, updateEventID, updateNow)

	evt, ok := decision.Event.(entities.TransactionUpdated)
	require.True(t, ok)
	assert.Len(t, evt.RefMonthsAffected, 1, "mesmo mês → apenas 1 competência afetada")
	assert.Equal(t, "2024-03", evt.RefMonthsAffected[0].String())
}

func TestTransactionWorkflow_DecideUpdate_ChangedRefMonth(t *testing.T) {
	sut := services.TransactionWorkflow{}
	now := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	txID := uuid.New()
	eventID := uuid.New()

	cmd := buildCreateTransactionCmd(t)
	createDecision := sut.DecideCreate(cmd, txID, eventID, now)

	updateEventID := uuid.New()
	updateNow := now.Add(time.Hour)

	amount, _ := valueobjects.NewMoney(2000)
	desc, _ := valueobjects.NewDescription("Updated transaction")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())

	updateCmd := commands.UpdateTransaction{
		TransactionID: txID,
		UserID:        cmd.UserID,
		Direction:     cmd.Direction,
		PaymentMethod: cmd.PaymentMethod,
		Amount:        amount,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		OccurredAt:    time.Date(2024, 4, 5, 12, 0, 0, 0, time.UTC),
		Version:       1,
	}

	decision := sut.DecideUpdate(createDecision.Transaction, updateCmd, updateEventID, updateNow)

	evt, ok := decision.Event.(entities.TransactionUpdated)
	require.True(t, ok)
	assert.Len(t, evt.RefMonthsAffected, 2, "mês mudou → 2 competências afetadas")

	refStrings := make([]string, len(evt.RefMonthsAffected))
	for i, r := range evt.RefMonthsAffected {
		refStrings[i] = r.String()
	}
	assert.Contains(t, refStrings, "2024-03")
	assert.Contains(t, refStrings, "2024-04")
}
