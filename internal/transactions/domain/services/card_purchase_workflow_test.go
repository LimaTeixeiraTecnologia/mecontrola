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

func buildCreateCardPurchaseCmd(t *testing.T, totalCents int64, installments int, purchasedAt time.Time) commands.CreateCardPurchase {
	t.Helper()
	userID := valueobjects.UserIDFromUUID(uuid.New())
	cardID := valueobjects.CardIDFromUUID(uuid.New())
	amount, err := valueobjects.NewMoney(totalCents)
	require.NoError(t, err)
	inst, err := valueobjects.NewInstallmentCount(installments)
	require.NoError(t, err)
	desc, err := valueobjects.NewDescription("Test purchase")
	require.NoError(t, err)
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	return commands.CreateCardPurchase{
		UserID:        userID,
		CardID:        cardID,
		TotalAmount:   amount,
		Installments:  inst,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		PurchasedAt:   purchasedAt,
	}
}

func TestCardPurchaseWorkflow_DecideCreate(t *testing.T) {
	sut := services.NewCardPurchaseWorkflow()
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	snapshot := mustSnapshot(15, 25)

	tests := []struct {
		name          string
		totalCents    int64
		installments  int
		purchasedAt   time.Time
		wantRefMonths []string
		wantItemCount int
	}{
		{
			name:          "1 parcela antes do fechamento",
			totalCents:    1000,
			installments:  1,
			purchasedAt:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			wantRefMonths: []string{"2024-01"},
			wantItemCount: 1,
		},
		{
			name:          "3 parcelas com virada",
			totalCents:    3000,
			installments:  3,
			purchasedAt:   time.Date(2024, 11, 10, 0, 0, 0, 0, time.UTC),
			wantRefMonths: []string{"2024-11", "2024-12", "2025-01"},
			wantItemCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildCreateCardPurchaseCmd(t, tc.totalCents, tc.installments, tc.purchasedAt)
			purchaseID := uuid.New()
			eventID := uuid.New()

			decision := sut.DecideCreate(cmd, snapshot, purchaseID, eventID, now)

			assert.Equal(t, purchaseID, decision.Purchase.ID())
			require.Len(t, decision.Items, tc.wantItemCount)

			evt, ok := decision.Event.(entities.CardPurchaseCreated)
			require.True(t, ok)
			assert.Equal(t, eventID, evt.EventID)
			require.Len(t, evt.RefMonthsAffected, len(tc.wantRefMonths))
			for i, ref := range tc.wantRefMonths {
				assert.Equal(t, ref, evt.RefMonthsAffected[i].String())
			}

			var totalSum int64
			for _, item := range decision.Items {
				totalSum += item.Amount().Cents()
			}
			assert.Equal(t, tc.totalCents, totalSum, "soma dos itens deve ser igual ao total")

			require.Len(t, evt.Installments, tc.wantItemCount)
			var installmentSum int64
			for i, inst := range evt.Installments {
				assert.Equal(t, decision.Items[i].RefMonth().String(), inst.RefMonth.String())
				assert.Equal(t, decision.Items[i].Amount().Cents(), inst.AmountCents)
				assert.Equal(t, decision.Items[i].InstallmentIndex(), inst.Index)
				installmentSum += inst.AmountCents
			}
			assert.Equal(t, tc.totalCents, installmentSum, "soma das parcelas do evento deve ser igual ao total")
			assert.Equal(t, uuid.Nil, evt.SubcategoryID)
		})
	}
}

func TestCardPurchaseWorkflow_DecideCreate_PopulatesSubcategory(t *testing.T) {
	sut := services.NewCardPurchaseWorkflow()
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	snapshot := mustSnapshot(15, 25)
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	cmd := buildCreateCardPurchaseCmd(t, 1000, 1, purchasedAt)
	subUUID := uuid.New()
	subID := valueobjects.SubcategoryIDFromUUID(subUUID)
	cmd.SubcategoryID = option.Some(subID)

	decision := sut.DecideCreate(cmd, snapshot, uuid.New(), uuid.New(), now)

	evt, ok := decision.Event.(entities.CardPurchaseCreated)
	require.True(t, ok)
	assert.Equal(t, subUUID, evt.SubcategoryID)
}

func TestCardPurchaseWorkflow_DecideUpdate_12to3(t *testing.T) {
	sut := services.NewCardPurchaseWorkflow()
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	snapshot := mustSnapshot(15, 25)
	purchasedAt := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)

	createCmd := buildCreateCardPurchaseCmd(t, 12000, 12, purchasedAt)
	purchaseID := uuid.New()
	createDecision := sut.DecideCreate(createCmd, snapshot, purchaseID, uuid.New(), now)

	userID := valueobjects.UserIDFromUUID(uuid.New())
	amount, _ := valueobjects.NewMoney(3000)
	inst, _ := valueobjects.NewInstallmentCount(3)
	desc, _ := valueobjects.NewDescription("Updated purchase")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())

	updateCmd := commands.UpdateCardPurchase{
		PurchaseID:    purchaseID,
		UserID:        userID,
		TotalAmount:   amount,
		Installments:  inst,
		Description:   desc,
		CategoryID:    catID,
		SubcategoryID: option.None[valueobjects.SubcategoryID](),
		PurchasedAt:   purchasedAt,
		Version:       1,
	}

	updateDecision := sut.DecideUpdate(createDecision.Purchase, createDecision.Items, updateCmd, uuid.New(), now.Add(time.Hour))

	evt, ok := updateDecision.Event.(entities.CardPurchaseUpdated)
	require.True(t, ok)

	assert.Len(t, evt.RefMonthsAffected, 12, "deve ter 12 competências afetadas (9 antigas + 3 novas overlap)")

	negativeCount := 0
	for _, delta := range evt.InvoiceDeltas {
		if delta < 0 {
			negativeCount++
		}
	}
	assert.Equal(t, 9, negativeCount, "9 competências removidas devem ter delta negativo")

	assert.Len(t, updateDecision.Items, 3, "deve ter 3 itens após update")
}

func TestCardPurchaseWorkflow_DecideDelete(t *testing.T) {
	sut := services.NewCardPurchaseWorkflow()
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	snapshot := mustSnapshot(15, 25)

	cmd := buildCreateCardPurchaseCmd(t, 3000, 3, time.Date(2024, 11, 10, 0, 0, 0, 0, time.UTC))
	purchaseID := uuid.New()
	createDecision := sut.DecideCreate(cmd, snapshot, purchaseID, uuid.New(), now)

	deleteDecision, err := sut.DecideDelete(createDecision.Purchase, createDecision.Items, uuid.New(), now.Add(time.Hour))
	require.NoError(t, err)

	evt, ok := deleteDecision.Event.(entities.CardPurchaseDeleted)
	require.True(t, ok)

	assert.Len(t, evt.RefMonthsAffected, 3)
	for _, delta := range evt.InvoiceDeltas {
		assert.Less(t, delta, int64(0), "delta deve ser negativo (remoção)")
	}
}
