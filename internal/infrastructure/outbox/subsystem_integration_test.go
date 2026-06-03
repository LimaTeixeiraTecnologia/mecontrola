//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

// SubsystemIntegrationSuite cobre RF-34 (integration ponta-a-ponta) e RF-39 (flag-off + reaper).
type SubsystemIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	cfg *configs.Config
	mgr *dbpkg.Manager
}

func TestSubsystemIntegration(t *testing.T) {
	suite.Run(t, new(SubsystemIntegrationSuite))
}

func (s *SubsystemIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.cfg = s.startPostgres()

	mgr, err := dbpkg.NewManager(s.cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *SubsystemIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *SubsystemIntegrationSuite) SetupTest() {
	dbtx := s.mgr.Inner().DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx, "TRUNCATE outbox_deliveries, outbox_events CASCADE")
	s.Require().NoError(err)
}

func (s *SubsystemIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = container.Terminate(context.Background()) })

	host, err := container.Host(s.ctx)
	s.Require().NoError(err)
	mappedPort, err := container.MappedPort(s.ctx, "5432")
	s.Require().NoError(err)

	return &configs.Config{
		DBConfig: configs.DBConfig{
			Host:     host,
			Port:     int(mappedPort.Num()),
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
			MaxConns: 10,
			MinConns: 2,
		},
	}
}

func (s *SubsystemIntegrationSuite) mustEventName(v string) events.EventName {
	n, err := events.NewEventName(v)
	s.Require().NoError(err)
	return n
}

func (s *SubsystemIntegrationSuite) mustSubscriptionName(v string) outbox.SubscriptionName {
	sn, err := outbox.NewSubscriptionName(v)
	s.Require().NoError(err)
	return sn
}

func (s *SubsystemIntegrationSuite) newSubsystemWithHandler(
	cfg configs.OutboxConfig,
	registry outbox.Registry,
) *outbox.Subsystem {
	storage := outbox.NewPgxStorage(s.mgr.Inner())
	sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
		Config:     cfg,
		Storage:    storage,
		Registry:   registry,
		InstanceID: outbox.NewInstanceID(),
	})
	s.Require().NoError(err)
	return sub
}

func (s *SubsystemIntegrationSuite) defaultCfg() configs.OutboxConfig {
	return configs.OutboxConfig{
		DispatcherEnabled:         true,
		DispatcherTickInterval:    50 * time.Millisecond,
		DispatcherBatchSize:       50,
		DispatcherHandlerTimeout:  5 * time.Second,
		RetryMaxAttempts:          5,
		RetryBaseBackoff:          100 * time.Millisecond,
		RetryMaxBackoff:           1 * time.Second,
		HousekeepingRetentionDays: 90,
		HousekeepingSchedule:      "@daily",
		ReaperInterval:            "@every 1m",
		ReaperStuckAfter:          5 * time.Minute,
	}
}

// TestEndToEnd verifica o ciclo completo publish → claim → handler → processed em < 2s (RF-34).
func (s *SubsystemIntegrationSuite) TestEndToEnd() {
	var processed int64

	eventType := s.mustEventName("platform.outbox-dummy")
	subName := s.mustSubscriptionName("outbox-dummy")

	registry := outbox.NewRegistry()
	s.Require().NoError(registry.Register(outbox.Subscription{
		Name:      subName,
		EventType: eventType,
		Handler: func(_ context.Context, _ outbox.Event) error {
			atomic.AddInt64(&processed, 1)
			return nil
		},
	}))

	sub := s.newSubsystemWithHandler(s.defaultCfg(), registry)
	s.Require().NoError(sub.Start(s.ctx))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sub.Stop(ctx)
	}()

	// Publicar 1 evento via Publisher.
	evtID, err := events.NewEventID("01HXTEST00000000000000001A")
	s.Require().NoError(err)

	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     eventType,
		Version:       1,
		AggregateType: "TestAggregate",
		AggregateID:   "agg-e2e",
		Payload:       json.RawMessage(`{"key":"value"}`),
		OccurredAt:    time.Now().UTC(),
	})
	s.Require().NoError(err)

	storage := outbox.NewPgxStorage(s.mgr.Inner())
	publisher := outbox.NewPublisher(storage, registry, nil)

	dbtx := s.mgr.Inner().DBTX(s.ctx)
	s.Require().NoError(publisher.Publish(s.ctx, dbtx, evt))

	// Aguardar até 2s para o delivery ser processado.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&processed) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	s.Equal(int64(1), atomic.LoadInt64(&processed), "evento deve ter sido processado em < 2s")

	// Verificar no banco que está marcado como processed.
	dbtx2 := s.mgr.Inner().DBTX(s.ctx)
	row := dbtx2.QueryRowContext(s.ctx,
		"SELECT COUNT(*) FROM outbox_deliveries WHERE status = 'processed'")
	var count int
	s.Require().NoError(row.Scan(&count))
	s.Equal(1, count, "delivery deve estar processed no banco")
}

