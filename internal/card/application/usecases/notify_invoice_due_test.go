package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	cardinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type recordingGateway struct {
	channel    string
	externalID string
	text       string
	calls      int
}

func (g *recordingGateway) SendText(_ context.Context, channel, externalID, text string) error {
	g.calls++
	g.channel = channel
	g.externalID = externalID
	g.text = text
	return nil
}

func (g *recordingGateway) SendActivationTemplate(_ context.Context, _, _, _, _ string) (string, error) {
	return "", nil
}

type NotifyInvoiceDueSuite struct {
	suite.Suite
	ctx      context.Context
	sentRepo *mockInterfaces.InvoiceDueAlertSentRepository
	resolver *mockInterfaces.UserChannelResolver
	gateway  *recordingGateway
	useCase  *usecases.NotifyInvoiceDue
}

func TestNotifyInvoiceDueSuite(t *testing.T) {
	suite.Run(t, new(NotifyInvoiceDueSuite))
}

func (s *NotifyInvoiceDueSuite) SetupTest() {
	s.ctx = context.Background()
	s.sentRepo = mockInterfaces.NewInvoiceDueAlertSentRepository(s.T())
	s.resolver = mockInterfaces.NewUserChannelResolver(s.T())
	s.gateway = &recordingGateway{}

	s.useCase = usecases.NewNotifyInvoiceDue(
		s.sentRepo,
		s.resolver,
		s.gateway,
		time.UTC,
		noop.NewProvider(),
	)
}

func (s *NotifyInvoiceDueSuite) TestExecute_SendsTextWithCorrectContent() {
	userID := uuid.New()
	cardID := uuid.New()
	dueDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	s.sentRepo.EXPECT().
		IsNotified(s.ctx, userID, cardID, dueDate).
		Return(false, nil).
		Once()
	s.resolver.EXPECT().
		ResolvePreferred(s.ctx, userID).
		Return(cardinterfaces.UserChannelPreference{Channel: notification.ChannelWhatsApp, ExternalID: "5511999999999"}, true, nil).
		Once()
	s.sentRepo.EXPECT().
		MarkNotified(s.ctx, userID, cardID, dueDate, notification.ChannelWhatsApp, mock.Anything).
		Return(true, nil).
		Once()

	in := usecases.NotifyInvoiceDueInput{
		UserID:     userID,
		CardID:     cardID,
		CardName:   "Nubank",
		LimitCents: 500000,
		DueDate:    dueDate,
		DaysUntil:  3,
	}

	result, err := s.useCase.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal(usecases.NotifyInvoiceDueOutcomeSent, result.Outcome)
	s.Equal(1, s.gateway.calls)
	s.Equal(notification.ChannelWhatsApp, s.gateway.channel)
	s.Equal("5511999999999", s.gateway.externalID)
	s.Equal("Sua fatura do cartao Nubank vence em 3 dias (10/07). Limite: R$5000,00.", s.gateway.text)
}

func (s *NotifyInvoiceDueSuite) TestExecute_AlreadyNotified_NoSend() {
	userID := uuid.New()
	cardID := uuid.New()
	dueDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

	s.sentRepo.EXPECT().
		IsNotified(s.ctx, userID, cardID, dueDate).
		Return(true, nil).
		Once()

	in := usecases.NotifyInvoiceDueInput{
		UserID:    userID,
		CardID:    cardID,
		CardName:  "Nubank",
		DueDate:   dueDate,
		DaysUntil: 3,
	}

	result, err := s.useCase.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal(usecases.NotifyInvoiceDueOutcomeAlreadySent, result.Outcome)
	s.Equal(0, s.gateway.calls)
}
