package messages_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
)

func TestExpenseConfirmationBlock(t *testing.T) {
	got := messages.ExpenseConfirmationBlock(messages.ConfirmationView{
		AmountFormatted: "R$ 300,00",
		DateFormatted:   "hoje",
		PaymentMethod:   "Dinheiro",
		Category:        "Casa",
	})
	want := "💰 Valor: R$ 300,00\n📅 Data: hoje\n📂 Categoria: Casa\n💳 Forma de pagamento: Dinheiro\n\nPosso registrar?"
	assert.Equal(t, want, got)
}

func TestIncomeConfirmationBlock(t *testing.T) {
	got := messages.IncomeConfirmationBlock(messages.ConfirmationView{
		AmountFormatted: "R$ 2.000,00",
		Origin:          "Salário",
	})
	want := "💰 Valor: R$ 2.000,00\n📥 Origem: Salário\n\nPosso registrar?"
	assert.Equal(t, want, got)
}

func TestWriteSuccess_Expense(t *testing.T) {
	seed := messages.NewMotivationSeed("wamid-1")
	got := messages.WriteSuccess(messages.WriteKindExpense, seed)
	assert.Contains(t, got, "Prontinho! ✅")
	assert.Contains(t, got, "💚")
}

func TestWriteSuccess_Income(t *testing.T) {
	seed := messages.NewMotivationSeed("wamid-2")
	got := messages.WriteSuccess(messages.WriteKindIncome, seed)
	assert.Contains(t, got, "Boa notícia! 🎉")
	assert.Contains(t, got, "💚")
}

func TestWriteSuccess_UnspecifiedFallsBackToExpense(t *testing.T) {
	seed := messages.NewMotivationSeed("wamid-3")
	got := messages.WriteSuccess(messages.WriteKindUnspecified, seed)
	assert.Contains(t, got, "Prontinho! ✅")
}

func TestTreatmentNameConfirmation(t *testing.T) {
	got := messages.TreatmentNameConfirmation("Stef")
	want := "Combinado, Stef! 💚 Vou te chamar assim daqui pra frente."
	assert.Equal(t, want, got)
}

func TestTreatmentNameEditQuestion(t *testing.T) {
	got := messages.TreatmentNameEditQuestion()
	want := "Claro! Como você gostaria que eu te chamasse a partir de agora? 💚"
	assert.Equal(t, want, got)
}

func TestTreatmentNameTooLong(t *testing.T) {
	got := messages.TreatmentNameTooLong()
	want := "Esse nome ficou um pouco longo. 😊 Pode me dizer uma forma mais curta pra eu te chamar? 💚"
	assert.Equal(t, want, got)
}

func TestMotivationSeed_DeterministicRotation(t *testing.T) {
	seedA1 := messages.NewMotivationSeed("wamid-abc")
	seedA2 := messages.NewMotivationSeed("wamid-abc")
	assert.Equal(t, seedA1, seedA2)

	msgA1 := messages.WriteSuccess(messages.WriteKindExpense, seedA1)
	msgA2 := messages.WriteSuccess(messages.WriteKindExpense, seedA2)
	assert.Equal(t, msgA1, msgA2, "mesmo seed deve produzir a mesma frase motivacional")
}

func TestMotivationSeed_DifferentSeedsCanRotate(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 50; i++ {
		seed := messages.NewMotivationSeed(fmt.Sprintf("wamid-rotation-%d", i))
		seen[messages.WriteSuccess(messages.WriteKindExpense, seed)] = struct{}{}
	}
	assert.Greater(t, len(seen), 1, "rotação deve variar entre seeds distintos")
}

func TestCategorySummaryBlock_Available(t *testing.T) {
	got := messages.CategorySummaryBlock(messages.CategoryView{
		Category: "Custo Fixo",
		Entries: []messages.CategoryEntryView{
			{DateFormatted: "01/07", AmountFormatted: "R$ 50,00", Subcategory: "Água"},
		},
		PlannedFormatted:   "R$ 500,00",
		SpentFormatted:     "R$ 50,00",
		AvailableFormatted: "R$ 450,00",
		Scenario:           messages.SummaryScenarioAvailable,
	})
	assert.Contains(t, got, "📊 Resumo de Custo Fixo:")
	assert.Contains(t, got, "- 01/07 | R$ 50,00 | Água")
	assert.Contains(t, got, "💰 Planejado: R$ 500,00")
	assert.Contains(t, got, "💸 Gasto: R$ 50,00")
	assert.Contains(t, got, "✅ Disponível: R$ 450,00")
}

