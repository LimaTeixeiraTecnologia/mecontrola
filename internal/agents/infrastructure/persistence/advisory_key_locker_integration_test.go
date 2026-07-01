//go:build integration

package persistence_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AdvisoryKeyLockerIntegrationSuite struct {
	suite.Suite
	ctx    context.Context
	ledger usecases.WriteLedgerRepository
	idem   *usecases.IdempotentWrite
}

func TestAdvisoryKeyLockerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AdvisoryKeyLockerIntegrationSuite))
}

func (s *AdvisoryKeyLockerIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	obs := fake.NewProvider()
	s.ledger = persistence.NewWriteLedgerRepository(db, obs)
	locker := persistence.NewAdvisoryKeyLocker(db, obs)
	s.idem = usecases.NewIdempotentWrite(s.ledger, obs, usecases.WithKeyLocker(locker))
}

func (s *AdvisoryKeyLockerIntegrationSuite) TestConcurrentSameKeyWritesExactlyOnce() {
	userID := uuid.New()
	wamid := "wamid-race-" + uuid.NewString()
	const operation = "create_transaction"

	var writeCalls int64
	writeFn := func(_ context.Context) (uuid.UUID, bool, error) {
		atomic.AddInt64(&writeCalls, 1)
		return uuid.New(), false, nil
	}

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]usecases.IdempotentWriteResult, goroutines)
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = s.idem.Execute(
				s.ctx,
				userID,
				wamid,
				0,
				operation,
				"transaction",
				writeFn,
			)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		s.Require().NoError(err)
	}

	s.Equal(int64(1), atomic.LoadInt64(&writeCalls), "advisory lock deve serializar e executar a escrita de domínio exatamente uma vez")

	entry, err := s.ledger.FindByKey(s.ctx, wamid, 0, operation)
	s.Require().NoError(err)

	var created, replay int
	for _, r := range results {
		s.Equal(entry.ResourceID, r.ResourceID)
		switch r.Outcome.String() {
		case "routed":
			created++
		case "replay":
			replay++
		}
	}
	s.Equal(1, created, "exatamente um racer cria")
	s.Equal(goroutines-1, replay, "os demais racers viram replay")
}
