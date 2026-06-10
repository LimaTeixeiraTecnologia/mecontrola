package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExpenseTombstoneSuite struct {
	suite.Suite
	now time.Time
}

func TestExpenseTombstoneSuite(t *testing.T) {
	suite.Run(t, new(ExpenseTombstoneSuite))
}

func (s *ExpenseTombstoneSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
}

func (s *ExpenseTombstoneSuite) TestNewExpenseTombstone() {
	src, _ := valueobjects.NewProducerSource("api")
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	userID := uuid.New()

	t := entities.NewExpenseTombstone(userID, src, ext, 2, s.now)
	s.Equal(userID, t.UserID())
	s.Equal(src, t.Source())
	s.Equal(ext, t.ExternalTransactionID())
	s.Equal(int64(2), t.TombstoneVersion())
	s.Equal(s.now, t.DeletedAt())
	s.True(t.IsPresent())
}

func (s *ExpenseTombstoneSuite) TestZeroTombstoneNotPresent() {
	src, _ := valueobjects.NewProducerSource("api")
	ext, _ := valueobjects.NewExternalTransactionID("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	t := entities.NewExpenseTombstone(uuid.New(), src, ext, 0, s.now)
	s.False(t.IsPresent())
}
