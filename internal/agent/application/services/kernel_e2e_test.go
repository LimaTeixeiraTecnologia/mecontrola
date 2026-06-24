package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type e2eStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newE2EStore() *e2eStore {
	return &e2eStore{runs: make(map[string]platform.Snapshot)}
}

func (s *e2eStore) key(wf, key string) string { return wf + ":" + key }

func (s *e2eStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *e2eStore) Load(_ context.Context, wf, key string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(wf, key)]
	return snap, ok, nil
}

func (s *e2eStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *e2eStore) AppendStep(_ context.Context, _ platform.StepRecord) error { return nil }
func (s *e2eStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

type fakeNoPendingExpenseGateway struct{}

func (f *fakeNoPendingExpenseGateway) Load(_ context.Context, _ uuid.UUID, _ string) (pendingexpense.Draft, bool, error) {
	return pendingexpense.Draft{}, false, nil
}
func (f *fakeNoPendingExpenseGateway) Save(_ context.Context, _ uuid.UUID, _ string, _ pendingexpense.Draft) error {
	return nil
}
func (f *fakeNoPendingExpenseGateway) Clear(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

type KernelE2ESuite struct {
	suite.Suite
	ctx context.Context
	wa  *fakeWhatsAppGateway
}

func TestKernelE2ESuite(t *testing.T) {
	suite.Run(t, new(KernelE2ESuite))
}

func (s *KernelE2ESuite) SetupTest() {
	s.ctx = context.Background()
	s.wa = &fakeWhatsAppGateway{}
}

func (s *KernelE2ESuite) buildRouter(
	resolver steps.CategoryResolverFunc,
	persistFn steps.PersistFunc,
) *services.IntentRouter {
	obs := fake.NewProvider()
	store := newE2EStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	settleReg := services.NewSettleRegistry()

	deps := services.IntentRouterDeps{
		Parser:                     &fakeParser{intent: mustBuildExpenseIntent2(5800, "iFood", "Prazeres")},
		Fallback:                   &fakeFallback{reply: "fallback"},
		WhatsAppGateway:            s.wa,
		Location:                   time.UTC,
		PendingExpenseConfirmation: &fakeNoPendingExpenseGateway{},
		Kernel: &services.KernelDeps{
			Engine:           engine,
			SettleReg:        settleReg,
			CategoryResolver: resolver,
			PersistFn:        persistFn,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)
	return router
}

func buildE2ETransactionsWriteDef(resolver steps.CategoryResolverFunc, persist steps.PersistFunc) platform.Definition[steps.ExpenseState] {
	return agentwf.NewTransactionsWriteDefinition(agentwf.TransactionsWriteDeps{
		Authorize: func(_ context.Context, _ steps.ExpenseState) bool { return true },
		Replay:    func(_ context.Context, _ steps.ExpenseState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
			return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
		},
		OnSettle:       nil,
		Resolver:       resolver,
		Persist:        persist,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})
}

func mustBuildExpenseIntent2(amountCents int64, merchant, hint string) intent.Intent {
	in, err := intent.NewRecordExpense(intent.RecordExpenseFields{
		AmountCents:  amountCents,
		Merchant:     merchant,
		CategoryHint: hint,
	})
	if err != nil {
		panic(err)
	}
	return in
}

func (s *KernelE2ESuite) TestE2E_KernelFlagOn_AutoLog_ReplyIdenticalToLegacy() {
	type dependencies struct {
		resolver steps.CategoryResolverFunc
		persist  steps.PersistFunc
	}
	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(result services.RouteResult)
	}{
		{
			name: "deve roteiar com sucesso (auto-log, flag on)",
			dependencies: dependencies{
				resolver: func() steps.CategoryResolverFunc {
					return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
						st.CategoryID = "prazeres-delivery"
						st.CategoryPath = "Prazeres > Delivery"
						return st, nil
					}
				}(),
				persist: func() steps.PersistFunc {
					return func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
						return steps.PersistResult{AmountCents: 5800, CategoryPath: "Prazeres > Delivery"}, nil
					}
				}(),
			},
			expect: func(result services.RouteResult) {
				s.Equal(intent.KindRecordExpense, result.Kind)
				s.Equal(tools.OutcomeRouted, result.Outcome)
				s.Require().Len(s.wa.sent, 1)
				s.Contains(s.wa.sent[0].Text, "R$ 58,00")
				s.Contains(s.wa.sent[0].Text, "iFood")
			},
		},
		{
			name: "deve retornar usecase error quando persist falha (flag on)",
			dependencies: dependencies{
				resolver: func() steps.CategoryResolverFunc {
					return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
						st.CategoryID = "prazeres"
						st.CategoryPath = "Prazeres"
						return st, nil
					}
				}(),
				persist: func() steps.PersistFunc {
					return func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
						return steps.PersistResult{}, errors.New("db unavailable")
					}
				}(),
			},
			expect: func(result services.RouteResult) {
				s.Equal(intent.KindRecordExpense, result.Kind)
				s.Equal(tools.OutcomeUsecaseError, result.Outcome)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.wa = &fakeWhatsAppGateway{}
			router := s.buildRouter(scenario.dependencies.resolver, scenario.dependencies.persist)
			principal := services.Principal{UserID: uuid.New()}
			result := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{
				Text:       "gastei 58 no iFood",
				WhatsAppTo: "+5511999",
				MessageID:  "msg-e2e-1",
			})
			scenario.expect(result)
		})
	}
}

