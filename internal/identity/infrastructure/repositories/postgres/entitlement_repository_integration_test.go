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

func (s *EntitlementRepositoryIntegrationSuite) SetupTest() {}

func (s *EntitlementRepositoryIntegrationSuite) TestUpsertPending() {
	type args struct {
		subscriptionID string
		firstToken     string
		secondToken    string
		firstPayload   []byte
		secondPayload  []byte
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string, string, error)
	}{
		{
			name: "deve preservar funnel token original ao atualizar pendencia",
			args: args{
				subscriptionID: "11111111-1111-1111-1111-111111111111",
				firstToken:     "funnel-original",
				secondToken:    "",
				firstPayload:   []byte(`{"status":"ACTIVE"}`),
				secondPayload:  []byte(`{"status":"REFUNDED"}`),
			},
			expect: func(funnelToken string, payload string, err error) {
				s.Require().NoError(err)
				s.Equal("funnel-original", funnelToken)
				s.JSONEq(`{"status":"REFUNDED"}`, payload)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			manager, _ := setupTestDB(s.T())
			repo := identitypostgres.NewEntitlementRepository(noop.NewProvider(), manager.DBTX(ctx))

			s.Require().NoError(repo.UpsertPending(ctx, scenario.args.subscriptionID, scenario.args.firstToken, scenario.args.firstPayload))
			s.Require().NoError(repo.UpsertPending(ctx, scenario.args.subscriptionID, scenario.args.secondToken, scenario.args.secondPayload))

			var funnelToken string
			var payload string
			err := manager.DBTX(ctx).QueryRowContext(ctx, `
				SELECT funnel_token, payload::text
				  FROM identity_entitlements_pending
				 WHERE subscription_id = $1
			`, scenario.args.subscriptionID).Scan(&funnelToken, &payload)

			scenario.expect(funnelToken, payload, err)
		})
	}
}
