package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type CreateCardSuite struct {
	suite.Suite
	obs            observability.Observability
	ctx            context.Context
	uowMock        *ucmocks.UnitOfWorkCard
	factoryMock    *ifacemocks.RepositoryFactory
	repoMock       *ifacemocks.CardRepository
	bankReaderMock *ifacemocks.BankDaysReader
	idemMock       *idemocks.Storage
}

func TestCreateCard(t *testing.T) {
	suite.Run(t, new(CreateCardSuite))
}

func (s *CreateCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = ucmocks.NewUnitOfWorkCard(s.T())
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	s.bankReaderMock = ifacemocks.NewBankDaysReader(s.T())
	s.idemMock = idemocks.NewStorage(s.T())
}

func (s *CreateCardSuite) makeInput() input.CreateCard {
	return input.CreateCard{
		UserID:   uuid.New(),
		Nickname: "Nu",
		Bank:     "nubank",
		DueDay:   22,
	}
}

func (s *CreateCardSuite) TestExecute_HappyPath() {
	type dependencies struct {
		factory    *ifacemocks.RepositoryFactory
		repo       *ifacemocks.CardRepository
		bankReader *ifacemocks.BankDaysReader
		idem       *idemocks.Storage
	}

	scenarios := []struct {
		name         string
		args         input.CreateCard
		dependencies dependencies
		expect       func(out interface{}, err error)
	}{
		{
			name: "deve criar cartao com sucesso",
			args: s.makeInput(),
			dependencies: dependencies{
				factory: func() *ifacemocks.RepositoryFactory {
					s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
					s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
					return s.factoryMock
				}(),
				repo: func() *ifacemocks.CardRepository {
					s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(nil).Once()
					return s.repoMock
				}(),
				bankReader: func() *ifacemocks.BankDaysReader {
					s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
					return s.bankReaderMock
				}(),
				idem: s.idemMock,
			},
			expect: func(out interface{}, err error) {
				s.Require().NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewCreateCard(s.uowMock, scenario.dependencies.factory, scenario.dependencies.idem, s.obs)
			out, err := sut.Execute(s.ctx, scenario.args)
			scenario.expect(out, err)
		})
	}
}

func (s *CreateCardSuite) TestExecute_ValidationError() {
	type dependencies struct {
		factory *ifacemocks.RepositoryFactory
		idem    *idemocks.Storage
	}

	scenarios := []struct {
		name         string
		args         input.CreateCard
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "banco vazio retorna erro de validacao",
			args: input.CreateCard{
				UserID:   uuid.New(),
				Nickname: "Nu",
				Bank:     "",
				DueDay:   22,
			},
			dependencies: dependencies{
				factory: s.factoryMock,
				idem:    s.idemMock,
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardBankRequired)
			},
		},
		{
			name: "due_day invalido retorna erro de validacao",
			args: input.CreateCard{
				UserID:   uuid.New(),
				Nickname: "Nu",
				Bank:     "nubank",
				DueDay:   0,
			},
			dependencies: dependencies{
				factory: s.factoryMock,
				idem:    s.idemMock,
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardDueDayInvalid)
			},
		},
		{
			name: "user_id vazio retorna erro de validacao",
			args: input.CreateCard{
				UserID:   uuid.Nil,
				Nickname: "Nu",
				Bank:     "nubank",
				DueDay:   22,
			},
			dependencies: dependencies{
				factory: s.factoryMock,
				idem:    s.idemMock,
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().ErrorIs(err, input.ErrCardUserIDRequired)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewCreateCard(s.uowMock, scenario.dependencies.factory, scenario.dependencies.idem, s.obs)
			_, err := sut.Execute(s.ctx, scenario.args)
			scenario.expect(err)
		})
	}
}

func (s *CreateCardSuite) TestExecute_WithIdempotency() {
	in := s.makeInput()
	ic := idempotency.IdempotencyContext{
		Scope:       "card",
		Key:         "key-001",
		UserID:      in.UserID.String(),
		RequestHash: "abc123",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	ctx := idempotency.WithContext(s.ctx, ic)

	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(nil).Once()
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(nil).Once()

	sut := NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().NoError(err)
}

func (s *CreateCardSuite) TestExecute_NicknameConflict() {
	in := s.makeInput()

	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(domain.ErrNicknameConflict).Once()

	sut := NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrNicknameConflict)
}

func (s *CreateCardSuite) TestExecute_RepositoryError() {
	in := s.makeInput()
	repoErr := errors.New("db error")

	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(repoErr).Once()

	sut := NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "db error")
}

func (s *CreateCardSuite) TestExecute_IdempotencyPutErrorCausesRollback() {
	in := s.makeInput()
	ic := idempotency.IdempotencyContext{
		Scope:       "card",
		Key:         "key-rollback",
		UserID:      in.UserID.String(),
		RequestHash: "hash-xyz",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	ctx := idempotency.WithContext(s.ctx, ic)

	idemErr := errors.New("idempotency storage unavailable")

	insertCount := 0
	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).
		RunAndReturn(func(ctx context.Context, c entities.Card) error {
			insertCount++
			return nil
		}).Once()
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(idemErr).Once()

	sut := NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "idempotency storage unavailable")
	s.Equal(1, insertCount)
}
