package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSubscriptionGraceExpiredSuite struct {
	suite.Suite
	ctx           context.Context
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSubscriptionGraceExpired(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionGraceExpiredSuite))
}

func (s *ProcessSubscriptionGraceExpiredSuite) SetupTest() {
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSubscriptionGraceExpiredSuite) TestPastDueComGracaVencidaTransitaParaExpired() {
	s.SetupTest()
	graceEnd := time.Now().UTC().Add(-time.Hour)
	candidate := interfaces.ExpiredGraceCandidate{
		SubscriptionID: "sub-001",
		UserID:         "user-001",
		GraceEnd:       graceEnd,
		LastEventAt:    graceEnd.Add(-72 * time.Hour),
	}

	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Twice()
	s.subRepoMock.EXPECT().
		ListPastDueGraceExpired(s.ctx, mock.Anything, 100).
		Return([]interfaces.ExpiredGraceCandidate{candidate}, nil).
		Once()
	s.subRepoMock.EXPECT().
		ApplyTransition(s.ctx, "sub-001", valueobjects.StatusExpired, time.Time{}, mock.Anything).
		Return(nil).
		Once()
	s.publisherMock.EXPECT().
		PublishExpired(s.ctx, mock.Anything, mock.MatchedBy(func(sub entities.Subscription) bool {
			return sub.ID() == "sub-001" && sub.UserID() == "user-001" && sub.Status() == valueobjects.StatusExpired
		}), "sub-001", graceEnd).
		Return(nil).
		Once()

	sut := usecases.NewProcessSubscriptionGraceExpired(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	err := sut.Execute(s.ctx)
	s.Require().NoError(err)
}

func (s *ProcessSubscriptionGraceExpiredSuite) TestQuandoNaoExisteCandidatoNaoEmitePublicacao() {
	s.SetupTest()

	s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
	s.subRepoMock.EXPECT().
		ListPastDueGraceExpired(s.ctx, mock.Anything, 100).
		Return(nil, nil).
		Once()

	sut := usecases.NewProcessSubscriptionGraceExpired(s.uowMock, s.factoryMock, s.publisherMock, noop.NewProvider())
	err := sut.Execute(s.ctx)
	s.Require().NoError(err)
}
