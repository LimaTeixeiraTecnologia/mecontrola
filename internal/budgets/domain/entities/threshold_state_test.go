package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdStateSuite struct {
	suite.Suite
	now time.Time
}

func TestThresholdStateSuite(t *testing.T) {
	suite.Run(t, new(ThresholdStateSuite))
}

func (s *ThresholdStateSuite) SetupTest() {
	s.now = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
}

func (s *ThresholdStateSuite) TestNewThresholdState() {
	comp, _ := valueobjects.NewCompetence("2025-06")
	key := entities.ThresholdKey{
		UserID:     uuid.New(),
		Competence: comp,
		RootSlug:   valueobjects.RootSlugCustoFixo,
		Threshold:  valueobjects.Threshold80,
	}
	ts := entities.NewThresholdState(key)
	s.Equal(key, ts.Key())
	s.False(ts.CurrentlyCrossed())
	s.Equal(int64(0), ts.Version())
	s.Nil(ts.LastCrossedAt())
	s.Nil(ts.LastUncrossedAt())
	s.Nil(ts.LastEvaluatedCommittedAt())
}

func (s *ThresholdStateSuite) TestHydrateThresholdState() {
	comp, _ := valueobjects.NewCompetence("2025-06")
	key := entities.ThresholdKey{
		UserID:     uuid.New(),
		Competence: comp,
		RootSlug:   valueobjects.RootSlugMetas,
		Threshold:  valueobjects.Threshold100,
	}
	crossed := s.now
	uncrossed := s.now.Add(time.Hour)
	evaluated := s.now.Add(2 * time.Hour)
	ts := entities.HydrateThresholdState(key, true, 3, &crossed, &uncrossed, &evaluated)
	s.Equal(key, ts.Key())
	s.True(ts.CurrentlyCrossed())
	s.Equal(int64(3), ts.Version())
	s.Equal(&crossed, ts.LastCrossedAt())
	s.Equal(&uncrossed, ts.LastUncrossedAt())
	s.Equal(&evaluated, ts.LastEvaluatedCommittedAt())
}
