package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

type RunReconciliationSuite struct {
	suite.Suite
	ctx              context.Context
	obs              *fake.Provider
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
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.checkpointMock = mocks.NewReconciliationCheckpointRepository(s.T())
	s.kiwifyClientMock = mocks.NewKiwifyClient(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
}

func (s *RunReconciliationSuite) newSUT() *RunReconciliation {
	saleApproved := NewProcessSaleApproved(s.uowMock, s.factoryMock, s.publisherMock, s.obs)
	refund := NewProcessRefundOrChargeback(s.uowMock, s.factoryMock, s.publisherMock, s.obs)
	reconcile := NewReconcileSubscriptions(s.checkpointMock, s.kiwifyClientMock, saleApproved, refund, s.obs)
	return NewRunReconciliation(s.checkpointMock, reconcile, s.obs)
}

func (s *RunReconciliationSuite) TestExecuteHappyPath() {
	checkpoint := time.Now().UTC().Add(-time.Hour)

	s.checkpointMock.EXPECT().
		Get(mock.Anything, "kiwify_sales").
		Return(checkpoint, nil).
		Once()
	s.kiwifyClientMock.EXPECT().
		ListSalesUpdatedSince(mock.Anything, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), 1).
		Return(appinterfaces.KiwifySalePage{Sales: nil, HasMore: false}, nil).
		Once()
	s.checkpointMock.EXPECT().
		Set(mock.Anything, "kiwify_sales", mock.AnythingOfType("time.Time")).
		Return(nil).
		Once()

	sut := s.newSUT()
	err := sut.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *RunReconciliationSuite) TestExecuteCheckpointGetError() {
	sentinel := errors.New("db connection lost")

	s.checkpointMock.EXPECT().
		Get(mock.Anything, "kiwify_sales").
		Return(time.Time{}, sentinel).
		Once()

	sut := s.newSUT()
	err := sut.Execute(s.ctx)
	s.Require().Error(err)
	s.Require().True(errors.Is(err, sentinel))
}
