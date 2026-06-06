package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
)

type DecideUserEntitlementSuite struct {
	suite.Suite
	mgr             *mocks.FakeManager
	factoryMock     *mocks.RepositoryFactory
	entitlementMock *mocks.EntitlementRepository
	uc              *usecases.DecideUserEntitlement
}

func TestDecideUserEntitlement(t *testing.T) {
	suite.Run(t, new(DecideUserEntitlementSuite))
}

func (s *DecideUserEntitlementSuite) SetupTest() {
	s.mgr = mocks.NewFakeManager()
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.entitlementMock = mocks.NewEntitlementRepository(s.T())
	s.uc = usecases.NewDecideUserEntitlement(s.mgr, s.factoryMock, noop.NewProvider())
}

func (s *DecideUserEntitlementSuite) TestActiveEntitled() {
	userID := "user-active"
	periodEnd := time.Now().UTC().Add(24 * time.Hour)
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-1",
		Status:         "ACTIVE",
		PeriodEnd:      periodEnd,
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.True(decision.Entitled)
	s.Equal("active", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestPastDueWithinGraceEntitled() {
	userID := "user-past-due-grace"
	graceEnd := time.Now().UTC().Add(24 * time.Hour)
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-2",
		Status:         "PAST_DUE",
		PeriodEnd:      time.Now().UTC().Add(-time.Hour),
		GraceEnd:       graceEnd,
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.True(decision.Entitled)
	s.Equal("past_due_grace", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestPastDueAfterGraceNotEntitled() {
	userID := "user-past-due-expired"
	graceEnd := time.Now().UTC().Add(-24 * time.Hour)
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-3",
		Status:         "PAST_DUE",
		PeriodEnd:      time.Now().UTC().Add(-48 * time.Hour),
		GraceEnd:       graceEnd,
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.False(decision.Entitled)
	s.Equal("past_due_no_grace", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestCanceledPendingUntilPeriodEndEntitled() {
	userID := "user-canceled-pending"
	periodEnd := time.Now().UTC().Add(48 * time.Hour)
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-4",
		Status:         "CANCELED_PENDING",
		PeriodEnd:      periodEnd,
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.True(decision.Entitled)
	s.Equal("canceled_pending", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestExpiredNotEntitled() {
	userID := "user-expired"
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-5",
		Status:         "EXPIRED",
		PeriodEnd:      time.Now().UTC().Add(-24 * time.Hour),
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.False(decision.Entitled)
	s.Equal("expired", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestRefundedNotEntitled() {
	userID := "user-refunded"
	record := interfaces.EntitlementRecord{
		UserID:         userID,
		SubscriptionID: "sub-6",
		Status:         "REFUNDED",
		PeriodEnd:      time.Now().UTC().Add(24 * time.Hour),
	}

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(record, nil)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.False(decision.Entitled)
	s.Equal("refunded", decision.Reason)
}

func (s *DecideUserEntitlementSuite) TestNoSubscriptionNotEntitled() {
	userID := "user-no-sub"

	s.factoryMock.On("EntitlementRepository", mock.Anything).Return(s.entitlementMock)
	s.entitlementMock.On("FindByUserID", mock.Anything, userID).Return(interfaces.EntitlementRecord{}, application.ErrEntitlementNotFound)

	decision, err := s.uc.Execute(context.Background(), userID)
	s.Require().NoError(err)
	s.False(decision.Entitled)
	s.Equal("no_subscription", decision.Reason)
}
