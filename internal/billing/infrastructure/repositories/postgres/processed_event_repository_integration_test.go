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

func (s *ProcessedEventRepositorySuite) SetupTest() {}

func (s *ProcessedEventRepositorySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *ProcessedEventRepositorySuite) newRepo() interfaces.ProcessedEventRepository {
	return s.factory.ProcessedEventRepository(s.mgr.DBTX(context.Background()))
}

func (s *ProcessedEventRepositorySuite) TestMarkApplied() {
	scenarios := []struct {
		name      string
		setup     func() (string, string, string, time.Time)
		expectErr error
		repeat    bool
	}{
		{
			name: "deve inserir a linha do evento processado",
			setup: func() (string, string, string, time.Time) {
				return "order_approved:order-pe-test-001", "order_approved", "order-pe-test-001", time.Now().UTC()
			},
		},
		{
			name: "deve retornar sentinela quando o evento ja foi processado",
			setup: func() (string, string, string, time.Time) {
				return "order_approved:order-pe-dup-001", "order_approved", "order-pe-dup-001", time.Now().UTC()
			},
			expectErr: interfaces.ErrEventAlreadyProcessed,
			repeat:    true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			eventKey, trigger, recursoID, occurredAt := scenario.setup()

			err := repo.MarkApplied(ctx, eventKey, trigger, recursoID, occurredAt)
			s.Require().NoError(err)

			if !scenario.repeat {
				return
			}

			err = repo.MarkApplied(ctx, eventKey, trigger, recursoID, occurredAt)
			s.Require().Error(err)
			s.Assert().True(errors.Is(err, scenario.expectErr), "expected %v, got: %v", scenario.expectErr, err)
		})
	}
}

func (s *ProcessedEventRepositorySuite) TestMarkSuperseded() {
	scenarios := []struct {
		name string
	}{
		{name: "deve atualizar o status para superseded"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.newRepo()
			eventKey := "subscription_late:sub-superseded-001:2026-06-01T00:00:00Z"
			s.Require().NoError(repo.MarkApplied(ctx, eventKey, "subscription_late", "sub-superseded-001", time.Now().UTC()))
			s.Require().NoError(repo.MarkSuperseded(ctx, eventKey))
		})
	}
}

type RF11ReplaySuite struct {
	suite.Suite
	mgr     manager.Manager
	factory interfaces.RepositoryFactory
}

func TestRF11ReplaySuite(t *testing.T) {
	suite.Run(t, new(RF11ReplaySuite))
}

func (s *RF11ReplaySuite) SetupTest() {}

func (s *RF11ReplaySuite) SetupSuite() {
	s.mgr = setupTestDB(s.T())
	s.factory = billingrepos.NewRepositoryFactory(noop.NewProvider())
}

func (s *RF11ReplaySuite) TestRF11_ThreeReplaysYieldOneInsertAndTwoSentinels() {
	scenarios := []struct {
		name string
	}{
		{name: "deve produzir um insert e duas sentinelas em tres replays"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			repo := s.factory.ProcessedEventRepository(s.mgr.DBTX(ctx))
			eventKey := "order_approved:order-rf11-replay-001"
			occurredAt := time.Now().UTC()

			var errs []error
			for range 3 {
				err := repo.MarkApplied(ctx, eventKey, "order_approved", "order-rf11-replay-001", occurredAt)
				errs = append(errs, err)
			}

			inserted := 0
			sentinels := 0
			for _, err := range errs {
				if err == nil {
					inserted++
					continue
				}
				if errors.Is(err, interfaces.ErrEventAlreadyProcessed) {
					sentinels++
					continue
				}
				s.Failf("unexpected error", "unexpected error on replay: %v", err)
			}

			s.Assert().Equal(1, inserted, "RF-11: exactly 1 INSERT expected")
			s.Assert().Equal(2, sentinels, "RF-11: exactly 2 sentinels expected")
		})
	}
}
