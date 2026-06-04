package server_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
	billinghttp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/server"
)

// stubIngestUseCase implementa ingestWebhookExecutor para testes.
type stubIngestUseCase struct {
	result    output.IngestWebhookResult
	err       error
	lastInput input.IngestWebhookInput
}

func (s *stubIngestUseCase) Execute(_ context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
	s.lastInput = in
	return s.result, s.err
}

type KiwifyWebhookHandlerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestKiwifyWebhookHandler(t *testing.T) {
	suite.Run(t, new(KiwifyWebhookHandlerSuite))
}

func (s *KiwifyWebhookHandlerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *KiwifyWebhookHandlerSuite) buildHandler(stub *stubIngestUseCase) *billinghttp.KiwifyWebhookHandler {
	return billinghttp.NewKiwifyWebhookHandler(stub, slog.Default())
}

type cenario struct {
	nome           string
	stubErr        error
	stubResult     output.IngestWebhookResult
	statusEsperado int
}

func (s *KiwifyWebhookHandlerSuite) TestCenarios() {
	webhookID, _ := valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440000")

	cenarios := []cenario{
		{
			nome:           "sucesso retorna 200",
			stubResult:     output.IngestWebhookResult{Duplicate: false, WebhookEventID: webhookID},
			statusEsperado: http.StatusOK,
		},
		{
			nome:           "duplicata retorna 204",
			stubResult:     output.IngestWebhookResult{Duplicate: true, WebhookEventID: webhookID},
			statusEsperado: http.StatusNoContent,
		},
		{
			nome:           "assinatura inválida retorna 401",
			stubErr:        kiwify.ErrInvalidSignature,
			statusEsperado: http.StatusUnauthorized,
		},
		{
			nome:           "assinatura ausente retorna 401",
			stubErr:        kiwify.ErrMissingSignature,
			statusEsperado: http.StatusUnauthorized,
		},
		{
			nome:           "payload inválido retorna 400",
			stubErr:        kiwify.ErrPayloadDecode,
			statusEsperado: http.StatusBadRequest,
		},
		{
			nome:           "payload json malformado retorna 400",
			stubErr:        valueobjects.ErrMalformedPayload,
			statusEsperado: http.StatusBadRequest,
		},
		{
			nome:           "erro interno retorna 500",
			stubErr:        errors.New("db error"),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	for _, c := range cenarios {
		s.Run(c.nome, func() {
			stub := &stubIngestUseCase{result: c.stubResult, err: c.stubErr}
			handler := s.buildHandler(stub)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/webhooks/kiwify", bytes.NewBufferString(`{"id":"1"}`))

			handler.ServeHTTP(rec, req)

			s.Equal(c.statusEsperado, rec.Code)
		})
	}
}

func (s *KiwifyWebhookHandlerSuite) TestHeaderAuthorizationNaoPropagado() {
	var capturedInput input.IngestWebhookInput
	stub := &capturingStub{fn: func(in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
		capturedInput = in
		webhookID, _ := valueobjects.NewWebhookEventID("550e8400-e29b-41d4-a716-446655440000")
		return output.IngestWebhookResult{WebhookEventID: webhookID}, nil
	}}
	handler := billinghttp.NewKiwifyWebhookHandler(stub, slog.Default())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/kiwify", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Cookie", "session=abc")
	req.Header.Set("X-Custom-Header", "valor")

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	_, hasAuth := capturedInput.Headers["Authorization"]
	s.False(hasAuth, "Authorization header não deve aparecer no input")
	_, hasCookie := capturedInput.Headers["Cookie"]
	s.False(hasCookie, "Cookie header não deve aparecer no input")
}

type capturingStub struct {
	fn func(in input.IngestWebhookInput) (output.IngestWebhookResult, error)
}

func (c *capturingStub) Execute(_ context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
	return c.fn(in)
}
