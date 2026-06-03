//go:build integration

package outbox_test

import (
	"context"
	"sync"
	"testing"
	"time"

	devkitdb "github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/stretchr/testify/suite"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

// StoragePgxIntegrationSuite cobre os 6 cenários obrigatórios da task 3.0.
type StoragePgxIntegrationSuite struct {
	suite.Suite
	ctx     context.Context
	cfg     *configs.Config
	mgr     *dbpkg.Manager
	storage *outbox.PgxStorage
}

func TestStoragePgxIntegration(t *testing.T) {
	suite.Run(t, new(StoragePgxIntegrationSuite))
}

func (s *StoragePgxIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.cfg = s.startPostgres()

	mgr, err := dbpkg.NewManager(s.cfg)
	s.Require().NoError(err)
	s.mgr = mgr

	s.Require().NoError(dbpkg.RunMigrations(s.ctx, s.mgr))
	s.storage = outbox.NewPgxStorage(s.mgr.Inner())
}

func (s *StoragePgxIntegrationSuite) TearDownSuite() {
	_ = s.mgr.Shutdown(context.Background())
}

func (s *StoragePgxIntegrationSuite) SetupTest() {
	// Truncate tables to isolate each test scenario.
	dbtx := s.mgr.Inner().DBTX(s.ctx)
	_, err := dbtx.ExecContext(s.ctx, "TRUNCATE outbox_deliveries, outbox_events CASCADE")
	s.Require().NoError(err)
}

func (s *StoragePgxIntegrationSuite) startPostgres() *configs.Config {
	container, err := tcpostgres.Run(s.ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

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

// mustSubscriptionName creates a SubscriptionName or fails the test.
func (s *StoragePgxIntegrationSuite) mustSubscriptionName(v string) outbox.SubscriptionName {
	sn, err := outbox.NewSubscriptionName(v)
	s.Require().NoError(err)
	return sn
}

// mustEvent creates a test Event.
func (s *StoragePgxIntegrationSuite) mustEvent(idStr string) outbox.Event {
	evtID, err := events.NewEventID(idStr)
	s.Require().NoError(err)
	evtName, err := events.NewEventName("test.outbox-storage")
	s.Require().NoError(err)
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtName,
		Version:       1,
		AggregateType: "TestAggregate",
		AggregateID:   "agg-001",
		Payload:       []byte(`{"key":"value"}`),
		OccurredAt:    time.Now().UTC(),
	})
	s.Require().NoError(err)
	return evt
}

// execInTx executes fn inside a transaction, committing on success.
func (s *StoragePgxIntegrationSuite) execInTx(fn func(tx devkitdb.DBTX)) {
	tx, err := s.mgr.Inner().BeginTx(s.ctx, devkitdb.TxOptions{})
	s.Require().NoError(err)
	defer func() {
		_ = tx.Rollback(s.ctx)
	}()
	fn(tx)
	s.Require().NoError(tx.Commit(s.ctx))
}

// ---------------------------------------------------------------------------
// Cenário 1: InsertEvent + InsertDeliveries com 3 subscriptions
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario1_InsertEventAndDeliveries() {
	scenarios := []struct {
		name string
	}{
		{"deve criar 1 event e 3 deliveries; segundo insert viola UNIQUE"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			evt := s.mustEvent("01HXZTEST00000000000000001")
			subs := []outbox.SubscriptionName{
				s.mustSubscriptionName("sub-alpha"),
				s.mustSubscriptionName("sub-beta"),
				s.mustSubscriptionName("sub-gamma"),
			}

			// First insert — must succeed.
			s.execInTx(func(tx devkitdb.DBTX) {
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evt))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evt.ID(), subs))
			})

			// Verify counts.
			dbtx := s.mgr.Inner().DBTX(s.ctx)
			var evtCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_events WHERE id = $1", evt.ID().String()).Scan(&evtCount))
			s.Equal(1, evtCount, "deve ter 1 evento")

			var delCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_deliveries WHERE event_id = $1", evt.ID().String()).Scan(&delCount))
			s.Equal(3, delCount, "deve ter 3 deliveries")

			// Second insert of same (event_id, subscription_name) must return ErrDuplicateSubscription.
			var txErr error
			tx2, err := s.mgr.Inner().BeginTx(s.ctx, devkitdb.TxOptions{})
			s.Require().NoError(err)
			txErr = s.storage.InsertDeliveries(s.ctx, tx2, evt.ID(), subs[:1])
			_ = tx2.Rollback(s.ctx)

			s.ErrorIs(txErr, outbox.ErrDuplicateSubscription, "segundo insert deve retornar ErrDuplicateSubscription")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 2: ClaimReady respeita batchSize, next_retry_at e atualiza campos
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario2_ClaimReadyBatchAndFields() {
	scenarios := []struct {
		name string
	}{
		{"deve retornar 10 deliveries ordenadas por id com status=claimed e attempts incrementado"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// Insert 100 events with 1 delivery each.
			dbtx := s.mgr.Inner().DBTX(s.ctx)
			for i := range 100 {
				evtID := eventIDFromIndex(i)
				_, err := dbtx.ExecContext(s.ctx, `
					INSERT INTO outbox_events (id, event_type, event_version, aggregate_type, aggregate_id, payload, headers, occurred_at)
					VALUES ($1, 'test.outbox-storage', 1, 'TestAggregate', 'agg', '{"k":1}', '{}', now())
				`, evtID)
				s.Require().NoError(err)
				_, err = dbtx.ExecContext(s.ctx, `
					INSERT INTO outbox_deliveries (event_id, subscription_name, status, next_retry_at)
					VALUES ($1, 'sub-claim', 'pending', now() - interval '1 second')
				`, evtID)
				s.Require().NoError(err)
			}

			claims, err := s.storage.ClaimReady(s.ctx, 10, "instance-a")
			s.Require().NoError(err)
			s.Len(claims, 10, "deve retornar exatamente 10 claims")

			// Claims must be ordered by delivery ID (ascending).
			for i := 1; i < len(claims); i++ {
				s.Less(int64(claims[i-1].ID), int64(claims[i].ID), "claims devem ser ordenados por id")
			}

			// All must have status=claimed, attempts=1, claimed_by=instance-a.
			for _, c := range claims {
				s.Equal(uint8(1), c.Attempt.Value(), "attempts deve ser 1")
				s.NotZero(c.ClaimedAt, "claimed_at deve estar preenchido")
				s.NotEmpty(c.Event.AggregateType(), "event deve estar hidratado")
			}

			// Verify in DB.
			var claimedCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_deliveries WHERE status='claimed' AND claimed_by='instance-a'",
			).Scan(&claimedCount))
			s.Equal(10, claimedCount)
		})
	}
}

