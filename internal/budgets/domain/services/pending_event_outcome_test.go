package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type PendingEventOutcomeSuite struct {
	suite.Suite
}

func TestPendingEventOutcomeSuite(t *testing.T) {
	suite.Run(t, new(PendingEventOutcomeSuite))
}

func buildEvent(s *PendingEventOutcomeSuite, kind valueobjects.MutationKind, version int64) entities.PendingEvent {
	src, err := valueobjects.NewProducerSource("inbox-email")
	s.Require().NoError(err)
	extID, err := valueobjects.NewExternalTransactionID(uuid.New().String())
	s.Require().NoError(err)
	return entities.NewPendingEvent(uuid.New(), src, uuid.New(), extID, version, kind, []byte(`{}`), time.Now().UTC())
}

func buildExpense(s *PendingEventOutcomeSuite, version int64) *entities.Expense {
	src, err := valueobjects.NewProducerSource("inbox-email")
	s.Require().NoError(err)
	extID, err := valueobjects.NewExternalTransactionID(uuid.New().String())
	s.Require().NoError(err)
	comp, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)
	e := entities.HydrateExpense(uuid.New(), uuid.New(), src, extID, uuid.New(), valueobjects.RootSlugCustoFixo, comp, 100, time.Now().UTC(), version, nil, nil, time.Now().UTC(), time.Now().UTC())
	return &e
}

func (s *PendingEventOutcomeSuite) TestDecide() {
	type tc struct {
		name           string
		kind           valueobjects.MutationKind
		version        int64
		current        func() *entities.Expense
		wantKind       services.PendingEventOutcomeKind
		wantExpVersion int64
	}

	cases := []tc{
		{
			name:     "create sem despesa existente — Create",
			kind:     valueobjects.MutationKindCreate,
			version:  1,
			current:  func() *entities.Expense { return nil },
			wantKind: services.OutcomeCreate,
		},
		{
			name:     "create com despesa existente — Noop",
			kind:     valueobjects.MutationKindCreate,
			version:  1,
			current:  func() *entities.Expense { return buildExpense(s, 1) },
			wantKind: services.OutcomeNoop,
		},
		{
			name:     "create com version != 1 — Noop",
			kind:     valueobjects.MutationKindCreate,
			version:  2,
			current:  func() *entities.Expense { return nil },
			wantKind: services.OutcomeNoop,
		},
		{
			name:     "update sem despesa atual — Defer",
			kind:     valueobjects.MutationKindUpdate,
			version:  2,
			current:  func() *entities.Expense { return nil },
			wantKind: services.OutcomeDefer,
		},
		{
			name:     "update com version <= atual — Noop",
			kind:     valueobjects.MutationKindUpdate,
			version:  2,
			current:  func() *entities.Expense { return buildExpense(s, 2) },
			wantKind: services.OutcomeNoop,
		},
		{
			name:     "update com version > atual+1 — Defer",
			kind:     valueobjects.MutationKindUpdate,
			version:  5,
			current:  func() *entities.Expense { return buildExpense(s, 2) },
			wantKind: services.OutcomeDefer,
		},
		{
			name:           "update consecutivo — Update",
			kind:           valueobjects.MutationKindUpdate,
			version:        3,
			current:        func() *entities.Expense { return buildExpense(s, 2) },
			wantKind:       services.OutcomeUpdate,
			wantExpVersion: 2,
		},
		{
			name:           "delete consecutivo — Delete",
			kind:           valueobjects.MutationKindDelete,
			version:        3,
			current:        func() *entities.Expense { return buildExpense(s, 2) },
			wantKind:       services.OutcomeDelete,
			wantExpVersion: 2,
		},
		{
			name:     "delete sem despesa — Defer",
			kind:     valueobjects.MutationKindDelete,
			version:  2,
			current:  func() *entities.Expense { return nil },
			wantKind: services.OutcomeDefer,
		},
	}

	resolver := services.NewPendingEventOutcomeResolver()
	for _, c := range cases {
		s.Run(c.name, func() {
			ev := buildEvent(s, c.kind, c.version)
			got := resolver.Decide(ev, c.current())
			s.Equal(c.wantKind, got.Kind)
			s.Equal(c.wantExpVersion, got.ExpectedVersion)
		})
	}
}
