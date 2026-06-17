package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeCreateBudget struct {
	capturedInput input.CreateBudgetInput
	err           error
}

func (f *fakeCreateBudget) Execute(_ context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error) {
	f.capturedInput = in
	return output.BudgetOutput{}, f.err
}

type fakeActivateBudget struct {
	capturedInput input.ActivateBudgetInput
	called        bool
	err           error
}

func (f *fakeActivateBudget) Execute(_ context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error) {
	f.called = true
	f.capturedInput = in
	return output.BudgetOutput{}, f.err
}

type onboardingBudgetConsumerSuite struct {
	suite.Suite
}

func TestOnboardingBudgetConsumer(t *testing.T) {
	suite.Run(t, new(onboardingBudgetConsumerSuite))
}

func (s *onboardingBudgetConsumerSuite) buildEnvelope(userID uuid.UUID, incomeCents int64, allocations []map[string]any) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"UserID":      userID.String(),
		"IncomeCents": incomeCents,
		"Allocations": allocations,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *onboardingBudgetConsumerSuite) TestHappyPath_DefaultSplit() {
	userID := uuid.New()
	allocations := []map[string]any{
		{"Kind": "fixed_cost", "Percent": 40},
		{"Kind": "knowledge", "Percent": 10},
		{"Kind": "pleasures", "Percent": 15},
		{"Kind": "goals", "Percent": 20},
		{"Kind": "financial_freedom", "Percent": 15},
	}
	env := s.buildEnvelope(userID, 350000, allocations)

	createUC := &fakeCreateBudget{}
	activateUC := &fakeActivateBudget{}
	consumer := consumers.NewOnboardingBudgetConsumer(createUC, activateUC, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
	s.Require().NoError(err)

	s.Equal(userID.String(), createUC.capturedInput.UserID)
	s.Equal(int64(350000), createUC.capturedInput.TotalCents)
	s.Len(createUC.capturedInput.Allocations, 5)

	expectedSlugs := map[string]int{
		"expense.custo_fixo":           4000,
		"expense.conhecimento":         1000,
		"expense.prazeres":             1500,
		"expense.metas":                2000,
		"expense.liberdade_financeira": 1500,
	}
	for _, a := range createUC.capturedInput.Allocations {
		expected, ok := expectedSlugs[a.RootSlug]
		s.Require().True(ok, "slug inesperado: %s", a.RootSlug)
		s.Equal(expected, a.BasisPoints, "basisPoints errado para slug %s", a.RootSlug)
	}

	s.True(activateUC.called)
	s.Equal(userID.String(), activateUC.capturedInput.UserID)
}

func (s *onboardingBudgetConsumerSuite) TestIdempotency_ConflictOnCreate_NoActivate() {
	userID := uuid.New()
	allocations := []map[string]any{
		{"Kind": "fixed_cost", "Percent": 40},
		{"Kind": "knowledge", "Percent": 10},
		{"Kind": "pleasures", "Percent": 15},
		{"Kind": "goals", "Percent": 20},
		{"Kind": "financial_freedom", "Percent": 15},
	}
	env := s.buildEnvelope(userID, 350000, allocations)

	createUC := &fakeCreateBudget{err: appinterfaces.ErrBudgetConflict}
	activateUC := &fakeActivateBudget{}
	consumer := consumers.NewOnboardingBudgetConsumer(createUC, activateUC, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
	s.Require().NoError(err)
	s.False(activateUC.called)
}

func (s *onboardingBudgetConsumerSuite) TestUnknownKind_ReturnsError() {
	userID := uuid.New()
	allocations := []map[string]any{
		{"Kind": "unknown_kind", "Percent": 100},
	}
	env := s.buildEnvelope(userID, 100000, allocations)

	consumer := consumers.NewOnboardingBudgetConsumer(&fakeCreateBudget{}, &fakeActivateBudget{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
	s.Require().Error(err)
}

func (s *onboardingBudgetConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	consumer := consumers.NewOnboardingBudgetConsumer(&fakeCreateBudget{}, &fakeActivateBudget{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: "not-envelope"})
	s.Require().Error(err)
}

func (s *onboardingBudgetConsumerSuite) TestInvalidUserID_ReturnsError() {
	raw, _ := json.Marshal(map[string]any{
		"UserID":      "not-a-uuid",
		"IncomeCents": 100000,
		"Allocations": []map[string]any{},
	})
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}
	consumer := consumers.NewOnboardingBudgetConsumer(&fakeCreateBudget{}, &fakeActivateBudget{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
	s.Require().Error(err)
}

func (s *onboardingBudgetConsumerSuite) TestCreateBudgetError_Propagated() {
	userID := uuid.New()
	allocations := []map[string]any{
		{"Kind": "fixed_cost", "Percent": 100},
	}
	env := s.buildEnvelope(userID, 100000, allocations)

	createUC := &fakeCreateBudget{err: errors.New("infra error")}
	consumer := consumers.NewOnboardingBudgetConsumer(createUC, &fakeActivateBudget{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
	s.Require().Error(err)
}

func (s *onboardingBudgetConsumerSuite) TestMapCategoryKindToRootSlug_AllFiveMapped() {
	cases := []struct {
		kind string
		slug string
	}{
		{"fixed_cost", "expense.custo_fixo"},
		{"knowledge", "expense.conhecimento"},
		{"pleasures", "expense.prazeres"},
		{"goals", "expense.metas"},
		{"financial_freedom", "expense.liberdade_financeira"},
	}
	for _, tc := range cases {
		userID := uuid.New()
		allocations := []map[string]any{{"Kind": tc.kind, "Percent": 100}}
		env := s.buildEnvelope(userID, 100000, allocations)
		createUC := &fakeCreateBudget{}
		consumer := consumers.NewOnboardingBudgetConsumer(createUC, &fakeActivateBudget{}, noop.NewProvider())
		err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.splits_calculated", payload: env})
		s.Require().NoError(err, "kind=%s deve mapear sem erro", tc.kind)
		s.Require().Len(createUC.capturedInput.Allocations, 1)
		s.Equal(tc.slug, createUC.capturedInput.Allocations[0].RootSlug, "kind=%s", tc.kind)
	}
}
