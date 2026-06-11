package services_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExternalExpenseStrategySuite struct {
	suite.Suite
}

func TestExternalExpenseStrategySuite(t *testing.T) {
	suite.Run(t, new(ExternalExpenseStrategySuite))
}

func (s *ExternalExpenseStrategySuite) TestPlan() {
	type tc struct {
		name           string
		kind           valueobjects.MutationKind
		currentVersion int64
		currentExists  bool
		eventVersion   int64
		want           services.ExternalExpenseAction
	}

	cases := []tc{
		{
			name:          "create sem despesa e version=1 — Create",
			kind:          valueobjects.MutationKindCreate,
			currentExists: false,
			eventVersion:  1,
			want:          services.ActionCreate,
		},
		{
			name:          "create com despesa existente — Noop",
			kind:          valueobjects.MutationKindCreate,
			currentExists: true,
			eventVersion:  1,
			want:          services.ActionNoop,
		},
		{
			name:          "create com version != 1 — QueuePending",
			kind:          valueobjects.MutationKindCreate,
			currentExists: false,
			eventVersion:  2,
			want:          services.ActionQueuePending,
		},
		{
			name:          "update sem despesa atual — QueuePending",
			kind:          valueobjects.MutationKindUpdate,
			currentExists: false,
			eventVersion:  2,
			want:          services.ActionQueuePending,
		},
		{
			name:           "update consecutivo — Update",
			kind:           valueobjects.MutationKindUpdate,
			currentExists:  true,
			currentVersion: 2,
			eventVersion:   3,
			want:           services.ActionUpdate,
		},
		{
			name:           "update obsoleto — Noop",
			kind:           valueobjects.MutationKindUpdate,
			currentExists:  true,
			currentVersion: 5,
			eventVersion:   3,
			want:           services.ActionNoop,
		},
		{
			name:           "update gap — QueuePending",
			kind:           valueobjects.MutationKindUpdate,
			currentExists:  true,
			currentVersion: 2,
			eventVersion:   5,
			want:           services.ActionQueuePending,
		},
		{
			name:           "delete consecutivo — Delete",
			kind:           valueobjects.MutationKindDelete,
			currentExists:  true,
			currentVersion: 2,
			eventVersion:   3,
			want:           services.ActionDelete,
		},
		{
			name:          "delete sem despesa — QueuePending",
			kind:          valueobjects.MutationKindDelete,
			currentExists: false,
			eventVersion:  3,
			want:          services.ActionQueuePending,
		},
	}

	strat := services.NewExternalExpenseStrategy()
	for _, c := range cases {
		s.Run(c.name, func() {
			got := strat.Plan(c.kind, c.currentVersion, c.currentExists, c.eventVersion)
			s.Equal(c.want, got)
		})
	}
}
