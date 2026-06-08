package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers"
	handlersmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/handlers/mocks"
)

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
	msgs map[string]string
}

func TestWhatsAppInboundHandler(t *testing.T) {
	suite.Run(t, new(WhatsAppInboundHandlerSuite))
}

func (s *WhatsAppInboundHandlerSuite) SetupTest() {
	s.msgs = map[string]string{
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
}

func (s *WhatsAppInboundHandlerSuite) TestWhatsAppInboundHandler_Scenarios() {
	token := "abcdefghijklmnopqrstuvwxyz0123456789abcde"

	scenarios := []struct {
		name                string
		from                string
		wamid               string
		text                string
		consumeOutcome      usecases.ConsumeOutcome
		consumeErr          error
		fallbackOutcome     usecases.FallbackOutcome
		fallbackErr         error
		metaRepoInserted    bool
		metaRepoErr         error
		expectHTTPStatus    int
		expectConsumeCall   bool
		expectFallbackCall  bool
		expectWAGatewaySend bool
		expectMetaRepoCall  bool
		expectRepoFacCall   bool
	}{
		{
			name:                "ATIVAR command calls consume use case",
			from:                "5511999999999",
			wamid:               "wamid-001",
			text:                fmt.Sprintf("ATIVAR %s", token),
			consumeOutcome:      usecases.ConsumeOutcomeActivated,
			expectHTTPStatus:    http.StatusOK,
			expectConsumeCall:   true,
			expectFallbackCall:  false,
			expectWAGatewaySend: true,
			expectMetaRepoCall:  true,
			expectRepoFacCall:   true,
		},
		{
			name:                "ATIVAR command is case insensitive",
			from:                "5511999999999",
			wamid:               "wamid-002",
			text:                fmt.Sprintf("ativar %s", token),
			consumeOutcome:      usecases.ConsumeOutcomeActivated,
			expectHTTPStatus:    http.StatusOK,
			expectConsumeCall:   true,
			expectFallbackCall:  false,
			expectWAGatewaySend: true,
			expectMetaRepoCall:  true,
			expectRepoFacCall:   true,
		},
		{
			name:                "Non-ATIVAR message calls fallback",
			from:                "5511999999999",
			wamid:               "wamid-003",
			text:                "Olá, preciso de ajuda",
			fallbackOutcome:     usecases.FallbackOutcomeNoMatch,
			expectHTTPStatus:    http.StatusOK,
			expectConsumeCall:   false,
			expectFallbackCall:  true,
			expectWAGatewaySend: false,
			expectMetaRepoCall:  true,
			expectRepoFacCall:   true,
		},
		{
			name:                "Unsupported country does not dispatch use cases",
			from:                "12125551234",
			wamid:               "wamid-us",
			text:                "Olá",
			expectHTTPStatus:    http.StatusOK,
			expectConsumeCall:   false,
			expectFallbackCall:  false,
			expectWAGatewaySend: true,
			expectMetaRepoCall:  true,
			expectRepoFacCall:   true,
		},
		{
			name:               "Duplicate WAMID skips dispatch",
			from:               "5511999999999",
			wamid:              "wamid-dupe",
			text:               fmt.Sprintf("ATIVAR %s", token),
			metaRepoInserted:   true,
			expectHTTPStatus:   http.StatusOK,
			expectConsumeCall:  false,
			expectFallbackCall: false,
			expectMetaRepoCall: true,
			expectRepoFacCall:  true,
		},
		{
			name:                "Already active sends message",
			from:                "5511999999999",
			wamid:               "wamid-active",
			text:                fmt.Sprintf("ATIVAR %s", token),
			consumeOutcome:      usecases.ConsumeOutcomeAlreadyActive,
			expectHTTPStatus:    http.StatusOK,
			expectConsumeCall:   true,
			expectWAGatewaySend: true,
			expectMetaRepoCall:  true,
			expectRepoFacCall:   true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			consumeUC := handlersmocks.NewConsumeMagicTokenUseCase(s.T())
			fallbackUC := handlersmocks.NewTryFallbackActivationUseCase(s.T())
			waGW := appinterfacesmocks.NewWhatsAppGateway(s.T())
			metaRepo := appinterfacesmocks.NewMetaMessageRepository(s.T())
			repoFactory := appinterfacesmocks.NewRepositoryFactory(s.T())

			if scenario.expectConsumeCall {
				consumeUC.EXPECT().Execute(mock.Anything, mock.AnythingOfType("input.ConsumeMagicTokenInput")).
					Return(usecases.ConsumeResult{Outcome: scenario.consumeOutcome}, scenario.consumeErr).Once()
			}

			if scenario.expectFallbackCall {
				fallbackUC.EXPECT().Execute(mock.Anything, mock.AnythingOfType("string")).
					Return(usecases.FallbackResult{Outcome: scenario.fallbackOutcome}, scenario.fallbackErr).Once()
			}

			if scenario.expectMetaRepoCall {
				metaRepo.EXPECT().InsertIfAbsent(mock.Anything, scenario.wamid).
					Return(!scenario.metaRepoInserted, scenario.metaRepoErr).Once()
			}

			if scenario.expectRepoFacCall {
				repoFactory.EXPECT().MetaMessageRepository(mock.Anything).Return(metaRepo).Once()
			}

			if scenario.expectWAGatewaySend {
				waGW.EXPECT().SendTextMessage(mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
					Return(nil).Once()
			}

			handler := handlers.NewWhatsAppInboundHandler(
				consumeUC,
				fallbackUC,
				waGW,
				repoFactory,
				nil,
				s.msgs,
				noop.NewProvider(),
			)

			body := buildMetaPayload(scenario.from, scenario.wamid, scenario.text)
			req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(body))
			rr := httptest.NewRecorder()

			handler.Handle(rr, req)

			s.Equal(scenario.expectHTTPStatus, rr.Code)
		})
	}
}

func (s *WhatsAppInboundHandlerSuite) TestEmptyPayload_ReturnsOK() {
	consumeUC := handlersmocks.NewConsumeMagicTokenUseCase(s.T())
	fallbackUC := handlersmocks.NewTryFallbackActivationUseCase(s.T())
	waGW := appinterfacesmocks.NewWhatsAppGateway(s.T())
	repoFactory := appinterfacesmocks.NewRepositoryFactory(s.T())

	handler := handlers.NewWhatsAppInboundHandler(
		consumeUC,
		fallbackUC,
		waGW,
		repoFactory,
		nil,
		s.msgs,
		noop.NewProvider(),
	)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/whatsapp", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	handler.Handle(rr, req)

	s.Equal(http.StatusOK, rr.Code)
}
