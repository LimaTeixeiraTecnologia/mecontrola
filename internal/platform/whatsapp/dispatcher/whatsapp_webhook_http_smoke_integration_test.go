//go:build integration

package dispatcher_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	dedup "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wahandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/signature"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
)

func hmacHex(secret string, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *DispatcherIntegrationSuite) TestWebhookHTTP_SignedPayload_EstablishesPrincipalAndReachesAgent() {
	const waFrom = "+5511900000020"
	const secret = "test-app-secret"
	user := s.seedActiveUser(waFrom)

	limiter := ratelimit.New(s.o11y)
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	var capturedCalled bool
	var capturedHadPrincipal bool
	var capturedUserID string
	var capturedText string

	factory := repositories.NewRepositoryFactory(s.o11y)
	establishUoW := uow.NewUnitOfWork(s.db)
	establishUC := usecases.NewEstablishPrincipal(establishUoW, factory, s.newPublisher(), s.o11y)
	dedupRepo := dedup.NewMessageRepository(s.o11y, s.db)

	onboardingRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeOnboarding
	}
	agentRoute := func(ctx context.Context, msg payload.Message) dispatcher.RouteOutcome {
		capturedCalled = true
		capturedText = msg.Text
		if principal, ok := auth.FromContext(ctx); ok {
			capturedHadPrincipal = true
			capturedUserID = principal.UserID.String()
		}
		return dispatcher.OutcomeAgent
	}

	disp := dispatcher.New(dedupRepo, establishUC, limiter, s.newPublisher(), onboardingRoute, agentRoute, s.o11y)
	inbound := wahandlers.NewInboundHandler(disp, s.o11y)
	handler := signature.Compose(secret, "", func() {})(http.HandlerFunc(inbound.Handle))

	raw := s.buildPayload(waFrom, "ifood 58 reais", "wamid.smoke.http.001")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(raw))
	req.Header.Set("X-Hub-Signature-256", "sha256="+hmacHex(secret, raw))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.True(capturedCalled, "agentRoute deve ser invocado pelo webhook assinado")
	s.True(capturedHadPrincipal, "principal deve estar no contexto entregue ao agente")
	s.Equal(user.ID(), capturedUserID, "principal deve ser exatamente o usuário ativado")
	s.Equal("ifood 58 reais", capturedText, "texto deve chegar íntegro ao agente")

	capturedCalled = false
	forged := s.buildPayload(waFrom, "ifood 58 reais", "wamid.smoke.http.forged")
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/whatsapp/inbound", bytes.NewReader(forged))
	reqBad.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	recBad := httptest.NewRecorder()
	handler.ServeHTTP(recBad, reqBad)

	s.Equal(http.StatusUnauthorized, recBad.Code)
	s.False(capturedCalled, "assinatura inválida não pode chegar ao agente")
}
