//go:build integration

package outbox_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

// ConcurrencyIntegrationSuite cobre RF-35: 3 dispatchers paralelos, 1000 deliveries, 0 double-processing.
type ConcurrencyIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	cfg *configs.Config
	mgr *dbpkg.Manager
}

func TestConcurrencyIntegration(t *testing.T) {
	suite.Run(t, new(ConcurrencyIntegrationSuite))
}

func (s *ConcurrencyIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.cfg = s.startPostgres()

	mgr, err := dbpkg.NewManager(s.cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
}

func (s *ConcurrencyIntegrationSuite) TearDownSuite() {
	if s.mgr != nil {
		s.Require().NoError(s.mgr.Shutdown(context.Background()))
	}
}

func (s *ConcurrencyIntegrationSuite) SetupTest() {
	dbtx := s.mgr.Inner().DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx, "TRUNCATE outbox_deliveries, outbox_events CASCADE")
	s.Require().NoError(err)
}

func (s *ConcurrencyIntegrationSuite) startPostgres() *configs.Config {
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
			MaxConns: 30,
			MinConns: 5,
		},
	}
}

// TestThreeDispatchersNoDoubleProcessing verifica RF-35: 3 dispatchers / 1000 deliveries / 0 double-processing.
func (s *ConcurrencyIntegrationSuite) TestThreeDispatchersNoDoubleProcessing() {
	const (
		numEvents      = 1000
		numDispatchers = 3
	)

	eventType, err := events.NewEventName("platform.outbox-concurrent")
	s.Require().NoError(err)
	subName, err := outbox.NewSubscriptionName("outbox-concurrent")
	s.Require().NoError(err)

	// Pré-popular 1000 eventos e deliveries diretamente no banco para velocidade.
	s.T().Log("Inserindo 1000 eventos e deliveries...")
	start := time.Now()
	dbtx := s.mgr.Inner().DBTX(s.ctx)
	for i := range numEvents {
		evtIDStr := fmt.Sprintf("01HXCONCUR%020d", i)
		_, err := dbtx.ExecContext(s.ctx, `
			INSERT INTO outbox_events (id, event_type, event_version, aggregate_type, aggregate_id, payload, headers, occurred_at)
			VALUES ($1, $2, 1, 'ConcurrentAggregate', $3, '{"test":true}', '{}', now())
		`, evtIDStr, eventType.String(), fmt.Sprintf("agg-%d", i))
		s.Require().NoError(err)

		_, err = dbtx.ExecContext(s.ctx, `
			INSERT INTO outbox_deliveries (event_id, subscription_name, status, next_retry_at)
			VALUES ($1, $2, 'pending', now())
		`, evtIDStr, subName.String())
		s.Require().NoError(err)
	}
	s.T().Logf("Inserção concluída em %s", time.Since(start))

	// Criar 3 subsystems com instanceIDs distintos, todos apontando para o mesmo Postgres.
	cfg := configs.OutboxConfig{
		DispatcherEnabled:         true,
		DispatcherTickInterval:    20 * time.Millisecond,
		DispatcherBatchSize:       100,
		DispatcherHandlerTimeout:  5 * time.Second,
		RetryMaxAttempts:          5,
		RetryBaseBackoff:          100 * time.Millisecond,
		RetryMaxBackoff:           1 * time.Second,
		HousekeepingRetentionDays: 90,
		HousekeepingSchedule:      "@daily",
		ReaperInterval:            "@every 1m",
		ReaperStuckAfter:          5 * time.Minute,
	}

	var (
		subsystems [numDispatchers]*outbox.Subsystem
		registries [numDispatchers]outbox.Registry
	)

	for i := range numDispatchers {
		registry := outbox.NewRegistry()
		s.Require().NoError(registry.Register(outbox.Subscription{
			Name:      subName,
			EventType: eventType,
			Handler:   outbox.DummyHandler,
		}))
		registries[i] = registry

		storage := outbox.NewPgxStorage(s.mgr.Inner())
		instanceID := fmt.Sprintf("worker-concurrent-%d", i)
		sub, err := outbox.NewSubsystem(outbox.SubsystemDeps{
			Config:     cfg,
			Storage:    storage,
			Registry:   registry,
			InstanceID: instanceID,
		})
		s.Require().NoError(err)
		subsystems[i] = sub
	}

	// Iniciar todos os dispatchers ao mesmo tempo.
	testStart := time.Now()
	for i := range numDispatchers {
		s.Require().NoError(subsystems[i].Start(s.ctx))
	}

	// Aguardar até 60s para todas as 1000 deliveries serem processadas.
	drainDeadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(drainDeadline) {
		dbtx2 := s.mgr.Inner().DBTX(s.ctx)
		row := dbtx2.QueryRowContext(s.ctx,
			"SELECT COUNT(*) FROM outbox_deliveries WHERE status IN ('pending', 'claimed')")
		var remaining int
		s.Require().NoError(row.Scan(&remaining))
		if remaining == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	elapsed := time.Since(testStart)

	// Parar todos os dispatchers.
	var wg sync.WaitGroup
	wg.Add(numDispatchers)
	for i := range numDispatchers {
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = subsystems[idx].Stop(ctx)
		}(i)
	}
	wg.Wait()

	// Verificar critério RF-35: zero double-processing via SQL GROUP BY HAVING COUNT(*) > 1.
	dbtx3 := s.mgr.Inner().DBTX(s.ctx)
	rows, err := dbtx3.QueryContext(s.ctx, `
		SELECT event_id, subscription_name, COUNT(*)
		  FROM outbox_deliveries
		 WHERE status = 'processed'
		 GROUP BY event_id, subscription_name
		HAVING COUNT(*) > 1
	`)
	s.Require().NoError(err)
	defer func() { _ = rows.Close() }()

	var doubled []string
	for rows.Next() {
		var evtID, subNameStr string
		var count int
		s.Require().NoError(rows.Scan(&evtID, &subNameStr, &count))
		doubled = append(doubled, fmt.Sprintf("%s/%s=%d", evtID, subNameStr, count))
	}
	s.Require().NoError(rows.Err())
	s.Empty(doubled, "zero double-processing esperado: %v", doubled)

	// Verificar que todas as 1000 deliveries foram processadas.
	dbtx4 := s.mgr.Inner().DBTX(s.ctx)
	row := dbtx4.QueryRowContext(s.ctx,
		"SELECT COUNT(*) FROM outbox_deliveries WHERE status = 'processed'")
	var processedCount int
	s.Require().NoError(row.Scan(&processedCount))
	s.Equal(numEvents, processedCount, "todas as %d deliveries devem estar processed", numEvents)

	// Calcular e logar throughput.
	elapsedSecs := elapsed.Seconds()
	throughput := float64(numEvents) / elapsedSecs
	s.T().Logf("Throughput: %.1f deliveries/s (%d deliveries em %.2fs)", throughput, numEvents, elapsedSecs)
	s.GreaterOrEqual(throughput, float64(100), "throughput mínimo de 100 deliveries/s não atingido")
}
