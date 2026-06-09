package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type CreateCardSuite struct {
	suite.Suite
	ctx         context.Context
	uowMock     *ucmocks.UnitOfWorkCard
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
	idemMock    *idemocks.Storage
}

func TestCreateCard(t *testing.T) {
	suite.Run(t, new(CreateCardSuite))
}

func (s *CreateCardSuite) SetupTest() {
	s.ctx = context.Background()
	s.uowMock = ucmocks.NewUnitOfWorkCard(s.T())
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	s.idemMock = idemocks.NewStorage(s.T())
}

func (s *CreateCardSuite) makeInput() input.CreateCard {
	return input.CreateCard{
		UserID:     uuid.New(),
		Name:       "Nubank",
		Nickname:   "Nu",
		ClosingDay: 15,
		DueDay:     22,
	}
}

func (s *CreateCardSuite) TestExecute_HappyPath() {
	in := s.makeInput()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(nil).Once()

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	out, err := sut.Execute(s.ctx, in)

	s.Require().NoError(err)
	s.Equal(in.Name, out.Name)
	s.Equal(in.Nickname, out.Nickname)
	s.Equal(in.ClosingDay, out.ClosingDay)
	s.Equal(in.DueDay, out.DueDay)
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

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(nil).Once()
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(nil).Once()

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	out, err := sut.Execute(ctx, in)

	s.Require().NoError(err)
	s.Equal(in.Name, out.Name)
}

func (s *CreateCardSuite) TestExecute_InvalidName() {
	in := s.makeInput()
	in.Name = ""

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidCardName)
}

func (s *CreateCardSuite) TestExecute_InvalidNickname() {
	in := s.makeInput()
	in.Nickname = ""

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidNickname)
}

func (s *CreateCardSuite) TestExecute_InvalidClosingDay() {
	in := s.makeInput()
	in.ClosingDay = 0

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrInvalidClosingDay)
}

func (s *CreateCardSuite) TestExecute_NicknameConflict() {
	in := s.makeInput()
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(domain.ErrNicknameConflict).Once()

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrNicknameConflict)
}

func (s *CreateCardSuite) TestExecute_RepositoryError() {
	in := s.makeInput()
	repoErr := errors.New("db error")
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).Return(repoErr).Once()

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "db error")
}

func (s *CreateCardSuite) TestExecute_RINT05_IdempotencyPutErrorCausesRollback() {
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
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.AnythingOfType("entities.Card")).
		RunAndReturn(func(ctx context.Context, c entities.Card) error {
			insertCount++
			return nil
		}).Once()
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(idemErr).Once()

	sut := usecases.NewCreateCard(s.uowMock, s.factoryMock, s.idemMock, noop.NewProvider())
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "idempotency storage unavailable")
	s.Equal(1, insertCount)
}
