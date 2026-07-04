package input

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type RawTransactionValidateSuite struct {
	suite.Suite
}

func TestRawTransactionValidateSuite(t *testing.T) {
	suite.Run(t, new(RawTransactionValidateSuite))
}

func (s *RawTransactionValidateSuite) TestRawCreateTransactionValidate() {
	cardID := uuid.New()
	catID := uuid.New()
	base := func() RawCreateTransaction {
		return RawCreateTransaction{
			Direction:     "outcome",
			PaymentMethod: "pix",
			AmountCents:   1000,
			Description:   "Mercado",
			CategoryID:    catID,
			OccurredAt:    "2026-06-01T12:00:00Z",
		}
	}

	scenarios := []struct {
		name   string
		mutate func(*RawCreateTransaction)
		expect func(err error)
	}{
		{
			name:   "não-crédito sem card_id é válido",
			mutate: func(_ *RawCreateTransaction) {},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "credit_card sem card_id falha nomeando o campo",
			mutate: func(r *RawCreateTransaction) {
				r.PaymentMethod = "credit_card"
				r.Installments = 1
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputCardIDRequired) },
		},
		{
			name: "credit_card com installments 0 falha",
			mutate: func(r *RawCreateTransaction) {
				r.PaymentMethod = "credit_card"
				r.CardID = &cardID
				r.Installments = 0
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputInstallmentsOutOfRange) },
		},
		{
			name: "credit_card com installments 25 falha",
			mutate: func(r *RawCreateTransaction) {
				r.PaymentMethod = "credit_card"
				r.CardID = &cardID
				r.Installments = 25
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputInstallmentsOutOfRange) },
		},
		{
			name: "credit_card com card_id e installments 1..24 é válido",
			mutate: func(r *RawCreateTransaction) {
				r.PaymentMethod = "credit_card"
				r.CardID = &cardID
				r.Installments = 12
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "origin_wamid sem origin_operation falha",
			mutate: func(r *RawCreateTransaction) {
				r.OriginWamid = "wamid.123"
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "origin_operation")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			raw := base()
			scenario.mutate(&raw)
			scenario.expect(raw.Validate())
		})
	}
}

func (s *RawTransactionValidateSuite) TestRawUpdateTransactionValidate() {
	cardID := uuid.New()
	catID := uuid.New()
	base := func() RawUpdateTransaction {
		return RawUpdateTransaction{
			Direction:     "outcome",
			PaymentMethod: "pix",
			AmountCents:   1000,
			Description:   "Mercado",
			CategoryID:    catID,
			OccurredAt:    "2026-06-01T12:00:00Z",
			Version:       1,
		}
	}

	scenarios := []struct {
		name   string
		mutate func(*RawUpdateTransaction)
		expect func(err error)
	}{
		{
			name:   "não-crédito sem card_id é válido",
			mutate: func(_ *RawUpdateTransaction) {},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "credit_card sem card_id falha nomeando o campo",
			mutate: func(r *RawUpdateTransaction) {
				r.PaymentMethod = "credit_card"
				r.Installments = 1
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputCardIDRequired) },
		},
		{
			name: "credit_card com installments fora de faixa falha",
			mutate: func(r *RawUpdateTransaction) {
				r.PaymentMethod = "credit_card"
				r.CardID = &cardID
				r.Installments = 25
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputInstallmentsOutOfRange) },
		},
		{
			name: "credit_card completo é válido",
			mutate: func(r *RawUpdateTransaction) {
				r.PaymentMethod = "credit_card"
				r.CardID = &cardID
				r.Installments = 6
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "version zero falha",
			mutate: func(r *RawUpdateTransaction) {
				r.Version = 0
			},
			expect: func(err error) { s.ErrorIs(err, ErrInputVersionRequired) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			raw := base()
			scenario.mutate(&raw)
			scenario.expect(raw.Validate())
		})
	}
}
