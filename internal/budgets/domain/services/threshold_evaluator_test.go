package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdEvaluatorSuite struct {
	suite.Suite
}

func TestThresholdEvaluatorSuite(t *testing.T) {
	suite.Run(t, new(ThresholdEvaluatorSuite))
}

func noCrossed() map[valueobjects.Threshold]bool {
	return map[valueobjects.Threshold]bool{
		valueobjects.Threshold80:  false,
		valueobjects.Threshold100: false,
	}
}

func bothCrossed() map[valueobjects.Threshold]bool {
	return map[valueobjects.Threshold]bool{
		valueobjects.Threshold80:  true,
		valueobjects.Threshold100: true,
	}
}

func (s *ThresholdEvaluatorSuite) TestEvaluateThresholds() {
	type testCase struct {
		name              string
		spent             int64
		planned           int64
		currentlyCrossed  map[valueobjects.Threshold]bool
		wantErr           bool
		want80Crossed     bool
		want100Crossed    bool
		want80Transition  bool
		want100Transition bool
	}

	cases := []testCase{
		{
			name:              "gasto zero — nenhum limiar cruzado",
			spent:             0,
			planned:           1000,
			currentlyCrossed:  noCrossed(),
			want80Crossed:     false,
			want100Crossed:    false,
			want80Transition:  false,
			want100Transition: false,
		},
		{
			name:              "exatamente 80% — cruza 80",
			spent:             800,
			planned:           1000,
			currentlyCrossed:  noCrossed(),
			want80Crossed:     true,
			want100Crossed:    false,
			want80Transition:  true,
			want100Transition: false,
		},
		{
			name:              "exatamente 100% — cruza 100",
			spent:             1000,
			planned:           1000,
			currentlyCrossed:  noCrossed(),
			want80Crossed:     true,
			want100Crossed:    true,
			want80Transition:  true,
			want100Transition: true,
		},
		{
			name:              "acima de 100% — ambos cruzados",
			spent:             1500,
			planned:           1000,
			currentlyCrossed:  noCrossed(),
			want80Crossed:     true,
			want100Crossed:    true,
			want80Transition:  true,
			want100Transition: true,
		},
		{
			name:              "79% — nenhum limiar cruzado",
			spent:             790,
			planned:           1000,
			currentlyCrossed:  noCrossed(),
			want80Crossed:     false,
			want100Crossed:    false,
			want80Transition:  false,
			want100Transition: false,
		},
		{
			name:              "gasto volta abaixo de 80 — rearma 80",
			spent:             700,
			planned:           1000,
			currentlyCrossed:  bothCrossed(),
			want80Crossed:     false,
			want100Crossed:    false,
			want80Transition:  true,
			want100Transition: true,
		},
		{
			name:    "permanece cruzado — sem transição",
			spent:   900,
			planned: 1000,
			currentlyCrossed: map[valueobjects.Threshold]bool{
				valueobjects.Threshold80:  true,
				valueobjects.Threshold100: false,
			},
			want80Crossed:     true,
			want100Crossed:    false,
			want80Transition:  false,
			want100Transition: false,
		},
		{
			name:             "planned zero — erro",
			spent:            100,
			planned:          0,
			currentlyCrossed: noCrossed(),
			wantErr:          true,
		},
		{
			name:             "planned negativo — erro",
			spent:            100,
			planned:          -1,
			currentlyCrossed: noCrossed(),
			wantErr:          true,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			transitions, err := services.ThresholdEvaluator{}.EvaluateThresholds(tc.spent, tc.planned, tc.currentlyCrossed)
			if tc.wantErr {
				s.Error(err)
				s.ErrorIs(err, services.ErrThresholdPlannedZero)
				return
			}
			s.NoError(err)
			s.Len(transitions, 2)

			t80 := findTransition(transitions, valueobjects.Threshold80)
			t100 := findTransition(transitions, valueobjects.Threshold100)

			s.NotNil(t80)
			s.NotNil(t100)

			s.Equal(tc.want80Crossed, t80.NowCrossed, "80%% NowCrossed")
			s.Equal(tc.want100Crossed, t100.NowCrossed, "100%% NowCrossed")
			s.Equal(tc.want80Transition, t80.IsRealTransition, "80%% IsRealTransition")
			s.Equal(tc.want100Transition, t100.IsRealTransition, "100%% IsRealTransition")
		})
	}
}

func findTransition(ts []services.Transition, t valueobjects.Threshold) *services.Transition {
	for i := range ts {
		if ts[i].Threshold == t {
			return &ts[i]
		}
	}
	return nil
}
