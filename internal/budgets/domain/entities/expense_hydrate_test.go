package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseHydrateSuite struct {
	suite.Suite
	now  time.Time
	src  valueobjects.ProducerSource
	ext  valueobjects.ExternalTransactionID
	comp valueobjects.Competence
}

func TestExpenseHydrateSuite(t *testing.T) {
	suite.Run(t, new(ExpenseHydrateSuite))
}

func (s *ExpenseHydrateSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	src, _ := valueobjects.NewProducerSource("api")
	s.src = src
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	s.ext = ext
	c, _ := valueobjects.NewCompetence("2025-06")
	s.comp = c
}

func (s *ExpenseHydrateSuite) TestHydrateExpense() {
	id := uuid.New()
	subID := uuid.New()
	tombV := int64(2)
	deleted := s.now.Add(time.Hour)

	e := entities.HydrateExpense(
		id, uuid.New(), s.src, s.ext, subID,
		valueobjects.RootSlugPrazeres, s.comp,
		500, s.now, 2, &tombV, &deleted, s.now, s.now,
	)

	s.Equal(id, e.ID())
	s.Equal(s.src, e.Source())
	s.Equal(s.ext, e.ExternalTransactionID())
	s.Equal(subID, e.SubcategoryID())
	s.Equal(valueobjects.RootSlugPrazeres, e.RootSlug())
	s.Equal(s.comp, e.Competence())
	s.Equal(int64(500), e.AmountCents())
	s.Equal(s.now, e.OccurredAt())
	s.Equal(int64(2), e.Version())
	s.NotNil(e.TombstoneVersion())
	s.Equal(int64(2), *e.TombstoneVersion())
	s.NotNil(e.DeletedAt())
	s.True(e.IsDeleted())
	s.Equal(s.now, e.CreatedAt())
	s.Equal(s.now, e.UpdatedAt())
}

func (s *ExpenseHydrateSuite) TestExpenseIdentity() {
	userID := uuid.New()
	e, err := entities.NewExpense(
		userID, s.src, s.ext, uuid.New(),
		valueobjects.RootSlugMetas, s.comp, 100, s.now, s.now,
	)
	s.Require().NoError(err)
	identity := e.Identity()
	s.Equal(userID, identity.UserID)
	s.Equal(s.src, identity.Source)
	s.Equal(s.ext, identity.ExternalTransactionID)
}

func (s *ExpenseHydrateSuite) TestEditAlreadyDeleted() {
	e, err := entities.NewExpense(
		uuid.New(), s.src, s.ext, uuid.New(),
		valueobjects.RootSlugCustoFixo, s.comp, 100, s.now, s.now,
	)
	s.Require().NoError(err)
	_, _ = e.SoftDelete(1, s.now)
	editErr := e.Edit(uuid.New(), valueobjects.RootSlugMetas, s.comp, 200, s.now, 2, s.now)
	s.ErrorIs(editErr, entities.ErrExpenseAlreadyDeleted)
}
