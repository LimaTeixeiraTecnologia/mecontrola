package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func buildTemplate(t *testing.T, paymentMethod valueobjects.PaymentMethod, dayOfMonth int, startedAt time.Time) entities.RecurringTemplate {
	t.Helper()

	userID := valueobjects.UserIDFromUUID(uuid.New())
	direction, err := valueobjects.ParseDirection("outcome")
	require.NoError(t, err)
	amount, err := valueobjects.NewMoney(1000)
	require.NoError(t, err)
	desc, err := valueobjects.NewDescription("Template")
	require.NoError(t, err)
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq, err := valueobjects.ParseFrequency("monthly")
	require.NoError(t, err)
	dom, err := valueobjects.NewDayOfMonth(dayOfMonth)
	require.NoError(t, err)
	inst, err := valueobjects.NewInstallmentCount(1)
	require.NoError(t, err)

	var cardID option.Option[valueobjects.CardID]
	if paymentMethod == valueobjects.PaymentMethodCreditCard {
		cardID = option.Some(valueobjects.CardIDFromUUID(uuid.New()))
	}

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return entities.NewRecurringTemplate(
		uuid.New(),
		userID,
		direction,
		paymentMethod,
		cardID,
		amount,
		desc,
		catID,
		option.None[valueobjects.SubcategoryID](),
		"",
		"",
		valueobjects.CategoryWriteEvidence{},
		freq,
		dom,
		inst,
		startedAt,
		option.None[time.Time](),
		now,
	)
}

func TestRecurringWorkflow_DecideMaterializeForDay(t *testing.T) {
	sut := services.RecurringWorkflow{}
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	startedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		paymentMethod     valueobjects.PaymentMethod
		dayOfMonth        int
		today             time.Time
		wantMaterialize   bool
		wantAsTransaction bool
		wantAsCard        bool
	}{
		{
			name:              "débito — dia correto → materializa como transação",
			paymentMethod:     valueobjects.PaymentMethodDebitInAccount,
			dayOfMonth:        15,
			today:             time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
			wantMaterialize:   true,
			wantAsTransaction: true,
			wantAsCard:        false,
		},
		{
			name:              "crédito — dia correto → materializa como card purchase",
			paymentMethod:     valueobjects.PaymentMethodCreditCard,
			dayOfMonth:        15,
			today:             time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
			wantMaterialize:   true,
			wantAsTransaction: false,
			wantAsCard:        true,
		},
		{
			name:            "dia diferente → não materializa",
			paymentMethod:   valueobjects.PaymentMethodPix,
			dayOfMonth:      15,
			today:           time.Date(2024, 3, 16, 12, 0, 0, 0, time.UTC),
			wantMaterialize: false,
		},
		{
			name:            "antes de startedAt → não materializa",
			paymentMethod:   valueobjects.PaymentMethodPix,
			dayOfMonth:      15,
			today:           time.Date(2023, 12, 15, 12, 0, 0, 0, time.UTC),
			wantMaterialize: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			template := buildTemplate(t, tc.paymentMethod, tc.dayOfMonth, startedAt)
			decision := sut.DecideMaterializeForDay(template, tc.today, loc)

			assert.Equal(t, tc.wantMaterialize, decision.ShouldMaterialize)
			if tc.wantMaterialize {
				assert.Equal(t, tc.wantAsTransaction, decision.AsTransaction)
				assert.Equal(t, tc.wantAsCard, decision.AsCardPurchase)
				assert.False(t, decision.RefMonth.IsZero())
				assert.Equal(t, template.ID(), decision.TemplateID)
			}
		})
	}
}

func TestRecurringWorkflow_EndedAt(t *testing.T) {
	sut := services.RecurringWorkflow{}
	loc := time.UTC

	startedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endedAt := time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC)

	userID := valueobjects.UserIDFromUUID(uuid.New())
	direction, _ := valueobjects.ParseDirection("outcome")
	amount, _ := valueobjects.NewMoney(1000)
	desc, _ := valueobjects.NewDescription("Template")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq, _ := valueobjects.ParseFrequency("monthly")
	dom, _ := valueobjects.NewDayOfMonth(15)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	template := entities.NewRecurringTemplate(
		uuid.New(),
		userID,
		direction,
		valueobjects.PaymentMethodPix,
		option.None[valueobjects.CardID](),
		amount,
		desc,
		catID,
		option.None[valueobjects.SubcategoryID](),
		"",
		"",
		valueobjects.CategoryWriteEvidence{},
		freq,
		dom,
		inst,
		startedAt,
		option.Some(endedAt),
		now,
	)

	decisionBefore := sut.DecideMaterializeForDay(template, time.Date(2024, 2, 15, 12, 0, 0, 0, loc), loc)
	assert.True(t, decisionBefore.ShouldMaterialize, "antes de ended_at deve materializar")

	decisionAfter := sut.DecideMaterializeForDay(template, time.Date(2024, 3, 15, 12, 0, 0, 0, loc), loc)
	assert.False(t, decisionAfter.ShouldMaterialize, "após ended_at não deve materializar")
}
