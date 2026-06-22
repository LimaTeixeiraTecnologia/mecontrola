package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type ProcessSubscriptionGraceExpiredSuite struct {
	suite.Suite
	ctx           context.Context
	obs           *fake.Provider
	uowMock       *mocks.UnitOfWorkSubscription
	factoryMock   *mocks.RepositoryFactory
	subRepoMock   *mocks.SubscriptionRepository
	publisherMock *mocks.SubscriptionEventPublisher
}

func TestProcessSubscriptionGraceExpired(t *testing.T) {
	suite.Run(t, new(ProcessSubscriptionGraceExpiredSuite))
}

func (s *ProcessSubscriptionGraceExpiredSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkSubscription(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.subRepoMock = mocks.NewSubscriptionRepository(s.T())
	s.publisherMock = mocks.NewSubscriptionEventPublisher(s.T())
}

func (s *ProcessSubscriptionGraceExpiredSuite) TestExecute() {
	type dependencies struct {
		factoryMock   *mocks.RepositoryFactory
		publisherMock *mocks.SubscriptionEventPublisher
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(error)
	}{
		{
			name: "past due com graca vencida transita para expired",
			dependencies: func() dependencies {
				graceEnd := time.Now().UTC().Add(-time.Hour)
				candidate := interfaces.ExpiredGraceCandidate{
					SubscriptionID: "sub-001",
					UserID:         "user-001",
					GraceEnd:       graceEnd,
					LastEventAt:    graceEnd.Add(-72 * time.Hour),
				}
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Twice()
				s.subRepoMock.EXPECT().
					ListPastDueGraceExpired(mock.Anything, mock.Anything, 100).
					Return([]interfaces.ExpiredGraceCandidate{candidate}, nil).
					Once()
				s.subRepoMock.EXPECT().
					ApplyTransition(mock.Anything, "sub-001", valueobjects.StatusExpired, time.Time{}, mock.Anything).
					Return(nil).
					Once()
				s.publisherMock.EXPECT().
					PublishExpired(mock.Anything, mock.Anything, mock.MatchedBy(func(sub entities.Subscription) bool {
						return sub.ID() == "sub-001" && sub.UserID() == "user-001" && sub.Status() == valueobjects.StatusExpired
					}), "sub-001", graceEnd).
					Return(nil).
					Once()
				return dependencies{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "quando nao existe candidato nao emite publicacao",
			dependencies: func() dependencies {
				s.factoryMock.EXPECT().SubscriptionRepository(mock.Anything).Return(s.subRepoMock).Once()
				s.subRepoMock.EXPECT().
					ListPastDueGraceExpired(mock.Anything, mock.Anything, 100).
					Return(nil, nil).
					Once()
				return dependencies{factoryMock: s.factoryMock, publisherMock: s.publisherMock}
			}(),
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewProcessSubscriptionGraceExpired(s.uowMock, nil, scenario.dependencies.factoryMock, scenario.dependencies.publisherMock, s.obs)
			err := sut.Execute(s.ctx)
			scenario.expect(err)
		})
	}
}
