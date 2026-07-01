//go:build integration

package persistence_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type WriteLedgerIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	repo usecases.WriteLedgerRepository
}

func TestWriteLedgerIntegrationSuite(t *testing.T) {
	suite.Run(t, new(WriteLedgerIntegrationSuite))
}

func (s *WriteLedgerIntegrationSuite) SetupSuite() {
	s.ctx = context.Background()
	db, _ := testcontainer.Postgres(s.T())
	s.repo = persistence.NewWriteLedgerRepository(db, fake.NewProvider())
}

func (s *WriteLedgerIntegrationSuite) TestInsertAndFindByKey() {
	userID := uuid.New()
	wamid := "wamid-" + uuid.NewString()
	resourceID := uuid.New()
	entry := usecases.WriteLedgerEntry{
		ID:           uuid.New(),
		UserID:       userID,
		WAMID:        wamid,
		ItemSeq:      0,
		Operation:    "create_transaction",
		ResourceID:   resourceID,
		ResourceKind: "transaction",
		CreatedAt:    time.Now().UTC(),
	}

	s.Require().NoError(s.repo.Insert(s.ctx, entry))

	found, err := s.repo.FindByKey(s.ctx, wamid, 0, "create_transaction")
	s.Require().NoError(err)
	s.Equal(resourceID, found.ResourceID)
	s.Equal("create_transaction", found.Operation)
	s.Equal(0, found.ItemSeq)
}

func (s *WriteLedgerIntegrationSuite) TestFindByKeyNotFound() {
	_, err := s.repo.FindByKey(s.ctx, "nonexistent-wamid", 0, "create_transaction")
	s.ErrorIs(err, usecases.ErrLedgerEntryNotFound)
}

func (s *WriteLedgerIntegrationSuite) TestInsertIdempotentOnConflict() {
	userID := uuid.New()
	wamid := "wamid-idem-" + uuid.NewString()
	resourceID := uuid.New()
	entry := usecases.WriteLedgerEntry{
		ID:           uuid.New(),
		UserID:       userID,
		WAMID:        wamid,
		ItemSeq:      0,
		Operation:    "create_transaction",
		ResourceID:   resourceID,
		ResourceKind: "transaction",
		CreatedAt:    time.Now().UTC(),
	}

	s.Require().NoError(s.repo.Insert(s.ctx, entry))

	entry2 := entry
	entry2.ID = uuid.New()
	s.Require().NoError(s.repo.Insert(s.ctx, entry2))

	found, err := s.repo.FindByKey(s.ctx, wamid, 0, "create_transaction")
	s.Require().NoError(err)
	s.Equal(resourceID, found.ResourceID)
}

func (s *WriteLedgerIntegrationSuite) TestUniqueConstraintUnderConcurrency() {
	wamid := "wamid-race-" + uuid.NewString()
	resourceID := uuid.New()
	userID := uuid.New()

	const goroutines = 10
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			e := usecases.WriteLedgerEntry{
				ID:           uuid.New(),
				UserID:       userID,
				WAMID:        wamid,
				ItemSeq:      0,
				Operation:    "create_transaction",
				ResourceID:   resourceID,
				ResourceKind: "transaction",
				CreatedAt:    time.Now().UTC(),
			}
			errs[idx] = s.repo.Insert(s.ctx, e)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		s.NoError(err, "ON CONFLICT DO NOTHING deve absorver conflitos")
	}

	found, err := s.repo.FindByKey(s.ctx, wamid, 0, "create_transaction")
	s.Require().NoError(err)
	s.Equal(resourceID, found.ResourceID)
}

func (s *WriteLedgerIntegrationSuite) TestMultipleItemSeqSameWamid() {
	wamid := "wamid-multi-" + uuid.NewString()
	userID := uuid.New()
	resource0 := uuid.New()
	resource1 := uuid.New()

	s.Require().NoError(s.repo.Insert(s.ctx, usecases.WriteLedgerEntry{
		ID: uuid.New(), UserID: userID, WAMID: wamid, ItemSeq: 0,
		Operation: "create_transaction", ResourceID: resource0, ResourceKind: "transaction",
		CreatedAt: time.Now().UTC(),
	}))

	s.Require().NoError(s.repo.Insert(s.ctx, usecases.WriteLedgerEntry{
		ID: uuid.New(), UserID: userID, WAMID: wamid, ItemSeq: 1,
		Operation: "create_transaction", ResourceID: resource1, ResourceKind: "transaction",
		CreatedAt: time.Now().UTC(),
	}))

	found0, err := s.repo.FindByKey(s.ctx, wamid, 0, "create_transaction")
	s.Require().NoError(err)
	s.Equal(resource0, found0.ResourceID)

	found1, err := s.repo.FindByKey(s.ctx, wamid, 1, "create_transaction")
	s.Require().NoError(err)
	s.Equal(resource1, found1.ResourceID)
}

func (s *WriteLedgerIntegrationSuite) TestDeleteBefore() {
	wamid := "wamid-del-" + uuid.NewString()
	userID := uuid.New()
	old := time.Now().UTC().Add(-48 * time.Hour)
	recent := time.Now().UTC()

	s.Require().NoError(s.repo.Insert(s.ctx, usecases.WriteLedgerEntry{
		ID: uuid.New(), UserID: userID, WAMID: wamid, ItemSeq: 0,
		Operation: "create_transaction", ResourceID: uuid.New(), ResourceKind: "transaction",
		CreatedAt: old,
	}))
	s.Require().NoError(s.repo.Insert(s.ctx, usecases.WriteLedgerEntry{
		ID: uuid.New(), UserID: userID, WAMID: wamid, ItemSeq: 1,
		Operation: "create_transaction", ResourceID: uuid.New(), ResourceKind: "transaction",
		CreatedAt: recent,
	}))

	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	deleted, err := s.repo.DeleteBefore(s.ctx, cutoff, 1000)
	s.Require().NoError(err)
	s.GreaterOrEqual(deleted, int64(1))

	_, err = s.repo.FindByKey(s.ctx, wamid, 0, "create_transaction")
	s.ErrorIs(err, usecases.ErrLedgerEntryNotFound)
}
