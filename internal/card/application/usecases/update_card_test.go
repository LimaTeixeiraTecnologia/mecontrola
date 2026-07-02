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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type UpdateCardSuite struct {
	suite.Suite
	obs            observability.Observability
	ctx            context.Context
	uowMock        *ucmocks.UnitOfWorkCard
	factoryMock    *ifacemocks.RepositoryFactory
	repoMock       *ifacemocks.CardRepository
	bankReaderMock *ifacemocks.BankDaysReader
	idemMock       *idemocks.Storage
}

func TestUpdateCard(t *testing.T) {
	suite.Run(t, new(UpdateCardSuite))
}

func (s *UpdateCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = ucmocks.NewUnitOfWorkCard(s.T())
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	s.bankReaderMock = ifacemocks.NewBankDaysReader(s.T())
	s.idemMock = idemocks.NewStorage(s.T())
}

func (s *UpdateCardSuite) existingCard(userID uuid.UUID) entities.Card {
	nick, _ := valueobjects.NewNickname("OldNick")
	bank, _ := valueobjects.NewBankCode("nubank")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), userID, nick, bank, cycle, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *UpdateCardSuite) makeInput(cardID, userID uuid.UUID) input.UpdateCard {
	nick := "NewNick"
	return input.UpdateCard{
		ID:       cardID,
		UserID:   userID,
		Nickname: &nick,
	}
}

func (s *UpdateCardSuite) TestExecute_HappyPath() {
	type dependencies struct {
		factory *ifacemocks.RepositoryFactory
		repo    *ifacemocks.CardRepository
		idem    *idemocks.Storage
	}

	userID := uuid.New()
	existing := s.existingCard(userID)
	in := s.makeInput(existing.ID, userID)

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve atualizar cartao com sucesso (apenas nickname)",
			dependencies: dependencies{
				factory: func() *ifacemocks.RepositoryFactory {
					s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
					return s.factoryMock
				}(),
				repo: func() *ifacemocks.CardRepository {
					s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
					s.repoMock.EXPECT().UpdateByIDForUser(mock.Anything, mock.AnythingOfType("entities.Card")).Return(existing, nil).Once()
					return s.repoMock
				}(),
				idem: s.idemMock,
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewUpdateCard(s.uowMock, scenario.dependencies.factory, scenario.dependencies.idem, s.obs)
			_, err := sut.Execute(s.ctx, in)
			scenario.expect(err)
		})
	}
}

func (s *UpdateCardSuite) TestExecute_UpdateBankAndDueDay() {
	userID := uuid.New()
	existing := s.existingCard(userID)
	bank := "itau"
	dueDay := 25
	in := input.UpdateCard{
		ID:     existing.ID,
		UserID: userID,
		Bank:   &bank,
		DueDay: &dueDay,
	}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.factoryMock.EXPECT().BankDaysReader(mock.Anything).Return(s.bankReaderMock).Once()
	s.bankReaderMock.EXPECT().DaysBeforeDue(mock.Anything, mock.Anything).Return(7, nil).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateByIDForUser(mock.Anything, mock.AnythingOfType("entities.Card")).Return(existing, nil).Once()

	sut := NewUpdateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(s.ctx, in)

	s.Require().NoError(err)
}

func (s *UpdateCardSuite) TestExecute_CardNotFound() {
	userID := uuid.New()
	in := s.makeInput(uuid.New(), userID)

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, in.ID.String(), userID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := NewUpdateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func (s *UpdateCardSuite) TestExecute_NicknameConflict() {
	userID := uuid.New()
	existing := s.existingCard(userID)
	in := s.makeInput(existing.ID, userID)

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateByIDForUser(mock.Anything, mock.AnythingOfType("entities.Card")).Return(entities.Card{}, domain.ErrNicknameConflict).Once()

	sut := NewUpdateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(s.ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrNicknameConflict)
}

func (s *UpdateCardSuite) TestExecute_IdempotencyPutRollback() {
	userID := uuid.New()
	existing := s.existingCard(userID)
	in := s.makeInput(existing.ID, userID)

	ic := idempotency.IdempotencyContext{
		Scope:       "card",
		Key:         "key-update",
		UserID:      userID.String(),
		RequestHash: "hash-update",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	ctx := idempotency.WithContext(s.ctx, ic)

	idemErr := errors.New("storage unavailable")

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateByIDForUser(mock.Anything, mock.AnythingOfType("entities.Card")).Return(existing, nil).Once()
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(idemErr).Once()

	sut := NewUpdateCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "storage unavailable")
}
