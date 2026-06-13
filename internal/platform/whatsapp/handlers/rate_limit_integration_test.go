//go:build integration

package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	identityserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
)

type rateLimitNoopDispatcher struct{}

func (d *rateLimitNoopDispatcher) Route(_ context.Context, _ json.RawMessage) (dispatcher.RouteOutcome, error) {
	return dispatcher.OutcomeAgent, nil
}

type WhatsAppRateLimitSuite struct {
	suite.Suite
}

func TestWhatsAppRateLimitSuite(t *testing.T) {
	suite.Run(t, new(WhatsAppRateLimitSuite))
}

func buildTestRouter(burst int, onExceeded func()) (*httptest.Server, *middleware.RateLimiter) {
	rateLimiter := middleware.NewRateLimiter(600, burst, nil)

	o11y := noop.NewProvider()
	verifyHandler := handlers.NewVerifyHandler("test-token")
	inboundHandler := handlers.NewInboundHandler(&rateLimitNoopDispatcher{}, o11y)

	router := identityserver.NewWhatsAppWebhookRouter(
		verifyHandler,
		inboundHandler,
		"testsecret",
		"",
		rateLimiter.Middleware,
		onExceeded,
	)

	r := chi.NewRouter()
	router.Register(r)
	return httptest.NewServer(r), rateLimiter
}

func (s *WhatsAppRateLimitSuite) TestRateLimit_Returns429AfterBurstExhausted() {
	const burst = 5

	exceededCount := 0
	srv, rateLimiter := buildTestRouter(burst, func() { exceededCount++ })
	defer srv.Close()
	defer rateLimiter.Stop()

	rejected := 0
	for range burst + 1 {
		resp, err := http.Post(srv.URL+"/api/v1/whatsapp/inbound", "application/json", nil) //nolint:noctx
		s.Require().NoError(err)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			rejected++
		}
	}

	s.GreaterOrEqual(rejected, 1, "deve ter ao menos 1 rejeicao 429 apos burst esgotar")
	s.GreaterOrEqual(exceededCount, 1, "callback de metrica deve ser chamado ao menos uma vez")
}

func (s *WhatsAppRateLimitSuite) TestRateLimit_AllowsUpToBurst() {
	const burst = 5

	srv, rateLimiter := buildTestRouter(burst, nil)
	defer srv.Close()
	defer rateLimiter.Stop()

	tooManyCount := 0
	for range burst {
		resp, err := http.Post(srv.URL+"/api/v1/whatsapp/inbound", "application/json", nil) //nolint:noctx
		s.Require().NoError(err)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			tooManyCount++
		}
	}

	s.Equal(0, tooManyCount, "dentro do burst nao deve haver rejeicoes")
}
