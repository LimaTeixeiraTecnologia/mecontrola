//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	identitypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type EntitlementRepositoryIntegrationSuite struct {
	suite.Suite
}

func TestEntitlementRepositoryIntegration(t *testing.T) {
	suite.Run(t, new(EntitlementRepositoryIntegrationSuite))
}

func (s *EntitlementRepositoryIntegrationSuite) TestPendingUpdatePreservesFunnelToken() {
	ctx := context.Background()
	mgr, _ := setupTestDB(s.T())
	repo := identitypostgres.NewEntitlementRepository(noop.NewProvider(), mgr.DBTX(ctx))
	subscriptionID := "11111111-1111-1111-1111-111111111111"

	s.Require().NoError(repo.UpsertPending(ctx, subscriptionID, "funnel-original", []byte(`{"status":"ACTIVE"}`)))
	s.Require().NoError(repo.UpsertPending(ctx, subscriptionID, "", []byte(`{"status":"REFUNDED"}`)))

	var funnelToken, payload string
	err := mgr.DBTX(ctx).QueryRowContext(ctx, `
		SELECT funnel_token, payload::text
		  FROM identity_entitlements_pending
		 WHERE subscription_id = $1
	`, subscriptionID).Scan(&funnelToken, &payload)
	s.Require().NoError(err)
	s.Equal("funnel-original", funnelToken)
	s.JSONEq(`{"status":"REFUNDED"}`, payload)
}
