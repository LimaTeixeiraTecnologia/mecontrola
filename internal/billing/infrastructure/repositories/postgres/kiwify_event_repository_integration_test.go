//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type KiwifyEventRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestKiwifyEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(KiwifyEventRepositorySuite))
}

func (s *KiwifyEventRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *KiwifyEventRepositorySuite) newRepo() interfaces.KiwifyEventRepository {
	return s.factory.KiwifyEventRepository(s.mgr.DBTX(context.Background()))
}

func (s *KiwifyEventRepositorySuite) TestPersist_StoresEvent() {
	ctx := context.Background()
	repo := s.newRepo()

	rawBody, err := json.Marshal(map[string]any{"trigger": "compra_aprovada", "order_id": "order-ke-001"})
	s.Require().NoError(err)

	err = repo.Persist(ctx, "envelope-ke-001", "compra_aprovada", rawBody, "valid")
	s.Require().NoError(err)
}

func (s *KiwifyEventRepositorySuite) TestPersist_DuplicateEnvelopeIsNoOp() {
	ctx := context.Background()
	repo := s.newRepo()

	rawBody, err := json.Marshal(map[string]any{"trigger": "compra_aprovada"})
	s.Require().NoError(err)

	err = repo.Persist(ctx, "envelope-ke-dup-001", "compra_aprovada", rawBody, "valid")
	s.Require().NoError(err)

	err = repo.Persist(ctx, "envelope-ke-dup-001", "compra_aprovada", rawBody, "valid")
	s.Require().NoError(err, "duplicate envelope insert with ON CONFLICT DO NOTHING must not error")
}

func (s *KiwifyEventRepositorySuite) TestMarkProcessed_SetsProcessedAt() {
	ctx := context.Background()
	repo := s.newRepo()

	rawBody, err := json.Marshal(map[string]any{"trigger": "subscription_late"})
	s.Require().NoError(err)

	envelopeID := "envelope-ke-processed-001"
	s.Require().NoError(repo.Persist(ctx, envelopeID, "subscription_late", rawBody, "valid"))

	processedAt := time.Now().UTC().Truncate(time.Millisecond)
	s.Require().NoError(repo.MarkProcessed(ctx, envelopeID, processedAt))

	dbtx := s.mgr.DBTX(ctx)
	var scanned time.Time
	err = dbtx.QueryRowContext(ctx,
		`SELECT processed_at FROM billing_kiwify_events WHERE envelope_id = $1`,
		envelopeID,
	).Scan(&scanned)
	s.Require().NoError(err)
	s.Assert().WithinDuration(processedAt, scanned, time.Second)
}
