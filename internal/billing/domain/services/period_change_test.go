package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
)

type PeriodChangeSuite struct {
	suite.Suite
}

func TestPeriodChangeSuite(t *testing.T) {
	suite.Run(t, new(PeriodChangeSuite))
}

func (s *PeriodChangeSuite) TestAdvancesPeriodTrue() {
	t1 := time.Now()
	t2 := t1.Add(30 * 24 * time.Hour)
	pc := services.PeriodChange{NewStart: t1, NewEnd: t2}
	s.True(pc.AdvancesPeriod())
}

func (s *PeriodChangeSuite) TestAdvancesPeriodFalseWhenStartZero() {
	pc := services.PeriodChange{NewEnd: time.Now()}
	s.False(pc.AdvancesPeriod())
}

func (s *PeriodChangeSuite) TestAdvancesPeriodFalseWhenEndZero() {
	pc := services.PeriodChange{NewStart: time.Now()}
	s.False(pc.AdvancesPeriod())
}

func (s *PeriodChangeSuite) TestNoPeriodChange() {
	pc := services.NoPeriodChange()
	s.False(pc.AdvancesPeriod())
	s.True(pc.NewStart.IsZero())
	s.True(pc.NewEnd.IsZero())
}
