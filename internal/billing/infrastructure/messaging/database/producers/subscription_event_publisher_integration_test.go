//go:build integration

package producers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type OutboxProducerIntegSuite struct {
	suite.Suite
	mgr             manager.Manager
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

	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr

	o11y := noop.NewProvider()
	s.factory = billingrepos.NewRepositoryFactory(o11y)
	s.outboxFactory = outboxrepo.NewRepositoryFactory(o11y)

	outboxCfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()

	s.publisher = producers.NewSubscriptionEventPublisher(s.outboxFactory, outboxCfg, idGen)

	saleUoW := uow.New[entities.Subscription](s.mgr, uow.WithObservability(o11y))
	s.processSaleUC = usecases.NewProcessSaleApproved(saleUoW, s.factory, s.publisher, o11y)

	s.seedKiwifyProductID(ctx)
}

func (s *OutboxProducerIntegSuite) seedKiwifyProductID(ctx context.Context) {
	row := s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT kiwify_product_id FROM billing_plans WHERE code='MONTHLY' LIMIT 1`)
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
			in := input.ProcessSaleApprovedInput{
				EnvelopeID:      fmt.Sprintf("env-%d", time.Now().UnixNano()),
				SaleID:          saleID,
				KiwifyProductID: s.kiwifyProductID,
				OrderID:         orderID,
				FunnelToken:     "token-integ-001",
				OccurredAt:      time.Now().UTC().Truncate(time.Millisecond),
			}

			err := s.processSaleUC.Execute(ctx, in)
			s.Require().NoError(err)

			var count int
			row := s.mgr.DBTX(ctx).QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, producers.EventTypeSubscriptionActivated)
			s.Require().NoError(row.Scan(&count))
			s.Equal(1, count, "expected exactly 1 outbox row with event_type billing.subscription.activated")
		})
	}
}
