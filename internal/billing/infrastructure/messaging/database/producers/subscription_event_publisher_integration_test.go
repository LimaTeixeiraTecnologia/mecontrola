//go:build integration

package producers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type OutboxProducerIntegSuite struct {
	suite.Suite
	db              *sqlx.DB
	factory         interfaces.RepositoryFactory
	outboxFactory   outbox.OutboxRepositoryFactory
	publisher       *producers.SubscriptionEventPublisher
	processSaleUC   *usecases.ProcessSaleApproved
	kiwifyProductID string
}

func TestOutboxProducerIntegSuite(t *testing.T) {
	suite.Run(t, new(OutboxProducerIntegSuite))
}

func (s *OutboxProducerIntegSuite) SetupTest() {}

func (s *OutboxProducerIntegSuite) SetupSuite() {
	ctx := context.Background()

	db, _ := testcontainer.Postgres(s.T())
	s.db = db

	o11y := noop.NewProvider()
	s.factory = billingrepos.NewRepositoryFactory(o11y)
	s.outboxFactory = outboxrepo.NewRepositoryFactory(o11y)

	outboxCfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()

	s.publisher = producers.NewSubscriptionEventPublisher(s.outboxFactory, outboxCfg, idGen, noop.NewProvider())

	saleUoW := uow.NewUnitOfWork(s.db)
	s.processSaleUC = usecases.NewProcessSaleApproved(saleUoW, s.factory, s.publisher, o11y)

	s.seedKiwifyProductID(ctx)
}

func (s *OutboxProducerIntegSuite) seedKiwifyProductID(ctx context.Context) {
	row := s.db.QueryRowContext(ctx, `SELECT kiwify_product_id FROM billing_plans WHERE code='MONTHLY' LIMIT 1`)
	var pid string
	s.Require().NoError(row.Scan(&pid))
	s.kiwifyProductID = pid
}

func (s *OutboxProducerIntegSuite) TestRF10_OutboxRowCreatedTransactionallyOnProcessSaleApproved() {
	scenarios := []struct {
		name string
	}{
		{name: "deve criar a linha de outbox transacionalmente"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			saleID := fmt.Sprintf("sale-integ-%d", time.Now().UnixNano())
			orderID := fmt.Sprintf("order-integ-%d", time.Now().UnixNano())
			var beforeCount int
			beforeRow := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, producers.EventTypeSubscriptionActivated)
			s.Require().NoError(beforeRow.Scan(&beforeCount))
			in := input.ProcessSaleApprovedInput{
				EnvelopeID:      fmt.Sprintf("env-%d", time.Now().UnixNano()),
				SaleID:          saleID,
				KiwifyProductID: s.kiwifyProductID,
				OrderID:         orderID,
				KiwifySubID:     fmt.Sprintf("kiwify-sub-%d", time.Now().UnixNano()),
				FunnelToken:     "token-integ-001",
				OccurredAt:      time.Now().UTC().Truncate(time.Millisecond),
			}

			err := s.processSaleUC.Execute(ctx, in)
			s.Require().NoError(err)

			var count int
			row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, producers.EventTypeSubscriptionActivated)
			s.Require().NoError(row.Scan(&count))
			s.Equal(beforeCount+1, count, "expected exactly 1 new outbox row with event_type billing.subscription.activated")
		})
	}
}

func (s *OutboxProducerIntegSuite) TestPublishActivated_PreservesSemanticOccurredAt() {
	ctx := context.Background()
	expectedUserID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	expectedOccurredAt := time.Date(2026, 6, 13, 12, 34, 56, 0, time.UTC)

	plan, err := valueobjects.NewPlan("MONTHLY", 30)
	s.Require().NoError(err)
	token, err := valueobjects.NewFunnelToken("token-integ-occurred-at")
	s.Require().NoError(err)

	sub := entities.HydrateWithUser(
		"sub-integ-occurred-at",
		expectedUserID,
		token,
		plan,
		valueobjects.StatusActive,
		expectedOccurredAt.Add(-24*time.Hour),
		expectedOccurredAt.Add(29*24*time.Hour),
		time.Time{},
		expectedOccurredAt,
	)

	err = s.publisher.PublishActivated(
		ctx,
		s.db,
		sub,
		sub.ID(),
		token.String(),
		"+5511999999999",
		"user@example.com",
		"sale-integ-occurred-at",
	)
	s.Require().NoError(err)

	var occurredAt time.Time
	var aggregateUserID string
	row := s.db.QueryRowContext(ctx, `
		SELECT occurred_at, aggregate_user_id
		FROM outbox_events
		WHERE aggregate_id = $1
		  AND event_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, sub.ID(), producers.EventTypeSubscriptionActivated)
	s.Require().NoError(row.Scan(&occurredAt, &aggregateUserID))
	s.True(occurredAt.Equal(expectedOccurredAt))
	s.Equal(expectedUserID, aggregateUserID)
}
