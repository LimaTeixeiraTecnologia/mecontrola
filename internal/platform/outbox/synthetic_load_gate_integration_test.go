//go:build integration

package outbox_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type SyntheticLoadGateSuite struct {
	suite.Suite
}

func TestSyntheticLoadGateSuite(t *testing.T) {
	suite.Run(t, new(SyntheticLoadGateSuite))
}

func (s *SyntheticLoadGateSuite) SetupTest() {}

type syntheticEvent struct {
	id         string
	userID     string
	occurredAt time.Time
}

type claimRecord struct {
	userID     string
	occurredAt time.Time
	claimTime  time.Time
}

func (s *SyntheticLoadGateSuite) TestCA08_Phase500() {
	s.runPhase(500, 2, 20, true)
}

func (s *SyntheticLoadGateSuite) TestCA08_Phase2000() {
	s.runPhase(2000, 4, 50, true)
}

func (s *SyntheticLoadGateSuite) TestCA08_Phase10000() {
	s.runPhase(10000, 8, 100, false)
}

func (s *SyntheticLoadGateSuite) TestCA05_ComposeStopFirstConfig() {
	composeFiles := []string{
		"../../../deployment/compose/compose.swarm.yml",
	}
	for _, p := range composeFiles {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(raw)
		s.Contains(content, "stop-first", "CA-05: compose must use order: stop-first")
		s.Contains(content, "stop_grace_period: 30s", "CA-05: compose must have stop_grace_period: 30s")
		s.Contains(content, "OTEL_SERVICE_VERSION: ${IMAGE_TAG}", "CA-06: OTEL_SERVICE_VERSION must equal IMAGE_TAG")
		return
	}
	s.Fail("CA-05: compose.swarm.yml not found")
}

func (s *SyntheticLoadGateSuite) TestCA06_ResumedOnConflictMetricCode() {
	engineFile := "../../../internal/platform/workflow/engine.go"
	raw, err := os.ReadFile(engineFile)
	if os.IsNotExist(err) {
		engineFile = "../../workflow/engine.go"
		raw, err = os.ReadFile(engineFile)
	}
	s.Require().NoError(err, "CA-06: engine.go must be readable")
	content := string(raw)
	s.Contains(content, "workflow_resumed_on_conflict_total", "CA-06: resumed_on_conflict metric must exist")
	s.Contains(content, "workflow_version_conflict_total", "CA-06: version_conflict metric must exist")
}

