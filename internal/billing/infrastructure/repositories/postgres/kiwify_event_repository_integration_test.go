//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type KiwifyEventRepositorySuite struct {
	suite.Suite
	db      *sqlx.DB
	factory interfaces.RepositoryFactory
}

func TestKiwifyEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(KiwifyEventRepositorySuite))
}

func (s *KiwifyEventRepositorySuite) SetupTest() {}

func (s *KiwifyEventRepositorySuite) SetupSuite() {
	s.db = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *KiwifyEventRepositorySuite) newRepo() interfaces.KiwifyEventRepository {
	return s.factory.KiwifyEventRepository(s.db)
}

func (s *KiwifyEventRepositorySuite) TestPersist() {
	scenarios := []struct {
		name   string
		setup  func() (string, string, []byte, string)
		expect func(context.Context, interfaces.KiwifyEventRepository, string, string, []byte, string)
	}{
		{
			name: "deve persistir o evento",
			setup: func() (string, string, []byte, string) {
				rawBody, err := json.Marshal(map[string]any{"trigger": "order_approved", "order_id": "order-ke-001"})
				s.Require().NoError(err)
				return "envelope-ke-001", "order_approved", rawBody, "valid"
			},
			expect: func(ctx context.Context, repo interfaces.KiwifyEventRepository, envelopeID string, trigger string, rawBody []byte, signatureStatus string) {
				err := repo.Persist(ctx, envelopeID, trigger, rawBody, signatureStatus)
				s.Require().NoError(err)
			},
		},
		{
			name: "deve ignorar envelope duplicado",
			setup: func() (string, string, []byte, string) {
				rawBody, err := json.Marshal(map[string]any{"trigger": "order_approved"})
				s.Require().NoError(err)
				return "envelope-ke-dup-001", "order_approved", rawBody, "valid"
			},
			expect: func(ctx context.Context, repo interfaces.KiwifyEventRepository, envelopeID string, trigger string, rawBody []byte, signatureStatus string) {
				s.Require().NoError(repo.Persist(ctx, envelopeID, trigger, rawBody, signatureStatus))
				s.Require().NoError(repo.Persist(ctx, envelopeID, trigger, rawBody, signatureStatus))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			envelopeID, trigger, rawBody, signatureStatus := scenario.setup()
			scenario.expect(ctx, repo, envelopeID, trigger, rawBody, signatureStatus)
		})
	}
}

func (s *KiwifyEventRepositorySuite) TestMarkProcessed() {
	scenarios := []struct {
		name   string
		setup  func(context.Context, interfaces.KiwifyEventRepository) (string, time.Time)
		expect func(context.Context, string, time.Time)
	}{
		{
			name: "deve definir processed_at",
			setup: func(ctx context.Context, repo interfaces.KiwifyEventRepository) (string, time.Time) {
				rawBody, err := json.Marshal(map[string]any{"trigger": "subscription_late"})
				s.Require().NoError(err)
				envelopeID := "envelope-ke-processed-001"
				s.Require().NoError(repo.Persist(ctx, envelopeID, "subscription_late", rawBody, "valid"))
				return envelopeID, time.Now().UTC().Truncate(time.Millisecond)
			},
			expect: func(ctx context.Context, envelopeID string, processedAt time.Time) {
				dbtx := s.db
				var scanned time.Time
				err := dbtx.QueryRowContext(ctx, `SELECT processed_at FROM billing_kiwify_events WHERE envelope_id = $1`, envelopeID).Scan(&scanned)
				s.Require().NoError(err)
				s.Assert().WithinDuration(processedAt, scanned, time.Second)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			envelopeID, processedAt := scenario.setup(ctx, repo)
			s.Require().NoError(repo.MarkProcessed(ctx, envelopeID, processedAt))
			scenario.expect(ctx, envelopeID, processedAt)
		})
	}
}