func (s *KernelE2ESuite) TestE2E_KernelFlagOn_AuditFieldsPropagateToAuditBegin() {
	var captured steps.ExpenseState
	resolver := func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
		st.CategoryID = "prazeres-delivery"
		st.CategoryPath = "Prazeres > Delivery"
		return st, nil
	}
	persist := func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
		return steps.PersistResult{AmountCents: 5800, CategoryPath: "Prazeres > Delivery"}, nil
	}

	obs := fake.NewProvider()
	store := newE2EStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	settleReg := services.NewSettleRegistry()

	parser := &fakeParser{
		intent:       mustBuildExpenseIntent2(5800, "iFood", "Prazeres"),
		llmModel:     "google/gemini-test",
		promptSHA256: "sha256-audit",
		rawResponse:  []byte(`{"kind":"record_expense"}`),
	}
	deps := services.IntentRouterDeps{
		Parser:                     parser,
		Fallback:                   &fakeFallback{reply: "fallback"},
		WhatsAppGateway:            s.wa,
		Location:                   time.UTC,
		PendingExpenseConfirmation: &fakeNoPendingExpenseGateway{},
		Kernel: &services.KernelDeps{
			Engine:           engine,
			SettleReg:        settleReg,
			CategoryResolver: resolver,
			PersistFn:        persist,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	customDef := agentwf.NewTransactionsWriteDefinition(agentwf.TransactionsWriteDeps{
		Authorize: func(_ context.Context, _ steps.ExpenseState) bool { return true },
		Replay:    func(_ context.Context, _ steps.ExpenseState) (string, bool) { return "", false },
		Policy:    func(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" },
		AuditBegin: func(_ context.Context, st steps.ExpenseState) steps.AuditBeginResult {
			captured = st
			return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
		},
		OnSettle:       nil,
		Resolver:       resolver,
		Persist:        persist,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha",
	})
	router.EnableKernel(engine, customDef, settleReg)

	principal := services.Principal{UserID: uuid.New()}
	result := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{
		Text:       "gastei 58 no iFood",
		WhatsAppTo: "+5511999",
		MessageID:  "msg-audit-1",
	})

	s.Equal(tools.OutcomeRouted, result.Outcome)
	s.Equal("google/gemini-test", captured.LLMModel)
	s.Equal("sha256-audit", captured.PromptSHA256)
	s.Equal(`{"kind":"record_expense"}`, captured.RawResponse)
}

func (s *KernelE2ESuite) TestE2E_KernelFlagOn_AmbiguousChoiceCycle() { //nolint:revive // test: scenario requires >40 statements to cover full suspend→resume cycle end-to-end
	candidates := []string{"Prazeres > Academia", "Custo Fixo > Academia"}

	persistCalls := 0
	resolver := func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
		if st.ForceCategory == nil && st.AwaitingKind == "" {
			return st, &tools.CategoryAmbiguousError{Hint: "academia", Candidates: candidates}
		}
		cat := candidates[0]
		if st.ForceCategory != nil {
			cat = *st.ForceCategory
		}
		st.CategoryID = cat
		st.CategoryPath = cat
		return st, nil
	}
	persist := func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
		persistCalls++
		return steps.PersistResult{AmountCents: 5800, CategoryPath: candidates[0]}, nil
	}

	obs := fake.NewProvider()
	store := newE2EStore()
	engine := platform.NewEngine[steps.ExpenseState](store, obs)
	settleReg := services.NewSettleRegistry()

	userID := uuid.New()
	channel := "whatsapp"

	parser := &fakeParser{intent: mustBuildExpenseIntent2(5800, "academia", "academia")}
	deps := services.IntentRouterDeps{
		Parser:                     parser,
		Fallback:                   &fakeFallback{reply: "fallback"},
		WhatsAppGateway:            s.wa,
		Location:                   time.UTC,
		PendingExpenseConfirmation: &fakeNoPendingExpenseGateway{},
		Kernel: &services.KernelDeps{
			Engine:           engine,
			SettleReg:        settleReg,
			CategoryResolver: resolver,
			PersistFn:        persist,
		},
	}
	router, err := services.NewIntentRouter(obs, deps)
	s.Require().NoError(err)

	principal := services.Principal{UserID: userID}

	result1 := router.RouteWhatsApp(s.ctx, principal, services.InboundMessage{
		Text:       "gastei 58 na academia",
		WhatsAppTo: "+" + channel,
		MessageID:  "msg-ambig-1",
	})
	s.Equal(tools.OutcomeClarify, result1.Outcome, "initial: deve clarificar")
	s.Require().Len(s.wa.sent, 1)
	s.Contains(s.wa.sent[0].Text, "Prazeres > Academia")
	s.Equal(0, persistCalls, "persist nao deve ser chamado no suspend")

	snap, found, loadErr := store.Load(s.ctx, "transactions_write", userID.String()+":"+channel)
	s.Require().NoError(loadErr)
	s.Require().True(found, "snapshot deve existir apos suspend")
	s.Equal(platform.RunStatusSuspended, snap.Status)

	storedState, decErr := platform.NewCodec[steps.ExpenseState]().Decode(snap.State)
	s.Require().NoError(decErr)
	s.Equal(pendingexpense.AwaitingCategoryChoice, storedState.AwaitingKind)
	s.Equal(candidates, storedState.Candidates)

	storedState.ResumeText = "1"
	resumeBytes, encErr := platform.NewCodec[steps.ExpenseState]().Encode(storedState)
	s.Require().NoError(encErr)

	def := buildE2ETransactionsWriteDef(resolver, persist)

	resumeResult, resumeErr := engine.Resume(s.ctx, def, userID.String()+":"+channel, resumeBytes)
	s.Require().NoError(resumeErr)
	s.Equal(platform.RunStatusSucceeded, resumeResult.Status, "resume: esperado succeeded")
	s.Equal(tools.OutcomeRouted, resumeResult.State.Outcome, "resume: esperado routed")
	s.Equal(1, persistCalls, "persist deve ser chamado exatamente uma vez")
}
