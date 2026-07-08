package workflows

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
)

func todayOccurredAt() (string, string) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return now.Format("2006-01-02"), now.Format("02/01/2006")
}

func TestFormatDateLabel_ExplicitCalendarDate(t *testing.T) {
	raw, formatted := todayOccurredAt()

	label := formatDateLabel(raw)

	require.Equal(t, fmt.Sprintf("hoje (%s)", formatted), label)
}

func TestFormatDateLabel_YesterdayIncludesDate(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	now := time.Now().In(loc)
	yesterday := now.AddDate(0, 0, -1)

	label := formatDateLabel(yesterday.Format("2006-01-02"))

	require.Equal(t, fmt.Sprintf("ontem (%s)", yesterday.Format("02/01/2006")), label)
}

func TestFormatDateLabel_OtherDateFullYear(t *testing.T) {
	label := formatDateLabel("2020-03-15")

	require.Equal(t, "15/03/2020", label)
}

func TestBuildConfirmSummary_ExpensePixCarriesAllFields(t *testing.T) {
	raw, formatted := todayOccurredAt()
	state := PendingEntryState{
		Kind:          interfaces.CategoryKindExpense,
		Description:   "Supermercado",
		AmountCents:   15000,
		PaymentMethod: "pix",
		OccurredAt:    raw,
		Candidates:    []PendingCategoryCandidate{{Path: "Custo Fixo > Supermercado"}},
	}

	summary := buildConfirmSummary(state)

	assert.Contains(t, summary, "*Supermercado*")
	assert.Contains(t, summary, "R$ 150,00")
	assert.Contains(t, summary, "*Custo Fixo > Supermercado*")
	assert.Contains(t, summary, fmt.Sprintf("para hoje (%s)", formatted))
	assert.Contains(t, summary, "no pix")
}

func TestBuildConfirmSummary_IncomeOmitsPaymentMethod(t *testing.T) {
	raw, _ := todayOccurredAt()
	state := PendingEntryState{
		Kind:          interfaces.CategoryKindIncome,
		Description:   "Freelancer",
		AmountCents:   20000,
		PaymentMethod: "pix",
		OccurredAt:    raw,
		Candidates:    []PendingCategoryCandidate{{Path: "Receita Variável > Freelance"}},
	}

	summary := buildConfirmSummary(state)

	assert.Contains(t, summary, "*Freelancer*")
	assert.Contains(t, summary, "R$ 200,00")
	assert.NotContains(t, summary, "pix")
	assert.Empty(t, confirmPaymentSegment(state))
}

func TestBuildConfirmSummary_CreditCardShowsInstallments(t *testing.T) {
	raw := "2020-03-15"
	state := PendingEntryState{
		Kind:          interfaces.CategoryKindExpense,
		Description:   "Geladeira",
		AmountCents:   200000,
		PaymentMethod: "credit_card",
		Installments:  10,
		OccurredAt:    raw,
		Candidates:    []PendingCategoryCandidate{{Path: "Casa > Eletrodomésticos"}},
	}

	summary := buildConfirmSummary(state)

	assert.Contains(t, summary, "no crédito em 10x")
	assert.Contains(t, summary, "15/03/2020")
}

func TestBuildConfirmSummary_CreditCardAVista(t *testing.T) {
	state := PendingEntryState{
		Kind:          interfaces.CategoryKindExpense,
		Description:   "Gasolina",
		AmountCents:   12000,
		PaymentMethod: "credit_card",
		Installments:  1,
		OccurredAt:    "2020-03-15",
		Candidates:    []PendingCategoryCandidate{{Path: "Transporte > Combustível"}},
	}

	summary := buildConfirmSummary(state)

	assert.Contains(t, summary, "no crédito à vista")
	assert.NotContains(t, summary, "em 1x")
}

func TestBuildConfirmSummary_KeepsConfirmaPrefix(t *testing.T) {
	state := PendingEntryState{
		Kind:        interfaces.CategoryKindExpense,
		Description: "Café",
		AmountCents: 500,
		OccurredAt:  "2020-03-15",
		Candidates:  []PendingCategoryCandidate{{Path: "Prazeres > Lazer"}},
	}

	summary := buildConfirmSummary(state)

	assert.True(t, strings.HasPrefix(summary, "Confirma? "))
}
