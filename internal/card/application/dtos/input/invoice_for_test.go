package input_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

type InvoiceForInputSuite struct {
	suite.Suite
}

func TestInvoiceForInput(t *testing.T) {
	suite.Run(t, new(InvoiceForInputSuite))
}

func (s *InvoiceForInputSuite) TestNewInvoiceFor_ValidDate() {
	cardID := uuid.New()
	userID := uuid.New()

	got, err := input.NewInvoiceFor(cardID, userID, "2026-01-15")
	s.Require().NoError(err)

	s.Equal(cardID, got.CardID)
	s.Equal(userID, got.UserID)
	want := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	s.True(got.Purchase.Equal(want))
}

func (s *InvoiceForInputSuite) TestNewInvoiceFor_InvalidFormatsWrapSentinel() {
	scenarios := []struct {
		name string
		raw  string
	}{
		{name: "vazio", raw: ""},
		{name: "ordem brasileira", raw: "15-01-2026"},
		{name: "com hora", raw: "2026-01-15T10:00:00Z"},
		{name: "lixo", raw: "not-a-date"},
		{name: "mês inválido", raw: "2026-13-01"},
		{name: "dia inválido", raw: "2026-02-30"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			_, err := input.NewInvoiceFor(uuid.New(), uuid.New(), sc.raw)
			s.Require().Error(err)
			s.True(errors.Is(err, domain.ErrInvalidPurchaseDate))
		})
	}
}
