package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/clock"
)

// ClockSuite testa SystemClock e FakeClock.
type ClockSuite struct {
	suite.Suite
	ctx context.Context
}

func TestClock(t *testing.T) {
	suite.Run(t, new(ClockSuite))
}

func (s *ClockSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ClockSuite) TestSystemClock() {
	scenarios := []struct {
		name   string
		expect func()
	}{
		{
			name: "deve retornar instante monotônico crescente",
			expect: func() {
				clk := clock.NewSystemClock()
				t1 := clk.Now()
				time.Sleep(time.Millisecond)
				t2 := clk.Now()
				s.True(t2.After(t1) || t2.Equal(t1),
					"SystemClock.Now() deve retornar instante >= chamada anterior")
			},
		},
		{
			name: "deve retornar instante em UTC",
			expect: func() {
				clk := clock.NewSystemClock()
				s.Equal(time.UTC, clk.Now().Location())
			},
		},
		{
			name: "deve implementar interface Clock",
			expect: func() {
				var _ clock.Clock = clock.SystemClock{}
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect()
		})
	}
}

func (s *ClockSuite) TestFakeClock() {
	scenarios := []struct {
		name   string
		expect func()
	}{
		{
			name: "deve retornar instante fixo sem Advance",
			expect: func() {
				fixed := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
				clk := clock.NewFakeClock(fixed)
				s.Equal(fixed, clk.Now())
				s.Equal(fixed, clk.Now())
			},
		},
		{
			name: "deve usar 2024-01-01T00:00:00Z quando zero value passado",
			expect: func() {
				clk := clock.NewFakeClock(time.Time{})
				expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				s.Equal(expected, clk.Now())
			},
		},
		{
			name: "deve avançar o instante com Advance",
			expect: func() {
				fixed := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				clk := clock.NewFakeClock(fixed)
				clk.Advance(time.Hour)
				s.Equal(fixed.Add(time.Hour), clk.Now())
				clk.Advance(30 * time.Minute)
				s.Equal(fixed.Add(time.Hour+30*time.Minute), clk.Now())
			},
		},
		{
			name: "deve definir instante absoluto com Set",
			expect: func() {
				clk := clock.NewFakeClock(time.Time{})
				newTime := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
				clk.Set(newTime)
				s.Equal(newTime, clk.Now())
			},
		},
		{
			name: "deve implementar interface Clock",
			expect: func() {
				var _ clock.Clock = (*clock.FakeClock)(nil)
			},
		},
		{
			name: "deve suportar Advance concorrente sem data race",
			expect: func() {
				clk := clock.NewFakeClock(time.Time{})
				done := make(chan struct{})
				go func() {
					defer close(done)
					for range 100 {
						clk.Advance(time.Millisecond)
					}
				}()
				for range 100 {
					_ = clk.Now()
				}
				<-done
			},
		},
	}
	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			sc.expect()
		})
	}
}