func TestCategorySummaryBlock_ExactLimit(t *testing.T) {
	got := messages.CategorySummaryBlock(messages.CategoryView{
		Category:         "Lazer",
		PlannedFormatted: "R$ 200,00",
		SpentFormatted:   "R$ 200,00",
		Scenario:         messages.SummaryScenarioExactLimit,
	})
	assert.Contains(t, got, "atingiu exatamente o valor planejado para Lazer")
}

func TestCategorySummaryBlock_Exceeded(t *testing.T) {
	got := messages.CategorySummaryBlock(messages.CategoryView{
		Category:         "Alimentação",
		PlannedFormatted: "R$ 300,00",
		SpentFormatted:   "R$ 350,00",
		OverrunFormatted: "R$ 50,00",
		Scenario:         messages.SummaryScenarioExceeded,
	})
	assert.Contains(t, got, "ultrapassou em R$ 50,00 o valor planejado para Alimentação")
}

func TestCategorySummaryBlock_NearLimit(t *testing.T) {
	got := messages.CategorySummaryBlock(messages.CategoryView{
		Category:           "Transporte",
		PlannedFormatted:   "R$ 100,00",
		SpentFormatted:     "R$ 90,00",
		AvailableFormatted: "R$ 10,00",
		Scenario:           messages.SummaryScenarioNearLimit,
	})
	assert.Contains(t, got, "perto do limite")
}

func TestGeneralSummaryBlock(t *testing.T) {
	got := messages.GeneralSummaryBlock(messages.GeneralView{
		Categories: []messages.GeneralCategoryRowView{
			{Category: "Casa", PlannedFormatted: "R$ 500,00", SpentFormatted: "R$ 400,00", AvailableFormatted: "R$ 100,00"},
		},
		TotalPlannedFormatted:   "R$ 1.000,00",
		TotalSpentFormatted:     "R$ 800,00",
		TotalAvailableFormatted: "R$ 200,00",
		Scenario:                messages.GeneralScenarioPositive,
	})
	assert.Contains(t, got, "📊 Panorama geral do orçamento:")
	assert.Contains(t, got, "*Casa*")
	assert.Contains(t, got, "💰 Total planejado: R$ 1.000,00")
	assert.Contains(t, got, "💸 Total gasto: R$ 800,00")
	assert.Contains(t, got, "✅ Total disponível: R$ 200,00")
}

func TestGeneralSummaryBlock_ScenarioEmoji(t *testing.T) {
	cases := []struct {
		name     string
		scenario messages.GeneralScenario
		emoji    string
	}{
		{"positive", messages.GeneralScenarioPositive, "✅"},
		{"attention", messages.GeneralScenarioAttention, "⚠️"},
		{"critical", messages.GeneralScenarioCritical, "🚨"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := messages.GeneralSummaryBlock(messages.GeneralView{Scenario: tc.scenario})
			assert.Contains(t, got, tc.emoji)
		})
	}
}

func TestCancelPlanInfo(t *testing.T) {
	got := messages.CancelPlanInfo()
	assert.Contains(t, got, "Minhas Compras")
	assert.Contains(t, got, "MeControla")
	assert.Contains(t, got, "Gerenciar Assinatura")
	assert.Contains(t, got, "Cancelar Assinatura")
}

func TestSupportInfo(t *testing.T) {
	got := messages.SupportInfo()
	assert.Contains(t, got, "contato@limateixeira.com.br")
	assert.Contains(t, got, "24 horas")
}

func TestClarificationQuestion(t *testing.T) {
	cases := []struct {
		field messages.MissingField
		want  string
	}{
		{messages.MissingFieldAmount, "valor"},
		{messages.MissingFieldPaymentMethod, "pagou"},
		{messages.MissingFieldCategory, "categoria"},
		{messages.MissingFieldOrigin, "origem"},
		{messages.MissingFieldDescription, "trata"},
	}
	for _, tc := range cases {
		got := messages.ClarificationQuestion(tc.field)
		assert.Contains(t, got, tc.want)
	}
}

func TestWriteKind_IsValid(t *testing.T) {
	assert.True(t, messages.WriteKindExpense.IsValid())
	assert.True(t, messages.WriteKindIncome.IsValid())
	assert.False(t, messages.WriteKindUnspecified.IsValid())
}

func TestSummaryScenario_IsValid(t *testing.T) {
	assert.True(t, messages.SummaryScenarioAvailable.IsValid())
	assert.False(t, messages.SummaryScenarioUnspecified.IsValid())
}

func TestGeneralScenario_IsValid(t *testing.T) {
	assert.True(t, messages.GeneralScenarioPositive.IsValid())
	assert.False(t, messages.GeneralScenarioUnspecified.IsValid())
}

func TestMissingField_IsValid(t *testing.T) {
	assert.True(t, messages.MissingFieldAmount.IsValid())
	assert.False(t, messages.MissingFieldUnspecified.IsValid())
}
