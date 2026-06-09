//go:build integration

package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/migration"
	dbpostgres "github.com/JailtonJunior94/devkit-go/pkg/database/postgres"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stretchr/testify/mock"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dedup/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
	"github.com/LimaTeixeiraTecnologia/mecontrola/migrations"
)

type mockIntegrationGateway struct {
	mock.Mock
}

func (m *mockIntegrationGateway) SendTextMessage(ctx context.Context, toE164, text string) error {
	args := m.Called(ctx, toE164, text)
	return args.Error(0)
}

const integrationPgImageDispatcher = "postgres:16"

type DispatcherIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	mgr  manager.Manager
	o11y *noop.Provider
}

func TestDispatcherIntegrationSuite(t *testing.T) {
	suite.Run(t, new(DispatcherIntegrationSuite))
}

func (s *DispatcherIntegrationSuite) SetupSuite() {
	s.mgr = setupDispatcherTestDB(s.T())
	s.o11y = noop.NewProvider()
}

func (s *DispatcherIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func setupDispatcherTestDB(t *testing.T) manager.Manager {
	t.Helper()
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        integrationPgImageDispatcher,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if terr := container.Terminate(context.Background()); terr != nil {
			t.Logf("container terminate: %v", terr)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}

	mapped, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	portNum, err := strconv.Atoi(mapped.Port())
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}

	cfg := dbpostgres.PostgresConfig{
		DSN: fmt.Sprintf("postgres://test:test@%s:%d/testdb?sslmode=disable&search_path=mecontrola,public", host, portNum),
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	t.Cleanup(func() {
		_ = mgr.Shutdown(context.Background())
	})

	dsn := fmt.Sprintf("pgx5://test:test@%s:%d/testdb?sslmode=disable", host, portNum)

	migrator, err := migration.New(mgr, migration.EmbedFS{FS: migrations.FS, Root: "."}, migration.WithDSN(dsn))
	if err != nil {
		t.Fatalf("failed to create migrator: %v", err)
	}

	if err := migrator.Up(ctx); err != nil && !errors.Is(err, migration.ErrNoChange) {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return mgr
}

func (s *DispatcherIntegrationSuite) outboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func (s *DispatcherIntegrationSuite) newPublisher() outbox.Publisher {
	storage := outbox.NewPostgresStorage(s.mgr.DBTX(s.ctx))
	return outbox.NewPostgresPublisher(storage, s.outboxCfg())
}

func (s *DispatcherIntegrationSuite) seedActiveUser(wa string) entities.User {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.mgr.DBTX(s.ctx))
	waNum, err := valueobjects.NewWhatsAppNumber(wa)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	return user
}

func (s *DispatcherIntegrationSuite) countOutboxByType(eventType string) int {
	var total int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`,
		eventType,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *DispatcherIntegrationSuite) buildPayload(from, text, wamid string) json.RawMessage {
	type textBody struct {
		Body string `json:"body"`
	}
	type message struct {
		From      string   `json:"from"`
		ID        string   `json:"id"`
		Timestamp string   `json:"timestamp"`
		Type      string   `json:"type"`
		Text      textBody `json:"text"`
	}
	type metadata struct {
		DisplayPhoneNumber string `json:"display_phone_number"`
		PhoneNumberID      string `json:"phone_number_id"`
	}
	type changeValue struct {
		MessagingProduct string    `json:"messaging_product"`
		Metadata         metadata  `json:"metadata"`
		Messages         []message `json:"messages"`
	}
	type change struct {
		Field string      `json:"field"`
		Value changeValue `json:"value"`
	}
	type entry struct {
		ID      string   `json:"id"`
		Changes []change `json:"changes"`
	}
	type webhookPayload struct {
		Object string  `json:"object"`
		Entry  []entry `json:"entry"`
	}

	fromRaw := from
	if len(fromRaw) > 0 && fromRaw[0] == '+' {
		fromRaw = fromRaw[1:]
	}

	wp := webhookPayload{
		Object: "whatsapp_business_account",
		Entry: []entry{{
			ID: "1",
			Changes: []change{{
				Field: "messages",
				Value: changeValue{
					MessagingProduct: "whatsapp",
					Metadata: metadata{
						DisplayPhoneNumber: "5511999999999",
						PhoneNumberID:      "123",
					},
					Messages: []message{{
						From:      fromRaw,
						ID:        wamid,
						Timestamp: "1700000000",
						Type:      "text",
						Text:      textBody{Body: text},
					}},
				},
			}},
		}},
	}

	raw, err := json.Marshal(wp)
	s.Require().NoError(err)
	return raw
}

func (s *DispatcherIntegrationSuite) newSUT(limiter *ratelimit.Limiter, waGW *mockIntegrationGateway) *dispatcher.Dispatcher {
	factory := repositories.NewRepositoryFactory(s.o11y)
	establishUoW := uow.New[usecases.EstablishResult](s.mgr, uow.WithObservability(s.o11y))
	establishUC := usecases.NewEstablishPrincipal(establishUoW, factory, s.newPublisher(), s.o11y)

	dedupRepo := postgres.NewMessageRepository(s.o11y, s.mgr)

	stubAgent := agent.NewStubAgent(waGW, map[string]string{
		"agent_stub_received": "MeControla recebeu sua mensagem — estamos preparando sua experiencia.",
	}, s.o11y)

	onboarding := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeOnboarding
	}
	agentRoute := func(ctx context.Context, msg payload.Message) dispatcher.RouteOutcome {
		if _, ok := auth.FromContext(ctx); !ok {
			return dispatcher.OutcomeFallback
		}
		if err := stubAgent.HandleMessage(ctx, msg); err != nil {
			s.o11y.Logger().Warn(ctx, "test.agent_route_failed")
		}
		return dispatcher.OutcomeAgent
	}

	return dispatcher.New(dedupRepo, establishUC, limiter, s.newPublisher(), onboarding, agentRoute, s.o11y)
}

func (s *DispatcherIntegrationSuite) TestRoute_ValidPayload_ActiveUser_RoutesToAgent() {
	const waFrom = "+5511900000010"
	s.seedActiveUser(waFrom)

	limiter := ratelimit.New(s.o11y)
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	gw := &mockIntegrationGateway{}
	gw.On("SendTextMessage", mock.Anything, waFrom, mock.AnythingOfType("string")).Return(nil)
	defer gw.AssertExpectations(s.T())

	raw := s.buildPayload(waFrom, "oi", "wamid.integration.001")
	sut := s.newSUT(limiter, gw)

	outcome, err := sut.Route(s.ctx, raw)

	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeAgent, outcome)
	s.GreaterOrEqual(s.countOutboxByType("auth.principal_established"), 1)
}

func (s *DispatcherIntegrationSuite) TestRoute_CorruptPayload_PublishesAuthFailed() {
	limiter := ratelimit.New(s.o11y)
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	beforeCount := s.countOutboxByType("auth.failed")

	raw := json.RawMessage(`not-json`)
	gw := &mockIntegrationGateway{}
	sut := s.newSUT(limiter, gw)

	outcome, err := sut.Route(s.ctx, raw)

	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeInvalid, outcome)
	s.Greater(s.countOutboxByType("auth.failed"), beforeCount)
}

func (s *DispatcherIntegrationSuite) TestRoute_RateLimitExceeded_PublishesAuthFailed() {
	const waFrom = "+5511900000011"
	s.seedActiveUser(waFrom)

	limiter := ratelimit.New(s.o11y)
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	gw := &mockIntegrationGateway{}
	gw.On("SendTextMessage", mock.Anything, waFrom, mock.AnythingOfType("string")).Return(nil)

	raw := s.buildPayload(waFrom, "oi", "wamid.integration.002")
	sut := s.newSUT(limiter, gw)

	firstOutcome, err := sut.Route(s.ctx, raw)
	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeAgent, firstOutcome)

	beforeCount := s.countOutboxByType("auth.failed")

	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.mgr.DBTX(s.ctx))
	waNum, err := valueobjects.NewWhatsAppNumber(waFrom)
	s.Require().NoError(err)
	user, found, err := repo.TryFindActiveByWhatsApp(s.ctx, waNum)
	s.Require().NoError(err)
	s.Require().True(found)

	userUUID, parseErr := uuid.Parse(user.ID())
	s.Require().NoError(parseErr)

	for range ratelimit.DefaultBucketCapacity {
		limiter.Allow(userUUID)
	}

	raw2 := s.buildPayload(waFrom, "segunda mensagem", "wamid.integration.003")
	outcome, err := sut.Route(s.ctx, raw2)

	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeRateLimited, outcome)
	s.Greater(s.countOutboxByType("auth.failed"), beforeCount)
}

func (s *DispatcherIntegrationSuite) TestRoute_DuplicateWAMID_NoAdditionalEvents() {
	const waFrom = "+5511900000012"
	s.seedActiveUser(waFrom)

	limiter := ratelimit.New(s.o11y)
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	gw := &mockIntegrationGateway{}
	gw.On("SendTextMessage", mock.Anything, waFrom, mock.AnythingOfType("string")).Return(nil)

	raw := s.buildPayload(waFrom, "oi", "wamid.integration.004")
	sut := s.newSUT(limiter, gw)

	outcome1, err := sut.Route(s.ctx, raw)
	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeAgent, outcome1)

	afterFirst := s.countOutboxByType("auth.principal_established")

	outcome2, err := sut.Route(s.ctx, raw)
	s.Require().NoError(err)
	s.Equal(dispatcher.OutcomeDuplicate, outcome2)
	s.Equal(afterFirst, s.countOutboxByType("auth.principal_established"))
}
