package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
)

type EvaluateInvoiceDueAlertsSuite struct {
	suite.Suite
	obs       observability.Observability
	ctx       context.Context
	factory   *mockInterfaces.RepositoryFactory
	cardRepo  *mockInterfaces.CardRepository
	sentRepo  *mockInterfaces.InvoiceDueAlertSentRepository
	publisher *mockInterfaces.InvoiceDuePublisher
	uow       *uowMocks.UnitOfWorkVoid
	useCase   *EvaluateInvoiceDueAlerts
}

func TestEvaluateInvoiceDueAlertsSuite(t *testing.T) {
	suite.Run(t, new(EvaluateInvoiceDueAlertsSuite))
}

func (s *EvaluateInvoiceDueAlertsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.cardRepo = mockInterfaces.NewCardRepository(s.T())
	s.sentRepo = mockInterfaces.NewInvoiceDueAlertSentRepository(s.T())
	s.publisher = mockInterfaces.NewInvoiceDuePublisher(s.T())
	s.factory.EXPECT().CardRepository(mock.Anything).Return(s.cardRepo).Maybe()
	s.factory.EXPECT().InvoiceDueAlertSentRepository(mock.Anything).Return(s.sentRepo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())

	s.useCase = NewEvaluateInvoiceDueAlerts(
		s.factory,
		s.publisher,
		s.uow,
		time.UTC,
		3,
		100,
		s.obs,
	)
}

func (s *EvaluateInvoiceDueAlertsSuite) buildCardDueInDays(days int) entities.Card {
	due := time.Now().UTC().AddDate(0, 0, days)
	cycle, err := valueobjects.NewBillingCycle(1, due.Day())
	s.Require().NoError(err)
	name, err := valueobjects.NewCardName("Cartao Teste")
	s.Require().NoError(err)
	nick, err := valueobjects.NewNickname("teste")
	s.Require().NoError(err)
	return entities.HydrateCardWithVersion(
		uuid.New(),
		uuid.New(),
		name,
		nick,
		cycle,
		500000,
		1,
		time.Now().UTC(),
		time.Now().UTC(),
		nil,
	)
}

func (s *EvaluateInvoiceDueAlertsSuite) TestExecute_NoCards_NoOp() {
	s.cardRepo.EXPECT().
		FindCardsWithInvoiceDueWithin(mock.Anything, 3, 100).
		Return(nil, nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateInvoiceDueAlertsSuite) TestExecute_DueInThreeDays_PublishesOneEvent() {
	card := s.buildCardDueInDays(3)

	s.cardRepo.EXPECT().
		FindCardsWithInvoiceDueWithin(mock.Anything, 3, 100).
		Return([]entities.Card{card}, nil).
		Once()
	s.sentRepo.EXPECT().
		ListSentForDueDates(mock.Anything, mock.Anything).
		Return(nil, nil).
		Once()
	s.publisher.EXPECT().
		Publish(mock.Anything, mock.Anything, mock.MatchedBy(func(a services.InvoiceDueAlert) bool {
			return a.UserID == card.UserID &&
				a.CardID == card.ID &&
				a.LimitCents == card.LimitCents &&
				a.DaysUntil >= 0 && a.DaysUntil <= 3
		}), mock.Anything).
		Return(nil).
		Once()
	s.sentRepo.EXPECT().
		InsertSent(mock.Anything, card.UserID, card.ID, mock.Anything).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateInvoiceDueAlertsSuite) TestExecute_AlreadySent_PublishesZero() {
	card := s.buildCardDueInDays(3)
	dueDate := nextDue(card.Cycle.DueDay)

	s.cardRepo.EXPECT().
		FindCardsWithInvoiceDueWithin(mock.Anything, 3, 100).
		Return([]entities.Card{card}, nil).
		Once()
	s.sentRepo.EXPECT().
		ListSentForDueDates(mock.Anything, mock.Anything).
		Return([]interfaces.InvoiceDueAlertSentRecord{
			{UserID: card.UserID, CardID: card.ID, RefDueDate: dueDate},
		}, nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func nextDue(dueDay int) time.Time {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	candidate := time.Date(now.Year(), now.Month(), dueDay, 0, 0, 0, 0, time.UTC)
	if candidate.Before(start) {
		candidate = candidate.AddDate(0, 1, 0)
	}
	return candidate.Truncate(24 * time.Hour)
}
