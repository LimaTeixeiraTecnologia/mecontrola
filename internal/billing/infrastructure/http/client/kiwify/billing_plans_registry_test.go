package kiwify_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

type stubPlansLoader struct {
	plans map[string]valueobjects.PlanCode
	err   error
}

func (s *stubPlansLoader) LoadKiwifyProductPlans(_ context.Context) (map[string]valueobjects.PlanCode, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.plans, nil
}

type BillingPlansRegistrySuite struct {
	suite.Suite
}

func TestBillingPlansRegistry(t *testing.T) {
	suite.Run(t, new(BillingPlansRegistrySuite))
}

func (s *BillingPlansRegistrySuite) TestNewFromLoader_Success() {
	loader := &stubPlansLoader{
		plans: map[string]valueobjects.PlanCode{
			"prod-monthly": valueobjects.PlanCodeMonthly,
		},
	}
	registry, err := kiwify.NewBillingPlansRegistry(context.Background(), loader)
	s.Require().NoError(err)
	plan, err := registry.ParsePlanCodeFromKiwifyProductID("prod-monthly")
	s.Require().NoError(err)
	s.Equal(valueobjects.PlanCodeMonthly, plan)
}

func (s *BillingPlansRegistrySuite) TestNewFromLoader_Error() {
	loader := &stubPlansLoader{err: errors.New("db unavailable")}
	_, err := kiwify.NewBillingPlansRegistry(context.Background(), loader)
	s.Require().Error(err)
}

func (s *BillingPlansRegistrySuite) TestParsePlanCode_NotFound() {
	registry := kiwify.NewBillingPlansRegistryFromMap(map[string]valueobjects.PlanCode{})
	_, err := registry.ParsePlanCodeFromKiwifyProductID("missing-id")
	s.Require().Error(err)
	s.ErrorIs(err, kiwify.ErrPlanNotFound)
}

func (s *BillingPlansRegistrySuite) TestParsePlanCode_AllPlans() {
	registry := kiwify.NewBillingPlansRegistryFromMap(map[string]valueobjects.PlanCode{
		"pm": valueobjects.PlanCodeMonthly,
		"pq": valueobjects.PlanCodeQuarterly,
		"pa": valueobjects.PlanCodeAnnual,
	})

	type testCase struct {
		id       string
		expected valueobjects.PlanCode
	}
	cases := []testCase{
		{"pm", valueobjects.PlanCodeMonthly},
		{"pq", valueobjects.PlanCodeQuarterly},
		{"pa", valueobjects.PlanCodeAnnual},
	}
	for _, tc := range cases {
		s.Run(tc.id, func() {
			plan, err := registry.ParsePlanCodeFromKiwifyProductID(tc.id)
			s.Require().NoError(err)
			s.Equal(tc.expected, plan)
		})
	}
}
