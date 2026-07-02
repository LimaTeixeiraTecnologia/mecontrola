package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type BankCodeSuite struct {
	suite.Suite
}

func TestBankCodeSuite(t *testing.T) {
	suite.Run(t, new(BankCodeSuite))
}

func (s *BankCodeSuite) TestNewBankCode_PreservesOriginalDisplay() {
	b, err := valueobjects.NewBankCode("Banco do Brasil")
	s.Require().NoError(err)
	s.Equal("Banco do Brasil", b.String())
}

func (s *BankCodeSuite) TestNewBankCode_LookupKeyNormalizesSpaces() {
	b, err := valueobjects.NewBankCode("Banco do Brasil")
	s.Require().NoError(err)
	s.Equal("banco-do-brasil", b.LookupKey())
}

func (s *BankCodeSuite) TestNewBankCode_LookupKeyLowercase() {
	b, err := valueobjects.NewBankCode("NuBank")
	s.Require().NoError(err)
	s.Equal("nubank", b.LookupKey())
	s.Equal("NuBank", b.String())
}

func (s *BankCodeSuite) TestNewBankCode_AccentRemoval() {
	b, err := valueobjects.NewBankCode("Itaú")
	s.Require().NoError(err)
	s.Equal("itau", b.LookupKey())
	s.Equal("Itaú", b.String())
}

func (s *BankCodeSuite) TestNewBankCode_C6Bank() {
	b, err := valueobjects.NewBankCode("C6 Bank")
	s.Require().NoError(err)
	s.Equal("c6-bank", b.LookupKey())
	s.Equal("C6 Bank", b.String())
}

func (s *BankCodeSuite) TestNewBankCode_TrimsLeadingTrailingSpaces() {
	b, err := valueobjects.NewBankCode("  Nubank  ")
	s.Require().NoError(err)
	s.Equal("Nubank", b.String())
	s.Equal("nubank", b.LookupKey())
}

func (s *BankCodeSuite) TestNewBankCode_Empty_ReturnsErrInvalidBank() {
	_, err := valueobjects.NewBankCode("")
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidBank)
}

func (s *BankCodeSuite) TestNewBankCode_OnlyWhitespace_ReturnsErrInvalidBank() {
	_, err := valueobjects.NewBankCode("   ")
	s.Require().Error(err)
	s.ErrorIs(err, domain.ErrInvalidBank)
}

func (s *BankCodeSuite) TestNewBankCode_Nubank_Variants_SameLookupKey() {
	variants := []string{"nubank", "Nubank", "NuBank", "NUBANK"}
	for _, v := range variants {
		b, err := valueobjects.NewBankCode(v)
		s.Require().NoError(err, "variant: %q", v)
		s.Equal("nubank", b.LookupKey(), "variant: %q", v)
	}
}

func (s *BankCodeSuite) TestNewBankCode_SantanderAccent() {
	b, err := valueobjects.NewBankCode("Santander")
	s.Require().NoError(err)
	s.Equal("santander", b.LookupKey())
}

func (s *BankCodeSuite) TestNewBankCode_Inter() {
	b, err := valueobjects.NewBankCode("Inter")
	s.Require().NoError(err)
	s.Equal("inter", b.LookupKey())
}
