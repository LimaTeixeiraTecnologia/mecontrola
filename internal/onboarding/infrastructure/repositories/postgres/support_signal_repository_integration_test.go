//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type SupportSignalRepositoryIntegrationSuite struct {
	suite.Suite
}

func TestSupportSignalRepositoryIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SupportSignalRepositoryIntegrationSuite))
}

func (s *SupportSignalRepositoryIntegrationSuite) TestInsert_PersistsSignal() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewSupportSignalRepository(noop.NewProvider(), db)

	payload := json.RawMessage(`{"external_sale_id": "sale-001"}`)
	signal, err := entities.NewSupportSignal(uuid.NewString(), valueobjects.SupportSignalKindPaidWithoutToken, payload)
	s.Require().NoError(err)

	s.Require().NoError(repo.Insert(ctx, signal))

	var count int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.support_signals WHERE id = $1`,
		signal.ID(),
	).Scan(&count))
	s.Equal(1, count)

	var kind string
	var occurredAtNotNull, resolvedAtNull bool
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT kind, (occurred_at IS NOT NULL), (resolved_at IS NULL)
		   FROM mecontrola.support_signals WHERE id = $1`,
		signal.ID(),
	).Scan(&kind, &occurredAtNotNull, &resolvedAtNull))
	s.Equal("paid_without_token", kind)
	s.True(occurredAtNotNull)
	s.True(resolvedAtNull)
}

func (s *SupportSignalRepositoryIntegrationSuite) TestInsert_DifferentKinds() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewSupportSignalRepository(noop.NewProvider(), db)

	kinds := []valueobjects.SupportSignalKind{
		valueobjects.SupportSignalKindPaidWithoutToken,
		valueobjects.SupportSignalKindOrphanExpiredSubscription,
		valueobjects.SupportSignalKindTokenReuseAttempt,
	}
	for _, k := range kinds {
		sig, err := entities.NewSupportSignal(uuid.NewString(), k, json.RawMessage(`{}`))
		s.Require().NoError(err)
		s.Require().NoError(repo.Insert(ctx, sig))
	}

	var total int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.support_signals`,
	).Scan(&total))
	s.Equal(3, total)

	var tokenReuseCount int
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mecontrola.support_signals WHERE kind = 'token_reuse_attempt'`,
	).Scan(&tokenReuseCount))
	s.Equal(1, tokenReuseCount)
}

func (s *SupportSignalRepositoryIntegrationSuite) TestInsert_PayloadRoundtrip() {
	db, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	repo := postgres.NewSupportSignalRepository(noop.NewProvider(), db)

	payload := json.RawMessage(`{"external_sale_id": "sale-abc", "mobile": "+5511999000001"}`)
	signal, err := entities.NewSupportSignal(uuid.NewString(), valueobjects.SupportSignalKindPaidWithoutToken, payload)
	s.Require().NoError(err)

	s.Require().NoError(repo.Insert(ctx, signal))

	var externalSaleID string
	s.Require().NoError(db.QueryRowContext(ctx,
		`SELECT payload->>'external_sale_id' FROM mecontrola.support_signals WHERE id = $1`,
		signal.ID(),
	).Scan(&externalSaleID))
	s.Equal("sale-abc", externalSaleID)
}