func (s *SyntheticLoadGateSuite) runPhase(nUsers, nWorkers, batchSize int, enforceLagSLO bool) {
	ctx := context.Background()
	db, _ := testcontainer.Postgres(s.T())
	db.SetMaxOpenConns(nWorkers + 2)
	repo := outbox.NewPostgresStorage(db)

	const msgsPerUser = 3
	totalEvents := nUsers * msgsPerUser

	base := time.Now().UTC()
	events := s.generateEvents(nUsers, msgsPerUser, base)
	s.batchInsert(ctx, db, events)
	readyAt := time.Now().UTC()

	var (
		mu       sync.Mutex
		records  = make(map[string]claimRecord, totalEvents)
		seen     = make(map[string]int, totalEvents)
		errCount int
		wg       sync.WaitGroup
	)

	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			for {
				rows, err := repo.ClaimBatch(ctx, name, batchSize)
				if err != nil {
					mu.Lock()
					errCount++
					mu.Unlock()
					return
				}
				if len(rows) == 0 {
					return
				}
				ct := time.Now().UTC()
				mu.Lock()
				for _, r := range rows {
					records[r.ID] = claimRecord{
						userID:     r.AggregateUserID,
						occurredAt: r.OccurredAt,
						claimTime:  ct,
					}
					seen[r.ID]++
				}
				mu.Unlock()
				for _, r := range rows {
					if pubErr := repo.MarkPublished(ctx, r.ID); pubErr != nil {
						mu.Lock()
						errCount++
						mu.Unlock()
					}
				}
			}
		}(fmt.Sprintf("worker-%d", i))
	}
	wg.Wait()

	duplicates := 0
	for _, count := range seen {
		if count > 1 {
			duplicates++
		}
	}

	lags := make([]float64, 0, len(records))
	for _, rec := range records {
		lags = append(lags, rec.claimTime.Sub(readyAt).Seconds())
	}
	sort.Float64s(lags)

	lagP95 := 0.0
	if len(lags) > 0 {
		idx := int(math.Ceil(float64(len(lags))*0.95)) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(lags) {
			idx = len(lags) - 1
		}
		lagP95 = lags[idx]
	}

	userClaims := make(map[string][]claimRecord)
	for _, rec := range records {
		userClaims[rec.userID] = append(userClaims[rec.userID], rec)
	}
	fifoViolations := 0
	for _, evts := range userClaims {
		if len(evts) < 2 {
			continue
		}
		sort.Slice(evts, func(i, j int) bool {
			return evts[i].occurredAt.Before(evts[j].occurredAt)
		})
		for i := 1; i < len(evts); i++ {
			if evts[i].claimTime.Before(evts[i-1].claimTime) {
				fifoViolations++
			}
		}
	}

	s.T().Logf(
		"RF-19 phase=%d workers=%d batch=%d claimed=%d lag_p95=%.3fs errors=%d duplicates=%d fifo_violations=%d",
		nUsers, nWorkers, batchSize, len(records), lagP95, errCount, duplicates, fifoViolations,
	)

	usersWithMultiple := 0
	for _, evts := range userClaims {
		if len(evts) >= 2 {
			usersWithMultiple++
		}
	}

	stats := db.Stats()
	s.T().Logf("CA-08 pool max_open=%d in_use=%d wait_count=%d wait_dur=%s",
		stats.MaxOpenConnections, stats.InUse, stats.WaitCount, stats.WaitDuration)

	s.Equal(0, errCount, "CA-08: zero claim/publish errors (pool não satura sob carga)")
	s.Equal(totalEvents, len(records), "CA-08: all events claimed (no missed)")
	s.Equal(0, duplicates, "CA-08: zero duplicate claims")
	s.Equal(0, fifoViolations, "CA-01: FIFO per user must hold under concurrent load")
	s.Equal(nUsers, usersWithMultiple, "CA-01: every user must have multiple events so the FIFO check is exercised")
	s.Equal(nWorkers+2, stats.MaxOpenConnections, "CA-08: pool bounded independent of user count (no per-user connection)")

	if enforceLagSLO {
		s.LessOrEqualf(lagP95, 5.0, "CA-08: lag p95 %.3fs must be < 5s (near-term scale, D-05)", lagP95)
	} else if lagP95 > 5.0 {
		s.T().Logf("ADR-001 TRIGGER: phase=%d lag p95 %.3fs >= 5s no single-node — evolucao para particao por hash exigida antes do lancamento 10k (ADR-001 fase 2.000-10.000)", nUsers, lagP95)
	}
}

func (s *SyntheticLoadGateSuite) generateEvents(nUsers, msgsPerUser int, base time.Time) []syntheticEvent {
	events := make([]syntheticEvent, 0, nUsers*msgsPerUser)
	for u := range nUsers {
		userID := uuid.NewString()
		userBase := base.Add(time.Duration(u) * time.Millisecond)
		for m := range msgsPerUser {
			events = append(events, syntheticEvent{
				id:         uuid.NewString(),
				userID:     userID,
				occurredAt: userBase.Add(time.Duration(m) * time.Millisecond),
			})
		}
	}
	return events
}

func (s *SyntheticLoadGateSuite) batchInsert(ctx context.Context, db *sqlx.DB, events []syntheticEvent) {
	const chunkSize = 500
	for start := 0; start < len(events); start += chunkSize {
		end := start + chunkSize
		if end > len(events) {
			end = len(events)
		}
		s.insertChunk(ctx, db, events[start:end])
	}
}

func (s *SyntheticLoadGateSuite) insertChunk(ctx context.Context, db *sqlx.DB, chunk []syntheticEvent) {
	if len(chunk) == 0 {
		return
	}
	const cols = `(id, event_type, aggregate_type, aggregate_id, aggregate_user_id,
     payload, metadata, status, attempts, max_attempts, next_attempt_at,
     occurred_at, created_at, updated_at)`
	var sb strings.Builder
	sb.WriteString("INSERT INTO outbox_events ")
	sb.WriteString(cols)
	sb.WriteString(" VALUES ")

	args := make([]any, 0, len(chunk)*11)
	for i, e := range chunk {
		if i > 0 {
			sb.WriteByte(',')
		}
		base := i * 11
		fmt.Fprintf(&sb, "($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,now(),$%d,now(),now())",
			base+1, base+2, base+3, base+4, base+5,
			base+6, base+7, base+8, base+9, base+10, base+11)
		args = append(args,
			e.id,
			"agents.whatsapp.inbound.v1",
			"user",
			uuid.NewString(),
			e.userID,
			[]byte(`{}`),
			[]byte(`{}`),
			1,
			0,
			5,
			e.occurredAt,
		)
	}

	_, err := db.ExecContext(ctx, sb.String(), args...)
	s.Require().NoError(err)
}
