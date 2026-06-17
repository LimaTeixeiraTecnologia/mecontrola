package entities_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestTransactionCreated_JSONRoundTrip(t *testing.T) {
	refMonth, _ := valueobjects.NewRefMonth("2026-06")
	categoryID := uuid.New()
	subcategoryID := uuid.New()
	evt := entities.TransactionCreated{
		EventID:       uuid.New(),
		AggregateID:   uuid.New(),
		UserID:        uuid.New(),
		OccurredAt:    time.Now().UTC(),
		Direction:     valueobjects.DirectionOutcome,
		PaymentMethod: valueobjects.PaymentMethodPix,
		AmountCents:   1000,
		RefMonth:      refMonth,
		CategoryID:    categoryID,
		SubcategoryID: subcategoryID,
	}

	data, err := json.Marshal(evt)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"ref_month":"2026-06"`)
	assert.Contains(t, string(data), `"category_id":"`+categoryID.String()+`"`)
	assert.Contains(t, string(data), `"subcategory_id":"`+subcategoryID.String()+`"`)

	var decoded entities.TransactionCreated
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, evt.EventID, decoded.EventID)
	assert.Equal(t, evt.AggregateID, decoded.AggregateID)
	assert.Equal(t, "2026-06", decoded.RefMonth.String())
	assert.Equal(t, categoryID, decoded.CategoryID)
	assert.Equal(t, subcategoryID, decoded.SubcategoryID)
}

func TestCardPurchaseUpdated_HasInvoiceDeltas(t *testing.T) {
	refMonth1, _ := valueobjects.NewRefMonth("2026-06")
	refMonth2, _ := valueobjects.NewRefMonth("2026-07")

	evt := entities.CardPurchaseUpdated{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		CardID:            uuid.New(),
		TotalAmountCents:  9000,
		InstallmentsTotal: 3,
		RefMonthsAffected: []valueobjects.RefMonth{refMonth1, refMonth2},
		InvoiceDeltas:     map[string]int64{"2026-06": 3000, "2026-07": 3000},
	}

	data, err := json.Marshal(evt)
	require.NoError(t, err)
	assert.Contains(t, string(data), "invoice_deltas")
	assert.Contains(t, string(data), "ref_months_affected")
}

func TestRecurringTemplateCreated_Fields(t *testing.T) {
	evt := entities.RecurringTemplateCreated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}
	data, err := json.Marshal(evt)
	require.NoError(t, err)
	assert.Contains(t, string(data), "event_id")
}
