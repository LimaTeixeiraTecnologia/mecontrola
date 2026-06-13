//go:build integration

package middleware_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/services"
	identitymiddleware "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type RequireGatewayAuthIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	mgr    manager.Manager
	o11y   *noop.Provider
	secret []byte
}

func TestRequireGatewayAuthIntegration(t *testing.T) {
	suite.Run(t, new(RequireGatewayAuthIntegrationSuite))
}

func (s *RequireGatewayAuthIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr
	s.o11y = noop.NewProvider()
	s.ctx = context.Background()
	s.secret = []byte("test-secret-32-bytes-padding-aaaa")
}

func (s *RequireGatewayAuthIntegrationSuite) newPublisher() outbox.Publisher {
	storage := outbox.NewPostgresStorage(s.mgr.DBTX(s.ctx))
	return outbox.NewPostgresPublisher(storage, configs.OutboxConfig{RetryMaxAttempts: 3})
}

func (s *RequireGatewayAuthIntegrationSuite) buildChain() http.Handler {
	publisher := s.newPublisher()
	failureUC := usecases.NewRecordGatewayAuthFailure(publisher, s.o11y)
	deps := identitymiddleware.RequireGatewayAuthDeps{
		Secrets:       services.SecretPair{Current: s.secret},
		Window:        60 * time.Second,
		FailureLogger: failureUC,
		O11y:          s.o11y,
	}
	gwMiddleware := identitymiddleware.RequireGatewayAuth(deps)
	injectMiddleware := identitymiddleware.InjectPrincipalFromHeaderWithO11y(s.o11y)
	requireUserMiddleware := identitymiddleware.RequireUserWithO11y(s.o11y)

	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return gwMiddleware(injectMiddleware(requireUserMiddleware(stub)))
}

func (s *RequireGatewayAuthIntegrationSuite) computeHMAC(userID, ts string) string {
	msg := strings.ToLower(userID) + "." + ts
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *RequireGatewayAuthIntegrationSuite) countOutboxEvents(eventType string) int {
	var count int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, eventType,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *RequireGatewayAuthIntegrationSuite) TestValidGateway_PassesChain() {
	chain := s.buildChain()
	userID := "00000000-0000-0000-0000-000000000001"
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	sig := s.computeHMAC(userID, ts)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
	req.Header.Set("X-User-ID", userID)
	req.Header.Set("X-Gateway-Auth", sig)
	req.Header.Set("X-Gateway-Timestamp", ts)
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	s.Equal(http.StatusOK, rr.Code, "valid gateway with valid user header should reach stub and return 200")
}

func (s *RequireGatewayAuthIntegrationSuite) TestInvalidGateway_Returns401AndPersistsOutboxEvent() {
	chain := s.buildChain()
	before := s.countOutboxEvents("auth.failed")

	userID := "00000000-0000-0000-0000-000000000002"
	ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
	req.Header.Set("X-User-ID", userID)
	req.Header.Set("X-Gateway-Auth", strings.Repeat("a", 64))
	req.Header.Set("X-Gateway-Timestamp", ts)
	req.Header.Set("X-Request-Id", "req-integ-test-001")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rr := httptest.NewRecorder()
	chain.ServeHTTP(rr, req)

	s.Equal(http.StatusUnauthorized, rr.Code)
	s.Contains(rr.Body.String(), "unauthorized")

	after := s.countOutboxEvents("auth.failed")
	s.Equal(before+1, after, "expected one new auth.failed outbox event")
}
