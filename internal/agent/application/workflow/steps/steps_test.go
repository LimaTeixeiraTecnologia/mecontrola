package steps

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type StepsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestStepsSuite(t *testing.T) {
	suite.Run(t, new(StepsSuite))
}

func (s *StepsSuite) SetupTest() {
	s.ctx = context.Background()
}

func baseState() ExpenseState {
	return ExpenseState{
		Kind:            intent.KindRecordExpense,
		TransactionKind: pendingexpense.TransactionKindExpense,
		AmountCents:     1000,
		Merchant:        "Loja",
		PaymentMethod:   "debit",
		Direction:       "outcome",
	}
}

func (s *StepsSuite) TestAuthorize_Permitted() {
	type dependencies struct {
		authFn AuthorizeFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve permitir quando authorized retorna true",
			state: baseState(),
			dependencies: dependencies{
				authFn: func() AuthorizeFunc {
					return func(_ context.Context, _ ExpenseState) bool { return true }
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.False(out.State.ShortCircuit)
			},
		},
		{
			name:  "deve bloquear quando authorized retorna false",
			state: baseState(),
			dependencies: dependencies{
				authFn: func() AuthorizeFunc {
					return func(_ context.Context, _ ExpenseState) bool { return false }
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeAuthzDenied, out.State.Outcome)
				s.Equal("negado", out.State.Reply)
			},
		},
		{
			name:  "deve ignorar quando ja ShortCircuit",
			state: func() ExpenseState { st := baseState(); st.ShortCircuit = true; return st }(),
			dependencies: dependencies{
				authFn: func() AuthorizeFunc {
					return func(_ context.Context, _ ExpenseState) bool {
						return false
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.ToolOutcome(0), out.State.Outcome)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewAuthorize(scenario.dependencies.authFn, "negado")
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestReplay_Found() {
	type dependencies struct {
		replayFn ReplayFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve aplicar replay quando encontrado",
			state: baseState(),
			dependencies: dependencies{
				replayFn: func() ReplayFunc {
					return func(_ context.Context, _ ExpenseState) (string, bool) {
						return "resposta anterior", true
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeReplay, out.State.Outcome)
				s.Equal("resposta anterior", out.State.Reply)
			},
		},
		{
			name:  "deve prosseguir quando nao encontrado",
			state: baseState(),
			dependencies: dependencies{
				replayFn: func() ReplayFunc {
					return func(_ context.Context, _ ExpenseState) (string, bool) { return "", false }
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.False(out.State.ShortCircuit)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewReplay(scenario.dependencies.replayFn)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestPolicy_Blocked() {
	type dependencies struct {
		policyFn PolicyFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve bloquear quando policy retorna blocked",
			state: baseState(),
			dependencies: dependencies{
				policyFn: func() PolicyFunc {
					return func(_ context.Context, _ ExpenseState) (bool, string) {
						return true, "confiança baixa"
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomePolicyBlocked, out.State.Outcome)
				s.Equal("confiança baixa", out.State.Reply)
			},
		},
		{
			name:  "deve prosseguir quando policy permite",
			state: baseState(),
			dependencies: dependencies{
				policyFn: func() PolicyFunc {
					return func(_ context.Context, _ ExpenseState) (bool, string) { return false, "" }
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.False(out.State.ShortCircuit)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewPolicy(scenario.dependencies.policyFn)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestAuditBegin() {
	type dependencies struct {
		beginFn AuditBeginFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve curto-circuitar em conflito",
			state: baseState(),
			dependencies: dependencies{
				beginFn: func() AuditBeginFunc {
					return func(_ context.Context, _ ExpenseState) AuditBeginResult {
						return AuditBeginResult{Conflicted: true}
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeReplay, out.State.Outcome)
			},
		},
		{
			name:  "deve curto-circuitar em falha",
			state: baseState(),
			dependencies: dependencies{
				beginFn: func() AuditBeginFunc {
					return func(_ context.Context, _ ExpenseState) AuditBeginResult {
						return AuditBeginResult{Failed: true}
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeUsecaseError, out.State.Outcome)
			},
		},
		{
			name:  "deve prosseguir e registrar settle",
			state: baseState(),
			dependencies: dependencies{
				beginFn: func() AuditBeginFunc {
					return func(_ context.Context, _ ExpenseState) AuditBeginResult {
						return AuditBeginResult{
							Settle: func(_ context.Context, _ bool) {},
						}
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.False(out.State.ShortCircuit)
				s.Equal(platform.StepStatusCompleted, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewAuditBegin(scenario.dependencies.beginFn, nil, "replay", "falha")
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestResolveCategory_Auto() {
	type dependencies struct {
		resolverFn CategoryResolverFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve resolver categoria automaticamente",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						st.CategoryID = "cat-1"
						st.CategoryPath = "Alimentação"
						return st, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal("Alimentação", out.State.CategoryPath)
			},
		},
		{
			name:  "deve suspender quando categoria ambigua",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						return st, &tools.CategoryAmbiguousError{Hint: "test", Candidates: []string{"A", "B"}}
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(pendingexpense.AwaitingCategoryChoice, out.State.AwaitingKind)
				s.Equal(tools.OutcomeClarify, out.State.Outcome)
			},
		},
		{
			name:  "deve suspender quando categoria precisa confirmacao",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						return st, &tools.CategoryNeedsConfirmationError{Hint: "test", Candidates: []string{"Alimentação"}}
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.Equal(pendingexpense.AwaitingCategoryConfirm, out.State.AwaitingKind)
				s.Equal(tools.OutcomeClarify, out.State.Outcome)
			},
		},
		{
			name:  "deve curto-circuitar quando categoria nao encontrada",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						return st, errors.Join(tools.ErrCategoryNotFound, errors.New("not found"))
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeClarify, out.State.Outcome)
			},
		},
		{
			name:  "deve curto-circuitar quando sem hint",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						return st, errors.Join(tools.ErrCategoryHintMissing, errors.New("no hint"))
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeClarify, out.State.Outcome)
			},
		},
		{
			name:  "deve retornar falha em erro desconhecido",
			state: baseState(),
			dependencies: dependencies{
				resolverFn: func() CategoryResolverFunc {
					return func(_ context.Context, st ExpenseState) (ExpenseState, error) {
						return st, errors.New("falha de banco")
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.Error(err)
				s.Equal(platform.StepStatusFailed, out.Status)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewResolveCategory(scenario.dependencies.resolverFn)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestResolveCategory_Resume() {
	suspendedState := func(awaitingKind pendingexpense.AwaitingKind, candidates []string) ExpenseState {
		st := baseState()
		st.AwaitingKind = awaitingKind
		st.Candidates = candidates
		st.CategoryID = candidates[0]
		st.Reply = "escolha"
		return st
	}

	noop := func(_ context.Context, st ExpenseState) (ExpenseState, error) { return st, nil }

	type dependencies struct {
		state ExpenseState
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name: "deve retomar choice com escolha numerica",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryChoice, []string{"Alimentação", "Transporte"})
					st.ResumeText = "1"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal("Alimentação", out.State.CategoryID)
				s.Equal(pendingexpense.AwaitingKind(""), out.State.AwaitingKind)
				s.NotNil(out.State.ForceCategory)
			},
		},
		{
			name: "deve retomar confirm com sim",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryConfirm, []string{"Alimentação"})
					st.ResumeText = "sim"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal(pendingexpense.AwaitingKind(""), out.State.AwaitingKind)
				s.NotNil(out.State.ForceCategory)
			},
		},
		{
			name: "deve cancelar com nao",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryChoice, []string{"Alimentação"})
					st.ResumeText = "não"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeRouted, out.State.Outcome)
			},
		},
		{
			name: "deve cancelar confirm com nao",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryConfirm, []string{"Alimentação"})
					st.ResumeText = "nao"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(tools.OutcomeRouted, out.State.Outcome)
			},
		},
		{
			name: "deve resuspender choice com texto nao reconhecido",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryChoice, []string{"Alimentação", "Transporte"})
					st.ResumeText = "qualquer coisa"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(pendingexpense.AwaitingCategoryChoice, out.State.AwaitingKind)
			},
		},
		{
			name: "deve resuspender confirm com texto nao reconhecido",
			dependencies: dependencies{
				state: func() ExpenseState {
					st := suspendedState(pendingexpense.AwaitingCategoryConfirm, []string{"Alimentação"})
					st.ResumeText = "talvez"
					return st
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(pendingexpense.AwaitingCategoryConfirm, out.State.AwaitingKind)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewResolveCategory(noop)
			out, err := step.Execute(s.ctx, scenario.dependencies.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestPersist() {
	type dependencies struct {
		persistFn PersistFunc
	}
	scenarios := []struct {
		name         string
		state        ExpenseState
		dependencies dependencies
		expect       func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name:  "deve persistir com sucesso",
			state: baseState(),
			dependencies: dependencies{
				persistFn: func() PersistFunc {
					return func(_ context.Context, _ ExpenseState) (PersistResult, error) {
						return PersistResult{AmountCents: 1000, CategoryPath: "Alimentação"}, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal(tools.OutcomeRouted, out.State.Outcome)
				s.Equal("Alimentação", out.State.CategoryPath)
			},
		},
		{
			name:  "deve retornar falha em erro de persistencia",
			state: baseState(),
			dependencies: dependencies{
				persistFn: func() PersistFunc {
					return func(_ context.Context, _ ExpenseState) (PersistResult, error) {
						return PersistResult{}, errors.New("db error")
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.Error(err)
				s.Equal(platform.StepStatusFailed, out.Status)
			},
		},
		{
			name: "deve ignorar quando ja ShortCircuit",
			state: func() ExpenseState {
				st := baseState()
				st.ShortCircuit = true
				st.Outcome = tools.OutcomeAuthzDenied
				return st
			}(),
			dependencies: dependencies{
				persistFn: func() PersistFunc {
					return func(_ context.Context, _ ExpenseState) (PersistResult, error) {
						return PersistResult{}, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal(tools.OutcomeAuthzDenied, out.State.Outcome)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewPersist(scenario.dependencies.persistFn)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestFormat() {
	scenarios := []struct {
		name   string
		state  ExpenseState
		expect func(out platform.StepOutput[ExpenseState], err error)
	}{
		{
			name: "deve formatar quando outcome routed",
			state: func() ExpenseState {
				st := baseState()
				st.Outcome = tools.OutcomeRouted
				st.AmountCents = 5000
				st.Merchant = "Supermercado"
				st.CategoryPath = "Alimentação"
				return st
			}(),
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.NotEmpty(out.State.Reply)
			},
		},
		{
			name: "deve ignorar quando nao routed",
			state: func() ExpenseState {
				st := baseState()
				st.Outcome = tools.OutcomeClarify
				st.Reply = "escolha categoria"
				return st
			}(),
			expect: func(out platform.StepOutput[ExpenseState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal("escolha categoria", out.State.Reply)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewFormat(func(st ExpenseState) string {
				return tools.FormatPersistedExpense(st.AmountCents, st.Merchant, st.CategoryPath)
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *StepsSuite) TestExpenseState_ToDraft() {
	state := ExpenseState{
		AmountCents:     1500,
		Merchant:        "Loja",
		PaymentMethod:   "credit",
		Direction:       "outcome",
		OccurredAt:      "2024-01-01",
		CategoryID:      "cat-1",
		CategoryPath:    "Alimentação",
		Candidates:      []string{"A", "B"},
		AwaitingKind:    pendingexpense.AwaitingCategoryChoice,
		TransactionKind: pendingexpense.TransactionKindExpense,
		Installments:    1,
		CardHint:        "",
	}
	draft := state.ToDraft()
	s.Equal(state.AmountCents, draft.AmountCents)
	s.Equal(state.Merchant, draft.Merchant)
	s.Equal(state.PaymentMethod, draft.PaymentMethod)
	s.Equal(state.Direction, draft.Direction)
	s.Equal(state.OccurredAt, draft.OccurredAt)
	s.Equal(state.CategoryID, draft.CategoryID)
	s.Equal(state.CategoryPath, draft.CategoryPath)
	s.Equal(state.Candidates, draft.Candidates)
	s.Equal(state.AwaitingKind, draft.AwaitingKind)
	s.Equal(state.TransactionKind, draft.TransactionKind)
	s.Equal(state.Installments, draft.Installments)
	s.Equal(state.CardHint, draft.CardHint)
}

func (s *StepsSuite) TestStateFromDraft_RoundTrip() {
	origDraft := pendingexpense.Draft{
		AmountCents:     2000,
		Merchant:        "Mercado",
		PaymentMethod:   "debit",
		Direction:       "outcome",
		OccurredAt:      "2024-06-01",
		CategoryID:      "cat-2",
		CategoryPath:    "Supermercado",
		Candidates:      []string{"X", "Y"},
		AwaitingKind:    pendingexpense.AwaitingCategoryConfirm,
		TransactionKind: pendingexpense.TransactionKindExpense,
		Installments:    1,
		CardHint:        "",
	}
	st := StateFromDraft(origDraft, [16]byte{}, "chan", "msg-1", [16]byte{}, "texto de retomada")
	draft := st.ToDraft()
	s.Equal(origDraft.AmountCents, draft.AmountCents)
	s.Equal(origDraft.Merchant, draft.Merchant)
	s.Equal(origDraft.AwaitingKind, draft.AwaitingKind)
	s.Equal(origDraft.Candidates, draft.Candidates)
	s.Equal("texto de retomada", st.ResumeText)
}
