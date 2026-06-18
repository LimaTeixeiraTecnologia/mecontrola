package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/repositories/postgres"
	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"
)

type SubscriptionBinderSuite struct {
	suite.Suite
	tx     *dbmocks.MockDBTX
	result *dbmocks.MockResult
	subID  string
	userID string
}

func TestSubscriptionBinderSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionBinderSuite))
}

func (s *SubscriptionBinderSuite) SetupTest() {
	s.tx = dbmocks.NewMockDBTX(s.T())
	s.result = dbmocks.NewMockResult(s.T())
	s.subID = "sub-unit-001"
	s.userID = "user-unit-001"
}

func (s *SubscriptionBinderSuite) newBinder() interface {
	BindUser(ctx context.Context, subscriptionID, userID string) error
} {
	return postgres.NewSubscriptionBinder(noop.NewProvider(), s.tx)
}

func (s *SubscriptionBinderSuite) TestBindUser_ResolvesTxFromContextPerCall() {
	s.tx.EXPECT().
		ExecContext(mock.Anything, mock.Anything, s.userID, s.subID).
		Return(s.result, nil).
		Once()
	s.result.EXPECT().
		RowsAffected().
		Return(int64(1), nil).
		Once()

	err := s.newBinder().BindUser(context.Background(), s.subID, s.userID)
	s.NoError(err)
}

func (s *SubscriptionBinderSuite) TestBindUser_PropagatesExecError() {
	s.tx.EXPECT().
		ExecContext(mock.Anything, mock.Anything, s.userID, s.subID).
		Return(nil, errors.New("foreign key violation")).
		Once()

	err := s.newBinder().BindUser(context.Background(), s.subID, s.userID)
	s.ErrorContains(err, "subscription_binder.bind_user")
	s.ErrorContains(err, "foreign key violation")
}

func (s *SubscriptionBinderSuite) TestBindUser_SubscriptionNotFound() {
	s.tx.EXPECT().
		ExecContext(mock.Anything, mock.Anything, s.userID, s.subID).
		Return(s.result, nil).
		Once()
	s.result.EXPECT().
		RowsAffected().
		Return(int64(0), nil).
		Once()

	err := s.newBinder().BindUser(context.Background(), s.subID, s.userID)
	s.ErrorContains(err, "subscription not found")
}
