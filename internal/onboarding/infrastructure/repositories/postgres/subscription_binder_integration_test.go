//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type SubscriptionBinderIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	mgr  manager.Manager
	o11y *noop.Provider
}

func TestSubscriptionBinderIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionBinderIntegrationSuite))
}

func (s *SubscriptionBinderIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr
	s.o11y = noop.NewProvider()
}

func (s *SubscriptionBinderIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionBinderIntegrationSuite) newBinder() appinterfaces.SubscriptionBinder {
	return postgres.NewSubscriptionBinder(s.o11y, s.mgr)
}

func (s *SubscriptionBinderIntegrationSuite) seedPlan(code string) {
	_, err := s.mgr.DBTX(s.ctx).ExecContext(s.ctx,
		`INSERT INTO mecontrola.billing_plans (kiwify_product_id, code, duration_days, currency)
		 VALUES ($1, $2, 30, 'BRL') ON CONFLICT (code) DO NOTHING`,
		uuid.NewString(), code,
	)
	s.Require().NoError(err)
}

func (s *SubscriptionBinderIntegrationSuite) seedSubscription(planCode string) string {
	subID := uuid.NewString()
	now := time.Now().UTC()
	_, err := s.mgr.DBTX(s.ctx).ExecContext(s.ctx,
		`INSERT INTO mecontrola.billing_subscriptions
		   (id, funnel_token, user_id, kiwify_order_id, plan_code, status,
		    period_start, period_end, last_event_at)
		 VALUES ($1, $2, NULL, $3, $4, 'ACTIVE', $5, $6, $5)`,
		subID, uuid.NewString(), uuid.NewString(), planCode, now, now.Add(30*24*time.Hour),
	)
	s.Require().NoError(err)
	return subID
}

func (s *SubscriptionBinderIntegrationSuite) insertUserTx(ctx context.Context, whatsapp string) string {
	userID := uuid.NewString()
	_, err := s.mgr.DBTX(ctx).ExecContext(ctx,
		`INSERT INTO mecontrola.users (id, whatsapp_number, status) VALUES ($1, $2, 'ACTIVE')`,
		userID, whatsapp,
	)
	s.Require().NoError(err)
	return userID
}

func (s *SubscriptionBinderIntegrationSuite) subscriptionUserID(subID string) (string, bool) {
	row := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
		`SELECT user_id FROM mecontrola.billing_subscriptions WHERE id = $1`, subID)
	var userID *string
	s.Require().NoError(row.Scan(&userID))
	if userID == nil {
		return "", false
	}
	return *userID, true
}

func (s *SubscriptionBinderIntegrationSuite) TestBindUser_SeesUserCreatedInSameTx() {
	s.seedPlan("MONTHLY")
	subID := s.seedSubscription("MONTHLY")

	var boundUserID string
	_, err := uow.New[struct{}](s.mgr).Do(s.ctx, func(txCtx context.Context, _ database.DBTX) (struct{}, error) {
		boundUserID = s.insertUserTx(txCtx, "+5511980001111")
		return struct{}{}, s.newBinder().BindUser(txCtx, subID, boundUserID)
	})
	s.Require().NoError(err)

	persistedUserID, ok := s.subscriptionUserID(subID)
	s.True(ok)
	s.Equal(boundUserID, persistedUserID)
}

func (s *SubscriptionBinderIntegrationSuite) TestBindUser_RollsBackWithTxOnLaterError() {
	s.seedPlan("MONTHLY")
	subID := s.seedSubscription("MONTHLY")

	sentinel := context.Canceled
	_, err := uow.New[struct{}](s.mgr).Do(s.ctx, func(txCtx context.Context, _ database.DBTX) (struct{}, error) {
		userID := s.insertUserTx(txCtx, "+5511980002222")
		if bindErr := s.newBinder().BindUser(txCtx, subID, userID); bindErr != nil {
			return struct{}{}, bindErr
		}
		return struct{}{}, sentinel
	})
	s.Require().ErrorIs(err, sentinel)

	_, ok := s.subscriptionUserID(subID)
	s.False(ok)
}

func (s *SubscriptionBinderIntegrationSuite) TestBindUser_SubscriptionNotFound() {
	err := s.newBinder().BindUser(s.ctx, uuid.NewString(), uuid.NewString())
	s.ErrorContains(err, "subscription not found")
}
