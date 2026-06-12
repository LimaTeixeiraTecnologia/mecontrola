package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	cardvos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/client"
)

type stubCardExecutor struct {
	cycle cardvos.BillingCycle
	err   error
}

func (s *stubCardExecutor) Execute(_ context.Context, _, _ uuid.UUID) (cardvos.BillingCycle, error) {
	return s.cycle, s.err
}

type CardLookupAdapterSuite struct {
	suite.Suite
}

func TestCardLookupAdapterSuite(t *testing.T) {
	suite.Run(t, new(CardLookupAdapterSuite))
}

func (s *CardLookupAdapterSuite) buildAdapter(executor *stubCardExecutor) interfaces.CardLookup {
	o11y := noop.NewProvider()
	return client.NewCardLookupAdapter(executor, o11y)
}

func (s *CardLookupAdapterSuite) TestGetForUser_HappyPath() {
	cycle, err := cardvos.NewBillingCycle(15, 20)
	s.Require().NoError(err)

	executor := &stubCardExecutor{cycle: cycle}
	adapter := s.buildAdapter(executor)

	snapshot, err := adapter.GetForUser(context.Background(), uuid.New(), uuid.New())
	s.Require().NoError(err)
	s.Equal(15, snapshot.ClosingDay().Value())
	s.Equal(20, snapshot.DueDay().Value())
}

func (s *CardLookupAdapterSuite) TestGetForUser_CardNotFound() {
	executor := &stubCardExecutor{err: carddomain.ErrCardNotFound}
	adapter := s.buildAdapter(executor)

	_, err := adapter.GetForUser(context.Background(), uuid.New(), uuid.New())
	s.Require().Error(err)
	s.True(errors.Is(err, interfaces.ErrCardNotFound))
}

func (s *CardLookupAdapterSuite) TestGetForUser_IOError_ReturnsFailed() {
	executor := &stubCardExecutor{err: errors.New("network timeout")}
	adapter := s.buildAdapter(executor)

	_, err := adapter.GetForUser(context.Background(), uuid.New(), uuid.New())
	s.Require().Error(err)
	s.True(errors.Is(err, client.ErrCardLookupFailed))
}
