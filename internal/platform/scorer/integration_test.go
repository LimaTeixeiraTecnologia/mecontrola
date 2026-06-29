//go:build integration

package scorer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	scorerpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ScorerResultStoreIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestScorerResultStoreIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ScorerResultStoreIntegrationSuite))
}

func (s *ScorerResultStoreIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *ScorerResultStoreIntegrationSuite) insertThread() uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_threads (id, resource_id, thread_id, title)
		VALUES ($1, $2, $3, $4)`,
		id, "res-"+id.String(), "thr-"+id.String(), "test thread",
	)
	s.Require().NoError(err)
	return id
}

func (s *ScorerResultStoreIntegrationSuite) insertRun(threadPK uuid.UUID) uuid.UUID {
	id := uuid.New()
	_, err := s.db.ExecContext(s.ctx, `
		INSERT INTO mecontrola.platform_runs
			(id, thread_pk, resource_id, thread_id, status, started_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, threadPK, "res-"+id.String(), "thr-"+id.String(), "running", time.Now().UTC(),
	)
	s.Require().NoError(err)
	return id
}

func (s *ScorerResultStoreIntegrationSuite) TestInsert_Persists() {
	threadPK := s.insertThread()
	runID := s.insertRun(threadPK)

	store := scorerpostgres.NewResultStore(s.db)

	r := scorer.ScorerResult{
		ID:        uuid.New(),
		RunID:     runID,
		ScorerID:  "tool-accuracy",
		Kind:      scorer.ScorerKindCodeBased,
		Score:     0.75,
		Reason:    "3/4 tools matched",
		Metadata:  map[string]any{"expected": []string{"weather", "geocode"}, "matched": 3},
		Sampled:   true,
		CreatedAt: time.Now().UTC(),
	}

	err := store.Insert(s.ctx, r)
	s.Require().NoError(err)

	var count int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.platform_scorer_results WHERE id=$1`, r.ID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}

func (s *ScorerResultStoreIntegrationSuite) TestInsert_LLMJudged() {
	threadPK := s.insertThread()
	runID := s.insertRun(threadPK)

	store := scorerpostgres.NewResultStore(s.db)

	r := scorer.ScorerResult{
		ID:        uuid.New(),
		RunID:     runID,
		ScorerID:  "llm-judge",
		Kind:      scorer.ScorerKindLLMJudged,
		Score:     0.9,
		Reason:    "output closely matches expected",
		Metadata:  map[string]any{"model": "openrouter/test"},
		Sampled:   true,
		CreatedAt: time.Now().UTC(),
	}

	err := store.Insert(s.ctx, r)
	s.Require().NoError(err)

	var count int
	err = s.db.QueryRowContext(s.ctx,
		`SELECT count(*) FROM mecontrola.platform_scorer_results WHERE id=$1`, r.ID,
	).Scan(&count)
	s.Require().NoError(err)
	s.Equal(1, count)
}
