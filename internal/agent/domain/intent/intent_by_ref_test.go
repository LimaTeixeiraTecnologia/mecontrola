package intent

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type IntentByRefSuite struct {
	suite.Suite
}

func TestIntentByRefSuite(t *testing.T) {
	suite.Run(t, new(IntentByRefSuite))
}

func (s *IntentByRefSuite) TestByRefKinds_StringAndParseRoundTrip() {
	for _, k := range []Kind{KindDeleteTransactionByRef, KindEditTransactionByRef} {
		parsed, err := ParseKind(k.String())
		s.NoError(err)
		s.Equal(k, parsed)
	}
}

func (s *IntentByRefSuite) TestByRefKinds_AreWrites() {
	s.True(KindDeleteTransactionByRef.IsWrite())
	s.True(KindEditTransactionByRef.IsWrite())
}

func (s *IntentByRefSuite) TestNewDeleteTransactionByRef() {
	s.Run("valido", func() {
		got, err := NewDeleteTransactionByRef("uber")
		s.NoError(err)
		s.Equal(KindDeleteTransactionByRef, got.Kind())
		s.Equal("uber", got.SearchQuery())
	})
	s.Run("trim", func() {
		got, err := NewDeleteTransactionByRef("  mercado  ")
		s.NoError(err)
		s.Equal("mercado", got.SearchQuery())
	})
	s.Run("muito curto", func() {
		_, err := NewDeleteTransactionByRef("a")
		s.ErrorIs(err, ErrSearchQueryTooShort)
	})
	s.Run("vazio", func() {
		_, err := NewDeleteTransactionByRef("   ")
		s.ErrorIs(err, ErrSearchQueryTooShort)
	})
}

func (s *IntentByRefSuite) TestNewEditTransactionByRef() {
	s.Run("valido", func() {
		got, err := NewEditTransactionByRef("uber", 4200)
		s.NoError(err)
		s.Equal(KindEditTransactionByRef, got.Kind())
		s.Equal("uber", got.SearchQuery())
		s.Equal(int64(4200), got.AmountCents())
	})
	s.Run("amount nao positivo", func() {
		_, err := NewEditTransactionByRef("uber", 0)
		s.ErrorIs(err, ErrAmountNonPositive)
	})
	s.Run("query curta", func() {
		_, err := NewEditTransactionByRef("a", 4200)
		s.ErrorIs(err, ErrSearchQueryTooShort)
	})
}
