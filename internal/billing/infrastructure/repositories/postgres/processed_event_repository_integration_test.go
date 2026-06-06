//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

	billingrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/repositories"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
)

type ProcessedEventRepositorySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestProcessedEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(ProcessedEventRepositorySuite))
}

func (s *ProcessedEventRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ProcessedEventRepositorySuite) newRepo() interfaces.ProcessedEventRepository {
	return s.factory.ProcessedEventRepository(s.mgr.DBTX(context.Background()))
}

func (s *ProcessedEventRepositorySuite) TestMarkApplied_InsertsRow() {
	ctx := context.Background()
	repo := s.newRepo()

	eventKey := "compra_aprovada:order-pe-test-001"
	err := repo.MarkApplied(ctx, eventKey, "compra_aprovada", "order-pe-test-001", time.Now().UTC())
	s.Require().NoError(err)
}

func (s *ProcessedEventRepositorySuite) TestMarkApplied_DuplicateReturnsSentinel() {
	ctx := context.Background()
	repo := s.newRepo()

	eventKey := "compra_aprovada:order-pe-dup-001"
	occurredAt := time.Now().UTC()

	err := repo.MarkApplied(ctx, eventKey, "compra_aprovada", "order-pe-dup-001", occurredAt)
	s.Require().NoError(err)

	err = repo.MarkApplied(ctx, eventKey, "compra_aprovada", "order-pe-dup-001", occurredAt)
	s.Require().Error(err)
	s.Assert().True(errors.Is(err, interfaces.ErrEventAlreadyProcessed),
		"expected ErrEventAlreadyProcessed, got: %v", err)
}

func (s *ProcessedEventRepositorySuite) TestMarkSuperseded_UpdatesStatus() {
	ctx := context.Background()
	repo := s.newRepo()

	eventKey := "subscription_late:sub-superseded-001:2026-06-01T00:00:00Z"
	err := repo.MarkApplied(ctx, eventKey, "subscription_late", "sub-superseded-001", time.Now().UTC())
	s.Require().NoError(err)

	err = repo.MarkSuperseded(ctx, eventKey)
	s.Require().NoError(err)
}

type RF11ReplaySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestRF11ReplaySuite(t *testing.T) {
	suite.Run(t, new(RF11ReplaySuite))
}

func (s *RF11ReplaySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *RF11ReplaySuite) TestRF11_ThreeReplaysYieldOneInsertAndTwoSentinels() {
	ctx := context.Background()
	repo := s.factory.ProcessedEventRepository(s.mgr.DBTX(ctx))

	eventKey := "compra_aprovada:order-rf11-replay-001"
	occurredAt := time.Now().UTC()

	var errs []error
	for i := 0; i < 3; i++ {
		err := repo.MarkApplied(ctx, eventKey, "compra_aprovada", "order-rf11-replay-001", occurredAt)
		errs = append(errs, err)
	}

	inserted := 0
	sentinels := 0
	for _, err := range errs {
		if err == nil {
			inserted++
		} else if errors.Is(err, interfaces.ErrEventAlreadyProcessed) {
			sentinels++
		} else {
			s.Failf("unexpected error", "unexpected error on replay: %v", err)
		}
	}

	s.Assert().Equal(1, inserted, "RF-11: exactly 1 INSERT expected")
	s.Assert().Equal(2, sentinels, "RF-11: exactly 2 sentinels expected")
}