func (s *StoragePgxIntegrationSuite) TestCenario2B_MarkFailedPreservesClaimAttemptSequence() {
	evt := s.mustEvent("01HXZTEST00000000000000008")
	sub := s.mustSubscriptionName("sub-retry")

	s.execInTx(func(tx devkitdb.DBTX) {
		s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evt))
		s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evt.ID(), []outbox.SubscriptionName{sub}))
	})

	claims, err := s.storage.ClaimReady(s.ctx, 1, "instance-retry-a")
	s.Require().NoError(err)
	s.Require().Len(claims, 1)
	s.Equal(uint8(1), claims[0].Attempt.Value(), "primeira claim deve consumir tentativa 1")

	pastRetry := time.Now().UTC().Add(-time.Second)
	s.Require().NoError(s.storage.MarkFailed(
		s.ctx,
		claims[0].ID,
		"erro transitorio",
		claims[0].Attempt,
		pastRetry,
	))

	dbtx := s.mgr.Inner().DBTX(s.ctx)
	var attemptsAfterFailure int
	s.Require().NoError(dbtx.QueryRowContext(s.ctx,
		"SELECT attempts FROM outbox_deliveries WHERE id = $1",
		int64(claims[0].ID),
	).Scan(&attemptsAfterFailure))
	s.Equal(1, attemptsAfterFailure, "falha nao deve incrementar attempt pela segunda vez")

	retryClaims, err := s.storage.ClaimReady(s.ctx, 1, "instance-retry-b")
	s.Require().NoError(err)
	s.Require().Len(retryClaims, 1)
	s.Equal(uint8(2), retryClaims[0].Attempt.Value(), "segunda claim deve consumir tentativa 2")
}

