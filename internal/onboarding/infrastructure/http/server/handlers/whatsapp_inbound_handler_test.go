package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
)

type stubConsumeUC struct {
	result usecases.ConsumeResult
	err    error
	called bool
}

func (s *stubConsumeUC) Execute(_ context.Context, _ input.ConsumeMagicTokenInput) (usecases.ConsumeResult, error) {
	s.called = true
	return s.result, s.err
}

type stubFallbackUC struct {
	result usecases.FallbackResult
	err    error
	called bool
}

func (s *stubFallbackUC) Execute(_ context.Context, _ string) (usecases.FallbackResult, error) {
	s.called = true
	return s.result, s.err
}

type stubWAGateway struct {
	sendErr error
	sent    []string
}

func (g *stubWAGateway) SendActivationTemplate(_ context.Context, toE164, _, _ string) (string, error) {
	return "wamid-1", nil
}

func (g *stubWAGateway) SendTextMessage(_ context.Context, toE164, _ string) error {
	g.sent = append(g.sent, toE164)
	return g.sendErr
}

type stubMetaMessageRepo struct {
	inserted bool
	err      error
}

func (r *stubMetaMessageRepo) InsertIfAbsent(_ context.Context, _ string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	if r.inserted {
		return false, nil
	}
	r.inserted = true
	return true, nil
}

type stubRepoFactory struct {
	metaRepo appinterfaces.MetaMessageRepository
}

func (f *stubRepoFactory) MagicTokenRepository(_ database.DBTX) appinterfaces.MagicTokenRepository {
	return nil
}
func (f *stubRepoFactory) SupportSignalRepository(_ database.DBTX) appinterfaces.SupportSignalRepository {
	return nil
}
func (f *stubRepoFactory) MetaMessageRepository(_ database.DBTX) appinterfaces.MetaMessageRepository {
	return f.metaRepo
}
func (f *stubRepoFactory) OnboardingCleanupRepository(_ database.DBTX) appinterfaces.OnboardingCleanupRepository {
	return nil
}

func buildMetaPayload(from, wamid, text string) string {
	body := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "entry-1",
				"changes": []map[string]any{
					{
						"field": "messages",
						"value": map[string]any{
							"messaging_product": "whatsapp",
							"messages": []map[string]any{
								{
									"from":      from,
									"id":        wamid,
									"timestamp": "1686000000",
									"type":      "text",
									"text":      map[string]any{"body": text},
								},
							},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(body)
	return string(b)
}

type WhatsAppInboundHandlerSuite struct {
	suite.Suite
}

func TestWhatsAppInboundHandler(t *testing.T) {
	suite.Run(t, new(WhatsAppInboundHandlerSuite))
}

func (s *WhatsAppInboundHandlerSuite) buildHandler(
	consumeUC *stubConsumeUC,
	fallbackUC *stubFallbackUC,
	waGW *stubWAGateway,
	metaRepo *stubMetaMessageRepo,
) *handlers.WhatsAppInboundHandler {
	factory := &stubRepoFactory{metaRepo: metaRepo}
	msgs := map[string]string{
		"welcome_activated":               "Bem-vindo!",
		"already_active":                  "Conta já ativa.",
		"code_already_used_other_account": "Código usado por outra conta.",
		"payment_still_processing_retry":  "Pagamento em processamento.",
		"code_expired_contact_support":    "Código expirado.",
		"code_invalid_check_again":        "Código inválido.",
		"system_unavailable_retry":        "Sistema indisponível.",
		"please_use_ativar_command":       "Use ATIVAR <código>.",
		"invalid_country":                 "Número não suportado.",
	}
	return handlers.NewWhatsAppInboundHandler(
		consumeUC,
		fallbackUC,
		waGW,
		factory,
		nil,
		msgs,
		noop.NewProvider(),
	)
}

func (s *WhatsAppInboundHandlerSuite) TestAtivarCommand_CallsConsumeUseCase() {
	token := "abcdefghijklmnopqrstuvwxyz0123456789abcde"
	consumeUC := &stubConsumeUC{result: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeActivated}}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("5511999999999", "wamid-001", fmt.Sprintf("ATIVAR %s", token))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.True(consumeUC.called)
	s.False(fallbackUC.called)
}

func (s *WhatsAppInboundHandlerSuite) TestAtivarCommand_CaseInsensitive() {
	token := "abcdefghijklmnopqrstuvwxyz0123456789abcde"
	consumeUC := &stubConsumeUC{result: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeActivated}}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("5511999999999", "wamid-002", fmt.Sprintf("ativar %s", token))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.True(consumeUC.called)
}

func (s *WhatsAppInboundHandlerSuite) TestNonAtivarMessage_CallsFallback() {
	consumeUC := &stubConsumeUC{}
	fallbackUC := &stubFallbackUC{result: usecases.FallbackResult{Outcome: usecases.FallbackOutcomeNoMatch}}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("5511999999999", "wamid-003", "Olá, preciso de ajuda")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.False(consumeUC.called)
	s.True(fallbackUC.called)
}

func (s *WhatsAppInboundHandlerSuite) TestUnsupportedCountry_DoesNotDispatchUseCases() {
	consumeUC := &stubConsumeUC{}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("12125551234", "wamid-us", "Olá")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.False(consumeUC.called)
	s.False(fallbackUC.called)
	s.Len(waGW.sent, 1)
}

func (s *WhatsAppInboundHandlerSuite) TestDuplicateWAMID_NoDispatch() {
	consumeUC := &stubConsumeUC{}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{inserted: true}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("5511999999999", "wamid-dupe", "ATIVAR abcdefghijklmnopqrstuvwxyz0123456789abcde")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.False(consumeUC.called)
	s.False(fallbackUC.called)
}

func (s *WhatsAppInboundHandlerSuite) TestEmptyPayload_ReturnsOK() {
	consumeUC := &stubConsumeUC{}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.False(consumeUC.called)
}

func (s *WhatsAppInboundHandlerSuite) TestConsumeAlreadyActive_SendsCorrectMessage() {
	token := "abcdefghijklmnopqrstuvwxyz0123456789abcde"
	consumeUC := &stubConsumeUC{result: usecases.ConsumeResult{Outcome: usecases.ConsumeOutcomeAlreadyActive}}
	fallbackUC := &stubFallbackUC{}
	waGW := &stubWAGateway{}
	metaRepo := &stubMetaMessageRepo{}

	handler := s.buildHandler(consumeUC, fallbackUC, waGW, metaRepo)
	body := buildMetaPayload("5511999999999", "wamid-active", fmt.Sprintf("ATIVAR %s", token))
	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
	s.Len(waGW.sent, 1)
}
