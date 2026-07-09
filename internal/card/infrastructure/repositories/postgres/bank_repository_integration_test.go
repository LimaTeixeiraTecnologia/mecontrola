//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	cardrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type BankRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestBankRepositorySuite(t *testing.T) {
	suite.Run(t, new(BankRepositorySuite))
}

func (s *BankRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = cardrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *BankRepositorySuite) newReader() interfaces.BankDaysReader {
	return s.factory.BankDaysReader(s.db)
}

func (s *BankRepositorySuite) TestDaysBeforeDue() {
	type args struct {
		raw string
	}
	type expects struct {
		days int
	}

	scenarios := []struct {
		name   string
		args   args
		expect expects
	}{
		{
			name:   "nubank deve retornar 7 dias",
			args:   args{raw: "Nubank"},
			expect: expects{days: 7},
		},
		{
			name:   "itau deve retornar 8 dias",
			args:   args{raw: "Itaú"},
			expect: expects{days: 8},
		},
		{
			name:   "banco desconhecido deve retornar fallback 7",
			args:   args{raw: "Banco Desconhecido XYZ"},
			expect: expects{days: 7},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			bank, err := valueobjects.NewBankCode(scenario.args.raw)
			s.Require().NoError(err)

			reader := s.newReader()
			days, err := reader.DaysBeforeDue(context.Background(), bank)

			s.NoError(err)
			s.Equal(scenario.expect.days, days)
		})
	}
}

func (s *BankRepositorySuite) TestIsBankRecognized() {
	type args struct {
		raw string
	}
	type expects struct {
		recognized bool
	}

	scenarios := []struct {
		name   string
		args   args
		expect expects
	}{
		{
			name:   "Nubank deve ser reconhecido",
			args:   args{raw: "Nubank"},
			expect: expects{recognized: true},
		},
		{
			name:   "Itaú com acento deve ser reconhecido",
			args:   args{raw: "Itaú"},
			expect: expects{recognized: true},
		},
		{
			name:   "banco XP nao deve ser reconhecido",
			args:   args{raw: "banco XP"},
			expect: expects{recognized: false},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			bank, err := valueobjects.NewBankCode(scenario.args.raw)
			s.Require().NoError(err)

			reader := s.newReader()
			recognized, err := reader.IsBankRecognized(context.Background(), bank)

			s.NoError(err)
			s.Equal(scenario.expect.recognized, recognized)
		})
	}
}
