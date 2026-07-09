package binding

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	agentsifaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	cardifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	cardusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	cardentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	uowmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow/mocks"
	idemmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type CardManagerAdapterSuite struct {
	suite.Suite
	ctx          context.Context
	userID       uuid.UUID
	cardRepo     *cardifacemocks.CardRepository
	bankDaysMock *cardifacemocks.BankDaysReader
	factory      *cardifacemocks.RepositoryFactory
	uow          *uowmocks.UnitOfWork
	idem         *idemmocks.Storage
}

func TestCardManagerAdapterSuite(t *testing.T) {
	suite.Run(t, new(CardManagerAdapterSuite))
}

func (s *CardManagerAdapterSuite) SetupTest() {
	s.ctx = context.Background()
	s.userID = uuid.New()
	s.cardRepo = cardifacemocks.NewCardRepository(s.T())
	s.bankDaysMock = cardifacemocks.NewBankDaysReader(s.T())
	s.factory = cardifacemocks.NewRepositoryFactory(s.T())
	s.uow = uowmocks.NewUnitOfWork(s.T())
	s.idem = idemmocks.NewStorage(s.T())
}

func (s *CardManagerAdapterSuite) buildAdapter() agentsifaces.CardManager {
	o11y := fake.NewProvider()
	createCard := cardusecases.NewCreateCard(s.uow, s.factory, s.idem, o11y)
	listCards := cardusecases.NewListCards(s.cardRepo, o11y)
	isBankRecognized := cardusecases.NewIsBankRecognized(s.factory, nil, o11y)
	return NewCardManagerAdapter(createCard, listCards, nil, nil, nil, nil, nil, nil, nil, isBankRecognized, o11y)
}

func (s *CardManagerAdapterSuite) TestCreateCard_NicknameConflict_ReturnsError() {
	s.uow.EXPECT().
		Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
			return fn(ctx, nil)
		}).
		Once()
	s.factory.EXPECT().BankDaysReader(mock.Anything).Return(s.bankDaysMock).Once()
	s.bankDaysMock.EXPECT().
		DaysBeforeDue(mock.Anything, mock.Anything).
		Return(7, nil).
		Once()
	s.factory.EXPECT().CardRepository(mock.Anything).Return(s.cardRepo).Once()
	s.cardRepo.EXPECT().
		Insert(mock.Anything, mock.AnythingOfType("entities.Card")).
		Return(carddomain.ErrNicknameConflict).
		Once()

	adapter := s.buildAdapter()
	ref, err := adapter.CreateCard(s.ctx, agentsifaces.NewCard{
		UserID:   s.userID,
		Nickname: "Nu",
		Bank:     "Nubank",
		DueDay:   1,
	})

	s.Error(err)
	s.True(errors.Is(err, carddomain.ErrNicknameConflict))
	s.Empty(ref.ID)
}

func (s *CardManagerAdapterSuite) TestCreateCard_PropagatesClosingDayProvided() {
	s.uow.EXPECT().
		Do(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
			return fn(ctx, nil)
		}).
		Once()
	s.factory.EXPECT().CardRepository(mock.Anything).Return(s.cardRepo).Once()
	s.cardRepo.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(c cardentities.Card) bool {
			return c.Cycle.ClosingDay == 20
		})).
		Return(nil).
		Once()

	adapter := s.buildAdapter()
	ref, err := adapter.CreateCard(s.ctx, agentsifaces.NewCard{
		UserID:             s.userID,
		Nickname:           "Nu",
		Bank:               "Banco Exótico",
		DueDay:             10,
		ClosingDay:         20,
		ClosingDayProvided: true,
	})

	s.NoError(err)
	s.NotEmpty(ref.ID)
}

func (s *CardManagerAdapterSuite) TestBankRecognized_DelegatesToIsBankRecognizedRead() {
	s.factory.EXPECT().BankDaysReader(mock.Anything).Return(s.bankDaysMock).Once()
	s.bankDaysMock.EXPECT().
		IsBankRecognized(mock.Anything, mock.Anything).
		Return(true, nil).
		Once()

	adapter := s.buildAdapter()
	recognized, err := adapter.BankRecognized(s.ctx, "Nubank")

	s.NoError(err)
	s.True(recognized)
}

func (s *CardManagerAdapterSuite) TestBankRecognized_ReadError_ReturnsWrappedError() {
	s.factory.EXPECT().BankDaysReader(mock.Anything).Return(s.bankDaysMock).Once()
	s.bankDaysMock.EXPECT().
		IsBankRecognized(mock.Anything, mock.Anything).
		Return(false, errors.New("falha no banco")).
		Once()

	adapter := s.buildAdapter()
	recognized, err := adapter.BankRecognized(s.ctx, "Nubank")

	s.Error(err)
	s.False(recognized)
}
