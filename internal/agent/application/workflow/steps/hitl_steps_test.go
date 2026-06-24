package steps

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type HITLStepsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestHITLStepsSuite(t *testing.T) {
	suite.Run(t, new(HITLStepsSuite))
}

func (s *HITLStepsSuite) SetupTest() {
	s.ctx = context.Background()
}

func baseConfirmState() confirmation.ConfirmState {
	return confirmation.ConfirmState{
		OperationKind:    confirmation.OperationDeleteLast,
		AwaitingApproval: confirmation.AwaitingNone,
		UserID:           "user-1",
		Channel:          "whatsapp",
		MessageID:        "msg-1",
		PromptText:       "Deseja realmente apagar o último lançamento?",
	}
}

func (s *HITLStepsSuite) TestPrepareTarget_KindNotMapped() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve curto-circuitar quando kind nao mapeado",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.OperationKind = confirmation.OperationBudgetCommit
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(int(tools.OutcomeMissingResolver), out.State.Outcome)
				s.NotEmpty(out.State.Reply)
			},
		},
		{
			name: "deve ignorar quando ja ShortCircuit",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.ShortCircuit = true
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewPrepareTarget(PrepareTargetDeps{
				Targets: map[confirmation.OperationKind]TargetResolver{
					confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						return st, nil
					},
				},
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestPrepareTarget_KindMapped() {
	type dependencies struct {
		resolver TargetResolver
	}
	scenarios := []struct {
		name         string
		state        confirmation.ConfirmState
		dependencies dependencies
		expect       func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name:  "deve chamar resolver quando kind mapeado",
			state: baseConfirmState(),
			dependencies: dependencies{
				resolver: func() TargetResolver {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						st.PromptText = "Deseja mesmo apagar?"
						return st, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.False(out.State.ShortCircuit)
				s.Equal("Deseja mesmo apagar?", out.State.PromptText)
			},
		},
		{
			name:  "deve retornar falha quando resolver falha",
			state: baseConfirmState(),
			dependencies: dependencies{
				resolver: func() TargetResolver {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						return st, errors.New("nao encontrado")
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.Error(err)
				s.Equal(platform.StepStatusFailed, out.Status)
			},
		},
		{
			name:  "deve curto-circuitar quando resolver sinaliza short-circuit",
			state: baseConfirmState(),
			dependencies: dependencies{
				resolver: func() TargetResolver {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						st.ShortCircuit = true
						st.Reply = "Nenhum lançamento encontrado."
						return st, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal("Nenhum lançamento encontrado.", out.State.Reply)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewPrepareTarget(PrepareTargetDeps{
				Targets: map[confirmation.OperationKind]TargetResolver{
					confirmation.OperationDeleteLast: scenario.dependencies.resolver,
				},
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_FirstPass() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name:  "deve suspender na primeira passada e setar AwaitingConfirm",
			state: baseConfirmState(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(platform.SuspendAwaitingInput, out.Suspend.Reason)
				s.Equal(confirmation.AwaitingConfirm, out.State.AwaitingApproval)
				s.False(out.State.SuspendedAt.IsZero())
			},
		},
		{
			name: "deve ignorar quando ja ShortCircuit",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.ShortCircuit = true
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Nil(out.Suspend)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_Confirms() {
	confirmTexts := []string{"sim", "s", "confirma", "confirmado", "pode", "ok", "yes"}
	for _, text := range confirmTexts {
		s.Run("deve prosseguir com confirmacao: "+text, func() {
			state := baseConfirmState()
			state.AwaitingApproval = confirmation.AwaitingConfirm
			state.SuspendedAt = time.Now().UTC()
			state.ResumeText = text

			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, state)

			s.NoError(err)
			s.Equal(platform.StepStatusCompleted, out.Status)
			s.False(out.State.ShortCircuit)
			s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_Cancels() {
	cancelTexts := []string{"não", "nao", "n", "no", "cancela", "cancelar"}
	for _, text := range cancelTexts {
		s.Run("deve cancelar com: "+text, func() {
			state := baseConfirmState()
			state.AwaitingApproval = confirmation.AwaitingConfirm
			state.SuspendedAt = time.Now().UTC()
			state.ResumeText = text

			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, state)

			s.NoError(err)
			s.Equal(platform.StepStatusCompleted, out.Status)
			s.True(out.State.ShortCircuit)
			s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
			s.Equal(confirmCancelledText, out.State.Reply)
			s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_AmbiguousFirstTime() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve re-prompt na primeira ambiguidade (RepromptCount=0)",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.AwaitingApproval = confirmation.AwaitingConfirm
				st.SuspendedAt = time.Now().UTC()
				st.RepromptCount = 0
				st.ResumeText = "talvez"
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusSuspended, out.Status)
				s.NotNil(out.Suspend)
				s.Equal(1, out.State.RepromptCount)
				s.False(out.State.ShortCircuit)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_AmbiguousSecondTime() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve cancelar na segunda ambiguidade (RepromptCount>=1)",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.AwaitingApproval = confirmation.AwaitingConfirm
				st.SuspendedAt = time.Now().UTC()
				st.RepromptCount = 1
				st.ResumeText = "talvez"
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
				s.Equal(confirmCancelledAmbiguousText, out.State.Reply)
				s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestConfirmGate_Expired() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve cancelar quando TTL expirado",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.AwaitingApproval = confirmation.AwaitingConfirm
				st.SuspendedAt = time.Now().UTC().Add(-20 * time.Minute)
				st.ResumeText = "sim"
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
				s.Equal(confirmExpiredText, out.State.Reply)
				s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewConfirmGate(10 * time.Minute)
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestExecuteDestructive_ShortCircuit() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		called bool
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve retornar sem chamar executor quando ShortCircuit=true",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.ShortCircuit = true
				st.Reply = "cancelado"
				return st
			}(),
			called: false,
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal("cancelado", out.State.Reply)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			executorCalled := false
			step := NewExecuteDestructive(ExecuteDestructiveDeps{
				Executors: map[confirmation.OperationKind]DestructiveExecutor{
					confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						executorCalled = true
						return st, nil
					},
				},
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
			s.Equal(scenario.called, executorCalled)
		})
	}
}

func (s *HITLStepsSuite) TestExecuteDestructive_KindNotMapped() {
	scenarios := []struct {
		name   string
		state  confirmation.ConfirmState
		expect func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name: "deve curto-circuitar quando kind nao mapeado",
			state: func() confirmation.ConfirmState {
				st := baseConfirmState()
				st.OperationKind = confirmation.OperationBudgetCommit
				return st
			}(),
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.True(out.State.ShortCircuit)
				s.Equal(int(tools.OutcomeMissingResolver), out.State.Outcome)
				s.NotEmpty(out.State.Reply)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewExecuteDestructive(ExecuteDestructiveDeps{
				Executors: map[confirmation.OperationKind]DestructiveExecutor{
					confirmation.OperationDeleteLast: func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						return st, nil
					},
				},
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}

func (s *HITLStepsSuite) TestExecuteDestructive_KindMapped() {
	type dependencies struct {
		executor DestructiveExecutor
	}
	scenarios := []struct {
		name         string
		state        confirmation.ConfirmState
		dependencies dependencies
		expect       func(out platform.StepOutput[confirmation.ConfirmState], err error)
	}{
		{
			name:  "deve executar quando kind mapeado com sucesso",
			state: baseConfirmState(),
			dependencies: dependencies{
				executor: func() DestructiveExecutor {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						st.Reply = "Lançamento apagado com sucesso."
						st.Outcome = int(tools.OutcomeRouted)
						return st, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.False(out.State.ShortCircuit)
				s.Equal("Lançamento apagado com sucesso.", out.State.Reply)
				s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
			},
		},
		{
			name:  "deve retornar falha quando executor retorna erro",
			state: baseConfirmState(),
			dependencies: dependencies{
				executor: func() DestructiveExecutor {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						return st, errors.New("falha no banco")
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.Error(err)
				s.Equal(platform.StepStatusFailed, out.Status)
			},
		},
		{
			name:  "deve setar OutcomeRouted quando executor nao seta outcome",
			state: baseConfirmState(),
			dependencies: dependencies{
				executor: func() DestructiveExecutor {
					return func(_ context.Context, st confirmation.ConfirmState) (confirmation.ConfirmState, error) {
						return st, nil
					}
				}(),
			},
			expect: func(out platform.StepOutput[confirmation.ConfirmState], err error) {
				s.NoError(err)
				s.Equal(platform.StepStatusCompleted, out.Status)
				s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
			},
		},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			step := NewExecuteDestructive(ExecuteDestructiveDeps{
				Executors: map[confirmation.OperationKind]DestructiveExecutor{
					confirmation.OperationDeleteLast: scenario.dependencies.executor,
				},
			})
			out, err := step.Execute(s.ctx, scenario.state)
			scenario.expect(out, err)
		})
	}
}
