package postgres

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type BankRepositoryIsRecognizedSuite struct {
	suite.Suite
}

func TestBankRepositoryIsRecognizedSuite(t *testing.T) {
	suite.Run(t, new(BankRepositoryIsRecognizedSuite))
}

func (s *BankRepositoryIsRecognizedSuite) TestIsBankRecognized_NormalizationParity() {
	type args struct {
		raw string
	}
	type expects struct {
		lookupKey  string
		recognized bool
	}

	scenarios := []struct {
		name    string
		args    args
		expects expects
	}{
		{
			name:    "Nubank normaliza para nubank",
			args:    args{raw: "Nubank"},
			expects: expects{lookupKey: "nubank", recognized: true},
		},
		{
			name:    "banco XP normaliza para banco-xp",
			args:    args{raw: "banco XP"},
			expects: expects{lookupKey: "banco-xp", recognized: false},
		},
		{
			name:    "entrada acentuada Itaú normaliza para itau",
			args:    args{raw: "Itaú"},
			expects: expects{lookupKey: "itau", recognized: true},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			bank, err := valueobjects.NewBankCode(scenario.args.raw)
			s.Require().NoError(err)
			s.Equal(scenario.expects.lookupKey, bank.LookupKey())

			db, mockDB, err := sqlmock.New()
			s.Require().NoError(err)
			defer func() { _ = db.Close() }()

			mockDB.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM mecontrola.banks WHERE code = \\$1\\)").
				WithArgs(scenario.expects.lookupKey).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(scenario.expects.recognized))

			repo := NewBankRepository(noop.NewProvider(), db)
			recognized, err := repo.IsBankRecognized(context.Background(), bank)

			require.NoError(s.T(), err)
			s.Equal(scenario.expects.recognized, recognized)
			s.NoError(mockDB.ExpectationsWereMet())
		})
	}
}
