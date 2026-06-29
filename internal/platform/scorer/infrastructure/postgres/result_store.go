package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type resultStore struct {
	db database.DBTX
}

func NewResultStore(db database.DBTX) scorer.ResultStore {
	return &resultStore{db: db}
}

func (s *resultStore) Insert(ctx context.Context, r scorer.ScorerResult) error {
	meta, err := json.Marshal(r.Metadata)
	if err != nil {
		return fmt.Errorf("scorer.postgres.result_store.insert: marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO mecontrola.platform_scorer_results
			(id, run_id, scorer_id, kind, score, reason, metadata, sampled, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = s.db.ExecContext(ctx, q,
		r.ID, r.RunID, r.ScorerID, r.Kind.String(),
		r.Score, r.Reason, meta, r.Sampled, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("scorer.postgres.result_store.insert: %w", err)
	}
	return nil
}