// ---------------------------------------------------------------------------
// Cenário 3: 2 goroutines paralelas nunca retornam o mesmo claim.ID (RF-14)
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario3_ConcurrentClaimNoOverlap() {
	scenarios := []struct {
		name string
	}{
		{"duas goroutines concorrentes nao retornam o mesmo claim.ID"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// Insert 100 deliveries.
			dbtx := s.mgr.Inner().DBTX(s.ctx)
			for i := range 100 {
				evtID := eventIDFromIndex(i)
				_, err := dbtx.ExecContext(s.ctx, `
					INSERT INTO outbox_events (id, event_type, event_version, aggregate_type, aggregate_id, payload, headers, occurred_at)
					VALUES ($1, 'test.outbox-storage', 1, 'TestAggregate', 'agg', '{"k":1}', '{}', now())
				`, evtID)
				s.Require().NoError(err)
				_, err = dbtx.ExecContext(s.ctx, `
					INSERT INTO outbox_deliveries (event_id, subscription_name, status, next_retry_at)
					VALUES ($1, 'sub-concur', 'pending', now() - interval '1 second')
				`, evtID)
				s.Require().NoError(err)
			}

			var (
				wg   sync.WaitGroup
				mu   sync.Mutex
				errA error
				errB error
				idsA []int64
				idsB []int64
			)

			wg.Add(2)
			go func() {
				defer wg.Done()
				cls, e := s.storage.ClaimReady(s.ctx, 50, "instance-a")
				mu.Lock()
				defer mu.Unlock()
				errA = e
				for _, c := range cls {
					idsA = append(idsA, int64(c.ID))
				}
			}()
			go func() {
				defer wg.Done()
				cls, e := s.storage.ClaimReady(s.ctx, 50, "instance-b")
				mu.Lock()
				defer mu.Unlock()
				errB = e
				for _, c := range cls {
					idsB = append(idsB, int64(c.ID))
				}
			}()
			wg.Wait()

			s.Require().NoError(errA)
			s.Require().NoError(errB)

			// Assert disjunction: no ID appears in both sets.
			setA := make(map[int64]struct{}, len(idsA))
			for _, id := range idsA {
				setA[id] = struct{}{}
			}
			for _, id := range idsB {
				_, found := setA[id]
				s.False(found, "claim.ID %d apareceu em ambas as goroutines — RF-14 violado", id)
			}

			s.Equal(100, len(idsA)+len(idsB), "todas as 100 deliveries devem ter sido claimadas")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 4: MarkProcessed e transição inválida
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario4_MarkProcessedAndInvalidTransition() {
	scenarios := []struct {
		name string
	}{
		{"MarkProcessed move claimed para processed; segunda chamada falha"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			evt := s.mustEvent("01HXZTEST00000000000000004")
			sub := s.mustSubscriptionName("sub-mark")

			s.execInTx(func(tx devkitdb.DBTX) {
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evt))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evt.ID(), []outbox.SubscriptionName{sub}))
			})

			// Claim it.
			claims, err := s.storage.ClaimReady(s.ctx, 1, "instance-test")
			s.Require().NoError(err)
			s.Require().Len(claims, 1)
			claim := claims[0]

			// Mark as processed.
			s.Require().NoError(s.storage.MarkProcessed(s.ctx, claim.ID, time.Now().UTC()))

			// Verify status in DB.
			dbtx := s.mgr.Inner().DBTX(s.ctx)
			var status string
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT status FROM outbox_deliveries WHERE id = $1", int64(claim.ID)).Scan(&status))
			s.Equal("processed", status)

			// Second MarkProcessed must fail (row has status='processed', WHERE status='claimed' matches nothing).
			err = s.storage.MarkProcessed(s.ctx, claim.ID, time.Now().UTC())
			s.Error(err, "segunda MarkProcessed deve retornar erro de transição inválida")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 5: ReleaseStuck volta para pending
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario5_ReleaseStuck() {
	scenarios := []struct {
		name string
	}{
		{"ReleaseStuck libera deliveries claimed há mais de 10min para pending"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			evt := s.mustEvent("01HXZTEST00000000000000005")
			sub := s.mustSubscriptionName("sub-stuck")

			s.execInTx(func(tx devkitdb.DBTX) {
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evt))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evt.ID(), []outbox.SubscriptionName{sub}))
			})

			// Manually set status=claimed with claimed_at = now() - 15min to simulate stuck.
			dbtx := s.mgr.Inner().DBTX(s.ctx)
			_, err := dbtx.ExecContext(s.ctx, `
				UPDATE outbox_deliveries
				   SET status = 'claimed', claimed_at = now() - interval '15 minutes', claimed_by = 'dead-instance'
				 WHERE event_id = $1
			`, evt.ID().String())
			s.Require().NoError(err)

			// ReleaseStuck with threshold = now() - 10min.
			olderThan := time.Now().UTC().Add(-10 * time.Minute)
			released, err := s.storage.ReleaseStuck(s.ctx, olderThan)
			s.Require().NoError(err)
			s.Equal(int64(1), released, "deve liberar 1 delivery")

			// Verify status reverted to pending.
			var status string
			var claimedBy *string
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT status, claimed_by FROM outbox_deliveries WHERE event_id = $1", evt.ID().String(),
			).Scan(&status, &claimedBy))
			s.Equal("pending", status)
			s.Nil(claimedBy, "claimed_by deve ser NULL após release")
		})
	}
}