// TestFlagOff verifica que com DispatcherEnabled=false o Publisher continua escrevendo
// mas o Dispatcher não processa (delivery permanece pending) (RF-39).
func (s *SubsystemIntegrationSuite) TestFlagOff() {
	eventType := s.mustEventName("platform.outbox-dummy")
	subName := s.mustSubscriptionName("outbox-dummy")

	registry := outbox.NewRegistry()
	s.Require().NoError(registry.Register(outbox.Subscription{
		Name:      subName,
		EventType: eventType,
		Handler:   outbox.DummyHandler,
	}))

	cfg := s.defaultCfg()
	cfg.DispatcherEnabled = false

	sub := s.newSubsystemWithHandler(cfg, registry)
	s.Require().NoError(sub.Start(s.ctx))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = sub.Stop(ctx)
	}()

	// Publicar evento.
	evtID, err := events.NewEventID("01HXTEST00000000000000002A")
	s.Require().NoError(err)

	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     eventType,
		Version:       1,
		AggregateType: "TestAggregate",
		AggregateID:   "agg-flagoff",
		Payload:       json.RawMessage(`{"key":"value"}`),
		OccurredAt:    time.Now().UTC(),
	})
	s.Require().NoError(err)

	storage := outbox.NewPgxStorage(s.mgr.Inner())
	publisher := outbox.NewPublisher(storage, registry, nil)

	dbtx := s.mgr.Inner().DBTX(s.ctx)
	s.Require().NoError(publisher.Publish(s.ctx, dbtx, evt))

	// Aguardar 500ms para garantir que o Dispatcher NÃO processou.
	time.Sleep(500 * time.Millisecond)

	dbtx2 := s.mgr.Inner().DBTX(s.ctx)
	row := dbtx2.QueryRowContext(s.ctx,
		"SELECT COUNT(*) FROM outbox_deliveries WHERE status = 'pending'")
	var pendingCount int
	s.Require().NoError(row.Scan(&pendingCount))
	s.Equal(1, pendingCount, "delivery deve permanecer pending quando dispatcher está desabilitado")

	row2 := dbtx2.QueryRowContext(s.ctx,
		"SELECT COUNT(*) FROM outbox_deliveries WHERE status = 'processed'")
	var processedCount int
	s.Require().NoError(row2.Scan(&processedCount))
	s.Equal(0, processedCount, "nenhuma delivery deve ser processada com dispatcher desabilitado")
}

// TestReaper verifica que delivery claimed há muito tempo volta para pending e é re-claimada (RF-39).
func (s *SubsystemIntegrationSuite) TestReaper() {
	eventType := s.mustEventName("platform.outbox-dummy")
	subName := s.mustSubscriptionName("outbox-dummy")

	var handlerCalls int64

	registry := outbox.NewRegistry()
	s.Require().NoError(registry.Register(outbox.Subscription{
		Name:      subName,
		EventType: eventType,
		Handler: func(_ context.Context, _ outbox.Event) error {
			atomic.AddInt64(&handlerCalls, 1)
			return nil
		},
	}))

	// Inserir um evento e delivery diretamente, com claimed_at = now() - 10m para testar o reaper.
	evtID, err := events.NewEventID("01HXTEST00000000000000003A")
	s.Require().NoError(err)

	dbtx := s.mgr.Inner().DBTX(s.ctx)
	_, err = dbtx.ExecContext(s.ctx, `
		INSERT INTO outbox_events (id, event_type, event_version, aggregate_type, aggregate_id, payload, headers, occurred_at)
		VALUES ($1, $2, 1, 'TestAggregate', 'agg-reaper', '{"key":"value"}', '{}', now())
	`, evtID.String(), eventType.String())
	s.Require().NoError(err)

	_, err = dbtx.ExecContext(s.ctx, `
		INSERT INTO outbox_deliveries (event_id, subscription_name, status, attempts, next_retry_at, claimed_at, claimed_by, updated_at)
		VALUES ($1, $2, 'claimed', 1, now(), now() - interval '10 minutes', 'dead-instance-001', now())
	`, evtID.String(), subName.String())
	s.Require().NoError(err)

	// Configurar reaper com intervalo curto para testes.
	cfg := s.defaultCfg()
	cfg.ReaperInterval = "@every 100ms"
	cfg.ReaperStuckAfter = 5 * time.Minute

	sub := s.newSubsystemWithHandler(cfg, registry)
	s.Require().NoError(sub.Start(s.ctx))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = sub.Stop(ctx)
	}()

	// Aguardar até 3s para o reaper liberar e o dispatcher reprocessar.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&handlerCalls) >= 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	s.GreaterOrEqual(atomic.LoadInt64(&handlerCalls), int64(1),
		"handler deve ter sido chamado após reaper liberar a delivery stuck")

	// Verificar que delivery foi processada.
	dbtx2 := s.mgr.Inner().DBTX(s.ctx)
	row := dbtx2.QueryRowContext(s.ctx,
		"SELECT COUNT(*) FROM outbox_deliveries WHERE status = 'processed'")
	var processedCount int
	s.Require().NoError(row.Scan(&processedCount))
	s.Equal(1, processedCount, "delivery deve estar processed após reaper + dispatcher")
}
