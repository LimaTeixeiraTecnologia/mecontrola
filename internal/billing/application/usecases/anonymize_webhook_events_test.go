package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	billingmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability/fakes"
)

const (
	_anonWebhookUUID1 = "550e8400-e29b-41d4-a716-446655440040"
	_anonWebhookUUID2 = "550e8400-e29b-41d4-a716-446655440041"
)

type AnonymizeWebhookEventsSuite struct {
	suite.Suite

	ctx         context.Context
	now         time.Time
	webhookRepo *billingmocks.WebhookEventRepository
	redactor    *billingmocks.PIIRedactor
}

func TestAnonymizeWebhookEventsSuite(t *testing.T) {
	suite.Run(t, new(AnonymizeWebhookEventsSuite))
}

func (s *AnonymizeWebhookEventsSuite) SetupTest() {
	s.ctx = context.Background()
	s.now = time.Now().UTC()
	s.webhookRepo = billingmocks.NewWebhookEventRepository(s.T())
	s.redactor = billingmocks.NewPIIRedactor(s.T())
}

func (s *AnonymizeWebhookEventsSuite) buildUC() *usecases.AnonymizeWebhookEventsUseCase {
	return usecases.NewAnonymizeWebhookEventsUseCase(s.webhookRepo, s.redactor, fakes.NoopObservability(), fakes.NoopUsecaseMetrics())
}

func (s *AnonymizeWebhookEventsSuite) buildInput() input.AnonymizeInput {
	return input.AnonymizeInput{
		OlderThan: s.now.Add(-30 * 24 * time.Hour),
		BatchSize: 100,
	}
}

func (s *AnonymizeWebhookEventsSuite) buildWebhookEvent(uuidStr string) entities.WebhookEvent {
	now := s.now
	webhookID, _ := valueobjects.NewWebhookEventID(uuidStr)
	rawBody := json.RawMessage(`{"id":"ext-` + uuidStr[:8] + `","webhook_event_type":"compra_aprovada"}`)
	externalID, _ := valueobjects.NewExternalEventIDCascade(rawBody)
	evt, _ := entities.NewWebhookEvent(entities.NewWebhookEventParams{
		ID:              webhookID,
		Provider:        "kiwify",
		ExternalEventID: externalID,
		EventType:       "compra_aprovada",
		Payload:         json.RawMessage(`{"name":"John","email":"john@example.com"}`),
		ReceivedAt:      now.Add(-35 * 24 * time.Hour),
	})
	return evt
}

