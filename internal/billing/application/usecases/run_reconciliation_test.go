package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

type RunReconciliationSuite struct {
	suite.Suite
	ctx              context.Context
	checkpointMock   *mocks.ReconciliationCheckpointRepository
	kiwifyClientMock *mocks.KiwifyClient
	factoryMock      *mocks.RepositoryFactory
	publisherMock    *mocks.SubscriptionEventPublisher
	uowMock          *mocks.UnitOfWorkSubscription
}

func TestRunReconciliation(t *testing.T) {
	suite.Run(t, new(RunReconciliationSuite))
}

func (s *RunReconciliationSuite) SetupTest() {
	s.ctx = context.Background()
	s.checkpointMock = mocks.NewReconciliationCheckpointRepository(s.T())
	s.kiwifyClientMock = mocks.NewKiwifyClient(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
}

func (s *RunReconciliationSuite) newSUT() *usecases.RunReconciliation {
	o11y := noop.NewProvider()
	saleApproved := usecases.NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, o11y)
	refund := usecases.NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, o11y)
	reconcile := usecases.NewReconcileSubscriptions(s.checkpointMock, s.kiwifyClientMock, saleApproved, refund, o11y)
	return usecases.NewRunReconciliation(s.checkpointMock, reconcile, o11y)
}

func (s *RunReconciliationSuite) TestExecute_HappyPath() {
	s.SetupTest()
	checkpoint := time.Now().UTC().Add(-time.Hour)

	s.checkpointMock.EXPECT().
		Get(s.ctx, "kiwify_sales").
		Return(checkpoint, nil).
		Once()
	s.kiwifyClientMock.EXPECT().
		ListSalesUpdatedSince(s.ctx, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), 1).
		Return(appinterfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil).
		Once()
	s.checkpointMock.EXPECT().
		Set(s.ctx, "kiwify_sales", mock.AnythingOfType("time.Time")).
		Return(nil).
		Once()

	sut := s.newSUT()
	err := sut.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *RunReconciliationSuite) TestExecute_CheckpointGetError() {
	s.SetupTest()
	sentinel := errors.New("db connection lost")

	s.checkpointMock.EXPECT().
		Get(s.ctx, "kiwify_sales").
		Return(time.Time{}, sentinel).
		Once()

	sut := s.newSUT()
	err := sut.Execute(s.ctx)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, sentinel))
}