// ---------------------------------------------------------------------------
// Cenário 6: PurgeOlderThan apaga deliveries finalizadas e eventos órfãos
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestCenario6_PurgeOlderThan() {
	scenarios := []struct {
		name string
	}{
		{"PurgeOlderThan apaga deliveries finalizadas e eventos sem deliveries restantes"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			// Insert event with one processed delivery (old) and one pending delivery (recent).
			evtOld := s.mustEvent("01HXZTEST00000000000000061")
			evtNew := s.mustEvent("01HXZTEST00000000000000062")
			subA := s.mustSubscriptionName("sub-purge-a")
			subB := s.mustSubscriptionName("sub-purge-b")

			s.execInTx(func(tx devkitdb.DBTX) {
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evtOld))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evtOld.ID(), []outbox.SubscriptionName{subA}))
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evtNew))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evtNew.ID(), []outbox.SubscriptionName{subB}))
			})

			dbtx := s.mgr.Inner().DBTX(s.ctx)

			// Mark evtOld's delivery as processed 91 days ago.
			_, err := dbtx.ExecContext(s.ctx, `
				UPDATE outbox_deliveries
				   SET status = 'processed', processed_at = now() - interval '91 days'
				 WHERE event_id = $1
			`, evtOld.ID().String())
			s.Require().NoError(err)

			// Also set created_at on old event to 91 days ago for orphan delete.
			_, err = dbtx.ExecContext(s.ctx, `
				UPDATE outbox_events SET created_at = now() - interval '91 days' WHERE id = $1
			`, evtOld.ID().String())
			s.Require().NoError(err)

			// PurgeOlderThan with threshold = now() - 90 days.
			olderThan := time.Now().UTC().Add(-90 * 24 * time.Hour)
			total, purgeErr := s.storage.PurgeOlderThan(s.ctx, olderThan)
			s.Require().NoError(purgeErr)
			s.GreaterOrEqual(total, int64(1), "deve apagar pelo menos 1 linha")

			// evtOld delivery must be gone.
			var oldDelCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_deliveries WHERE event_id = $1", evtOld.ID().String(),
			).Scan(&oldDelCount))
			s.Equal(0, oldDelCount, "delivery antiga deve ter sido apagada")

			// evtOld event must be gone (orphan).
			var oldEvtCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_events WHERE id = $1", evtOld.ID().String(),
			).Scan(&oldEvtCount))
			s.Equal(0, oldEvtCount, "evento órfão antigo deve ter sido apagado")

			// evtNew must still exist.
			var newDelCount int
			s.Require().NoError(dbtx.QueryRowContext(s.ctx,
				"SELECT COUNT(*) FROM outbox_deliveries WHERE event_id = $1", evtNew.ID().String(),
			).Scan(&newDelCount))
			s.Equal(1, newDelCount, "delivery recente deve persistir")
		})
	}
}

// ---------------------------------------------------------------------------
// Stats: verifica contagem de pending e dead_letter
// ---------------------------------------------------------------------------

func (s *StoragePgxIntegrationSuite) TestStats() {
	scenarios := []struct {
		name string
	}{
		{"Stats retorna contagens corretas por subscription e status"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			evt := s.mustEvent("01HXZTEST00000000000000007")
			sub := s.mustSubscriptionName("sub-stats")

			s.execInTx(func(tx devkitdb.DBTX) {
				s.Require().NoError(s.storage.InsertEvent(s.ctx, tx, evt))
				s.Require().NoError(s.storage.InsertDeliveries(s.ctx, tx, evt.ID(), []outbox.SubscriptionName{sub}))
			})

			st, err := s.storage.Stats(s.ctx)
			s.Require().NoError(err)
			s.Equal(int64(1), st.Pending[sub], "deve ter 1 pending para sub-stats")
		})
	}
}

// ---------------------------------------------------------------------------
// Helper: generate deterministic event IDs for bulk inserts.
// ---------------------------------------------------------------------------

func eventIDFromIndex(i int) string {
	return "01HXZTESTBULK" + padLeft(i, 13)
}

func padLeft(n, width int) string {
	s := ""
	tmp := n
	digits := []byte{}
	for tmp > 0 {
		digits = append([]byte{byte('0' + tmp%10)}, digits...)
		tmp /= 10
	}
	for len(s)+len(digits) < width {
		s += "0"
	}
	return s + string(digits)
}
