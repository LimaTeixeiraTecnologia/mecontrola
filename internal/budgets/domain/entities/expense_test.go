package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseSuite struct {
	suite.Suite
	now    time.Time
	source valueobjects.ProducerSource
	extID  valueobjects.ExternalTransactionID
	comp   valueobjects.Competence
}

func TestExpenseSuite(t *testing.T) {
	suite.Run(t, new(ExpenseSuite))
}

func (s *ExpenseSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	src, _ := valueobjects.NewProducerSource("api")
	s.source = src
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	s.extID = ext
	c, _ := valueobjects.NewCompetence("2025-06")
	s.comp = c
}

func (s *ExpenseSuite) newExpense() entities.Expense {
	e, err := entities.NewExpense(
		uuid.New(),
		s.source,
		s.extID,
		uuid.New(),
		valueobjects.RootSlugCustoFixo,
		s.comp,
		1000,
		s.now,
		s.now,
	)
	s.Require().NoError(err)
	return e
}

func (s *ExpenseSuite) TestNewExpense_RF25b_RejectsZeroAmount() {
	_, err := entities.NewExpense(
		uuid.New(),
		s.source,
		s.extID,
		uuid.New(),
		valueobjects.RootSlugCustoFixo,
		s.comp,
		0,
		s.now,
		s.now,
	)
	s.ErrorIs(err, entities.ErrExpenseInvalidAmount)
}

func (s *ExpenseSuite) TestNewExpense_RF25b_RejectsNegativeAmount() {
	_, err := entities.NewExpense(
		uuid.New(),
		s.source,
		s.extID,
		uuid.New(),
		valueobjects.RootSlugCustoFixo,
		s.comp,
		-1,
		s.now,
		s.now,
	)
	s.ErrorIs(err, entities.ErrExpenseInvalidAmount)
}

func (s *ExpenseSuite) TestEdit_RF25b_RejectsZeroAmount() {
	e := s.newExpense()
	err := e.Edit(uuid.New(), valueobjects.RootSlugCustoFixo, s.comp, 0, s.now, 1, s.now.Add(time.Hour))
	s.ErrorIs(err, entities.ErrExpenseInvalidAmount)
}

func (s *ExpenseSuite) TestEdit_RF25b_RejectsNegativeAmount() {
	e := s.newExpense()
	err := e.Edit(uuid.New(), valueobjects.RootSlugCustoFixo, s.comp, -100, s.now, 1, s.now.Add(time.Hour))
	s.ErrorIs(err, entities.ErrExpenseInvalidAmount)
}

func (s *ExpenseSuite) TestNewExpenseVersion1() {
	e := s.newExpense()
	s.Equal(int64(1), e.Version())
	s.Nil(e.TombstoneVersion())
	s.Nil(e.DeletedAt())
	s.False(e.IsDeleted())
}

func (s *ExpenseSuite) TestEdit() {
	type testCase struct {
		name            string
		expectedVersion int64
		wantErr         bool
		errTarget       error
	}

	cases := []testCase{
		{name: "edita com versão correta", expectedVersion: 1, wantErr: false},
		{name: "falha com versão errada", expectedVersion: 2, wantErr: true, errTarget: entities.ErrExpenseVersionMismatch},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			e := s.newExpense()
			newComp, _ := valueobjects.NewCompetence("2025-07")
			err := e.Edit(uuid.New(), valueobjects.RootSlugMetas, newComp, 2000, s.now, tc.expectedVersion, s.now)
			if tc.wantErr {
				s.Error(err)
				s.ErrorIs(err, tc.errTarget)
				return
			}
			s.NoError(err)
			s.Equal(int64(2), e.Version())
			s.Equal(int64(2000), e.AmountCents())
		})
	}
}

func (s *ExpenseSuite) TestSoftDelete() {
	type testCase struct {
		name            string
		expectedVersion int64
		wantErr         bool
		errTarget       error
	}

	cases := []testCase{
		{name: "exclui com versão correta", expectedVersion: 1, wantErr: false},
		{name: "falha com versão errada", expectedVersion: 99, wantErr: true, errTarget: entities.ErrExpenseVersionMismatch},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			e := s.newExpense()
			tombV, err := e.SoftDelete(tc.expectedVersion, s.now)
			if tc.wantErr {
				s.Error(err)
				s.ErrorIs(err, tc.errTarget)
				return
			}
			s.NoError(err)
			s.Equal(int64(2), tombV)
			s.Equal(int64(2), e.Version())
			s.NotNil(e.TombstoneVersion())
			s.Equal(int64(2), *e.TombstoneVersion())
			s.True(e.IsDeleted())
		})
	}
}

func (s *ExpenseSuite) TestSoftDeleteTwiceFails() {
	e := s.newExpense()
	_, _ = e.SoftDelete(1, s.now)
	_, err := e.SoftDelete(2, s.now)
	s.ErrorIs(err, entities.ErrExpenseAlreadyDeleted)
}
