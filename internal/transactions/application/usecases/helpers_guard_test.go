package usecases

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CategoryGuardSuite struct {
	suite.Suite
}

func TestCategoryGuardSuite(t *testing.T) {
	suite.Run(t, new(CategoryGuardSuite))
}

func (s *CategoryGuardSuite) TestGuardSubcategoryRequired() {
	scenarios := []struct {
		name      string
		direction valueobjects.Direction
		present   bool
		expect    func(err error)
	}{
		{
			name:      "outcome sem subcategory falha (RF-19)",
			direction: valueobjects.DirectionOutcome,
			present:   false,
			expect: func(err error) {
				s.ErrorIs(err, ErrTransactionRequiresSubcategory)
			},
		},
		{
			name:      "outcome com subcategory ok",
			direction: valueobjects.DirectionOutcome,
			present:   true,
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:      "income sem subcategory falha (RF-20)",
			direction: valueobjects.DirectionIncome,
			present:   false,
			expect: func(err error) {
				s.ErrorIs(err, ErrTransactionRequiresSubcategory)
			},
		},
		{
			name:      "income com subcategory ok",
			direction: valueobjects.DirectionIncome,
			present:   true,
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(guardSubcategoryRequired(scenario.direction, scenario.present))
		})
	}
}

func (s *CategoryGuardSuite) TestGuardCategoryKindDirection() {
	scenarios := []struct {
		name      string
		direction valueobjects.Direction
		kind      string
		expect    func(err error)
	}{
		{
			name:      "outcome com expense ok (RF-21)",
			direction: valueobjects.DirectionOutcome,
			kind:      "expense",
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:      "income com income ok (RF-21)",
			direction: valueobjects.DirectionIncome,
			kind:      "income",
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:      "outcome com income falha (RF-21)",
			direction: valueobjects.DirectionOutcome,
			kind:      "income",
			expect: func(err error) {
				s.ErrorIs(err, ErrCategoryKindDirectionMismatch)
			},
		},
		{
			name:      "income com expense falha (RF-21)",
			direction: valueobjects.DirectionIncome,
			kind:      "expense",
			expect: func(err error) {
				s.ErrorIs(err, ErrCategoryKindDirectionMismatch)
			},
		},
		{
			name:      "kind vazio nao valida",
			direction: valueobjects.DirectionOutcome,
			kind:      "",
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(guardCategoryKindDirection(scenario.direction, scenario.kind))
		})
	}
}

func (s *CategoryGuardSuite) TestGuardPaymentMethodMigration() {
	scenarios := []struct {
		name    string
		current valueobjects.PaymentMethod
		next    valueobjects.PaymentMethod
		expect  func(err error)
	}{
		{
			name:    "credit_card para pix é rejeitado",
			current: valueobjects.PaymentMethodCreditCard,
			next:    valueobjects.PaymentMethodPix,
			expect: func(err error) {
				s.ErrorIs(err, ErrPaymentMethodMigrationNotAllowed)
			},
		},
		{
			name:    "pix para credit_card é rejeitado",
			current: valueobjects.PaymentMethodPix,
			next:    valueobjects.PaymentMethodCreditCard,
			expect: func(err error) {
				s.ErrorIs(err, ErrPaymentMethodMigrationNotAllowed)
			},
		},
		{
			name:    "credit_card para credit_card é permitido",
			current: valueobjects.PaymentMethodCreditCard,
			next:    valueobjects.PaymentMethodCreditCard,
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name:    "pix para boleto é permitido",
			current: valueobjects.PaymentMethodPix,
			next:    valueobjects.PaymentMethodBoleto,
			expect: func(err error) {
				s.NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(guardPaymentMethodMigration(scenario.current, scenario.next))
		})
	}
}
