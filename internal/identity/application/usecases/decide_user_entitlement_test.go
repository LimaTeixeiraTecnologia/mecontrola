package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
)

type DecideUserEntitlementSuite struct {
	suite.Suite
	ctx context.Context
}

func TestDecideUserEntitlement(t *testing.T) {
	suite.Run(t, new(DecideUserEntitlementSuite))
}

func (s *DecideUserEntitlementSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *DecideUserEntitlementSuite) TestExecute() {
	type args struct {
		userID string
	}

	type dependencies struct {
		manager *usecasemocks.FakeManager
		factory *interfacesmocks.MockRepositoryFactory
		repo    *interfacesmocks.MockEntitlementRepository
	}

	now := time.Now().UTC()

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(bool, string, error)
	}{
		{
			name: "deve conceder acesso para assinatura ativa",
			args: args{userID: "user-active"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-active",
					SubscriptionID: "sub-1",
					Status:         "ACTIVE",
					PeriodEnd:      now.Add(24 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-active").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.True(entitled)
				s.Equal("active", reason)
			},
		},
		{
			name: "deve conceder acesso para past due dentro da carencia",
			args: args{userID: "user-past-due-grace"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-past-due-grace",
					SubscriptionID: "sub-2",
					Status:         "PAST_DUE",
					PeriodEnd:      now.Add(-time.Hour),
					GraceEnd:       now.Add(24 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-past-due-grace").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.True(entitled)
				s.Equal("past_due_grace", reason)
			},
		},
		{
			name: "deve negar acesso para past due fora da carencia",
			args: args{userID: "user-past-due-expired"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-past-due-expired",
					SubscriptionID: "sub-3",
					Status:         "PAST_DUE",
					PeriodEnd:      now.Add(-48 * time.Hour),
					GraceEnd:       now.Add(-24 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-past-due-expired").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.False(entitled)
				s.Equal("past_due_no_grace", reason)
			},
		},
		{
			name: "deve conceder acesso para cancelado com periodo vigente",
			args: args{userID: "user-canceled-pending"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-canceled-pending",
					SubscriptionID: "sub-4",
					Status:         "CANCELED_PENDING",
					PeriodEnd:      now.Add(48 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-canceled-pending").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.True(entitled)
				s.Equal("canceled_pending", reason)
			},
		},
		{
			name: "deve negar acesso para assinatura expirada",
			args: args{userID: "user-expired"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-expired",
					SubscriptionID: "sub-5",
					Status:         "EXPIRED",
					PeriodEnd:      now.Add(-24 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-expired").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.False(entitled)
				s.Equal("expired", reason)
			},
		},
		{
			name: "deve negar acesso para assinatura reembolsada",
			args: args{userID: "user-refunded"},
			setup: func(deps dependencies) {
				record := interfaces.EntitlementRecord{
					UserID:         "user-refunded",
					SubscriptionID: "sub-6",
					Status:         "REFUNDED",
					PeriodEnd:      now.Add(24 * time.Hour),
				}
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-refunded").Return(record, nil).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.False(entitled)
				s.Equal("refunded", reason)
			},
		},
		{
			name: "deve negar acesso quando nao existir assinatura",
			args: args{userID: "user-no-sub"},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().EntitlementRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByUserID(mock.Anything, "user-no-sub").Return(interfaces.EntitlementRecord{}, application.ErrEntitlementNotFound).Once()
			},
			expect: func(entitled bool, reason string, err error) {
				s.Require().NoError(err)
				s.False(entitled)
				s.Equal("no_subscription", reason)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				manager: usecasemocks.NewFakeManager(),
				factory: interfacesmocks.NewMockRepositoryFactory(s.T()),
				repo:    interfacesmocks.NewMockEntitlementRepository(s.T()),
			}
			scenario.setup(deps)

			sut := usecases.NewDecideUserEntitlement(deps.manager, deps.factory, noop.NewProvider())
			decision, err := sut.Execute(s.ctx, scenario.args.userID)

			scenario.expect(decision.Entitled, decision.Reason, err)
		})
	}
}