func (s *AnonymizeWebhookEventsSuite) TestExecute() {
	in := s.buildInput()

	type setupFn func()
	scenarios := []struct {
		name   string
		setup  setupFn
		expect func(err error, processed, errCount int)
	}{
		{
			name: "batch vazio: nenhum evento pendente → report zero",
			setup: func() {
				s.webhookRepo.EXPECT().
					ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
					Return(nil, nil).Once()
			},
			expect: func(err error, processed, errCount int) {
				s.NoError(err)
				s.Equal(0, processed)
				s.Equal(0, errCount)
			},
		},
		{
			name: "batch normal: 2 eventos processados com sucesso",
			setup: func() {
				evt1 := s.buildWebhookEvent(_anonWebhookUUID1)
				evt2 := s.buildWebhookEvent(_anonWebhookUUID2)
				redacted := json.RawMessage(`{"name":"***","email":"***"}`)
				s.webhookRepo.EXPECT().
					ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
					Return([]entities.WebhookEvent{evt1, evt2}, nil).Once()
				s.redactor.EXPECT().
					Strip(evt1.Payload()).
					Return(redacted, nil).Once()
				s.webhookRepo.EXPECT().
					Anonymize(mock.Anything, evt1.ID(), redacted, mock.AnythingOfType("time.Time")).
					Return(nil).Once()
				s.redactor.EXPECT().
					Strip(evt2.Payload()).
					Return(redacted, nil).Once()
				s.webhookRepo.EXPECT().
					Anonymize(mock.Anything, evt2.ID(), redacted, mock.AnythingOfType("time.Time")).
					Return(nil).Once()
			},
			expect: func(err error, processed, errCount int) {
				s.NoError(err)
				s.Equal(2, processed)
				s.Equal(0, errCount)
			},
		},
		{
			name: "redactor falha em 1 evento: não aborta o batch, conta como erro",
			setup: func() {
				evt1 := s.buildWebhookEvent(_anonWebhookUUID1)
				evt2 := s.buildWebhookEvent(_anonWebhookUUID2)
				redacted := json.RawMessage(`{"name":"***","email":"***"}`)
				s.webhookRepo.EXPECT().
					ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
					Return([]entities.WebhookEvent{evt1, evt2}, nil).Once()
				s.redactor.EXPECT().
					Strip(evt1.Payload()).
					Return(nil, errors.New("strip error")).Once()
				// evt1 falhou → não chama Anonymize para evt1.
				s.redactor.EXPECT().
					Strip(evt2.Payload()).
					Return(redacted, nil).Once()
				s.webhookRepo.EXPECT().
					Anonymize(mock.Anything, evt2.ID(), redacted, mock.AnythingOfType("time.Time")).
					Return(nil).Once()
			},
			expect: func(err error, processed, errCount int) {
				s.NoError(err)
				s.Equal(1, processed)
				s.Equal(1, errCount)
			},
		},
		{
			name: "Anonymize falha em 1 evento: não aborta o batch, conta como erro",
			setup: func() {
				evt1 := s.buildWebhookEvent(_anonWebhookUUID1)
				evt2 := s.buildWebhookEvent(_anonWebhookUUID2)
				redacted := json.RawMessage(`{"name":"***","email":"***"}`)
				s.webhookRepo.EXPECT().
					ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
					Return([]entities.WebhookEvent{evt1, evt2}, nil).Once()
				s.redactor.EXPECT().
					Strip(evt1.Payload()).
					Return(redacted, nil).Once()
				s.webhookRepo.EXPECT().
					Anonymize(mock.Anything, evt1.ID(), redacted, mock.AnythingOfType("time.Time")).
					Return(errors.New("db timeout")).Once()
				s.redactor.EXPECT().
					Strip(evt2.Payload()).
					Return(redacted, nil).Once()
				s.webhookRepo.EXPECT().
					Anonymize(mock.Anything, evt2.ID(), redacted, mock.AnythingOfType("time.Time")).
					Return(nil).Once()
			},
			expect: func(err error, processed, errCount int) {
				s.NoError(err)
				s.Equal(1, processed)
				s.Equal(1, errCount)
			},
		},
		{
			name: "ListPendingAnonymization falha → retorna erro",
			setup: func() {
				s.webhookRepo.EXPECT().
					ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
					Return(nil, errors.New("db error")).Once()
			},
			expect: func(err error, processed, errCount int) {
				s.Error(err)
				s.Contains(err.Error(), "anonimizar webhooks")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			scenario.setup()
			uc := s.buildUC()
			report, err := uc.Execute(s.ctx, in)
			scenario.expect(err, report.Processed, report.Errors)
		})
	}
}

// TestExecute_AllFail verifica que um batch onde todos os eventos falham retorna Processed=0 e Errors=n.
func (s *AnonymizeWebhookEventsSuite) TestExecute_AllFail() {
	in := s.buildInput()
	evt1 := s.buildWebhookEvent(_anonWebhookUUID1)
	evt2 := s.buildWebhookEvent(_anonWebhookUUID2)

	s.webhookRepo.EXPECT().
		ListPendingAnonymization(mock.Anything, in.OlderThan, in.BatchSize).
		Return([]entities.WebhookEvent{evt1, evt2}, nil).Once()
	s.redactor.EXPECT().
		Strip(mock.AnythingOfType("json.RawMessage")).
		Return(nil, errors.New("strip error")).Times(2)

	uc := s.buildUC()
	report, err := uc.Execute(s.ctx, in)
	s.NoError(err)
	s.Equal(0, report.Processed)
	s.Equal(2, report.Errors)
}
