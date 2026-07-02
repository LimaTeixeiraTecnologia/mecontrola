package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

type PurchaseDaySuite struct {
	suite.Suite
	svc services.PurchaseDayService
	tz  *time.Location
}

func TestPurchaseDaySuite(t *testing.T) {
	suite.Run(t, new(PurchaseDaySuite))
}

func (s *PurchaseDaySuite) SetupTest() {
	s.svc = services.PurchaseDayService{}
	var err error
	s.tz, err = time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
}

func (s *PurchaseDaySuite) TestDecide_Nubank_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Itau_Due10_Days8() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(10, 8, now, s.tz)
	s.Equal(2, pd.ClosingDay)
	s.Equal(3, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Bradesco_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_BancoDoBrasil_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Caixa_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Inter_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_C6Bank_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Santander_Due10_Days8() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(10, 8, now, s.tz)
	s.Equal(2, pd.ClosingDay)
	s.Equal(3, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Fallback_Due20_Days7() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Wrap_DueLow_CrossesMonthBoundary() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(3, 7, now, s.tz)
	s.Equal(27, pd.ClosingDay)
	s.Equal(28, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_ShortMonth_Clamps() {
	now := time.Date(2026, 2, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(31, 7, now, s.tz)
	s.Equal(21, pd.ClosingDay)
	s.Equal(22, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_Due31InJune_Clamps() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(31, 7, now, s.tz)
	s.Equal(23, pd.ClosingDay)
	s.Equal(24, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_ClosingOnDay30_BestIs31() {
	now := time.Date(2026, 5, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(7, 7, now, s.tz)
	s.Equal(30, pd.ClosingDay)
	s.Equal(31, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_ClosingOnDay31_BestWrapsTo1() {
	now := time.Date(2026, 2, 15, 0, 0, 0, 0, s.tz)
	pd := s.svc.Decide(7, 7, now, s.tz)
	s.Equal(31, pd.ClosingDay)
	s.Equal(1, pd.BestDay)
}

func (s *PurchaseDaySuite) TestDecide_IsDeterministic() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, s.tz)
	a := s.svc.Decide(20, 7, now, s.tz)
	b := s.svc.Decide(20, 7, now, s.tz)
	s.Equal(a, b)
}

func (s *PurchaseDaySuite) TestDecide_UTC_Timezone() {
	now := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	pd := s.svc.Decide(20, 7, now, time.UTC)
	s.Equal(13, pd.ClosingDay)
	s.Equal(14, pd.BestDay)
}
