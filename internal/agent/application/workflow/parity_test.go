package workflow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type parityStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newParityStore() *parityStore {
	return &parityStore{runs: make(map[string]platform.Snapshot)}
}

func (s *parityStore) key(workflow, correlationKey string) string {
	return workflow + ":" + correlationKey
}

func (s *parityStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *parityStore) Load(_ context.Context, workflow, key string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(workflow, key)]
	return snap, ok, nil
}

func (s *parityStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *parityStore) AppendStep(_ context.Context, _ platform.StepRecord) error {
	return nil
}

func (s *parityStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

type parityResult struct {
	Outcome tools.ToolOutcome
	Kind    intent.Kind
}

type parityBehaviors struct {
	authorize      func(ctx context.Context, state steps.ExpenseState) bool
	replay         func(ctx context.Context, state steps.ExpenseState) (string, bool)
	policy         func(ctx context.Context, state steps.ExpenseState) (bool, string)
	auditBegin     func(ctx context.Context, state steps.ExpenseState) steps.AuditBeginResult
	resolver       func(ctx context.Context, state steps.ExpenseState) (steps.ExpenseState, error)
	persist        func(ctx context.Context, state steps.ExpenseState) (steps.PersistResult, error)
	resumeText     string
	initialState   steps.ExpenseState
	correlationKey string
}

type ParitySuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestParitySuite(t *testing.T) {
	suite.Run(t, new(ParitySuite))
}

func (s *ParitySuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *ParitySuite) newKernelEngine() platform.Engine[steps.ExpenseState] {
	return platform.NewEngine[steps.ExpenseState](newParityStore(), s.obs)
}

func (s *ParitySuite) runKernel(b parityBehaviors) parityResult {
	eng := s.newKernelEngine()
	def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
		Authorize:      b.authorize,
		Replay:         b.replay,
		Policy:         b.policy,
		AuditBegin:     b.auditBegin,
		OnSettle:       nil,
		Resolver:       b.resolver,
		Persist:        b.persist,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha auditoria",
	})

	key := b.correlationKey
	if key == "" {
		key = b.initialState.UserID.String() + ":" + b.initialState.Channel
	}
	result, err := eng.Start(s.ctx, def, key, b.initialState)
	if err != nil {
		return parityResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	return parityResult{Outcome: result.State.Outcome, Kind: result.State.Kind}
}

func (s *ParitySuite) runKernelResume(b parityBehaviors) parityResult {
	store := newParityStore()
	eng := platform.NewEngine[steps.ExpenseState](store, s.obs)
	def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
		Authorize:      b.authorize,
		Replay:         b.replay,
		Policy:         b.policy,
		AuditBegin:     b.auditBegin,
		OnSettle:       nil,
		Resolver:       b.resolver,
		Persist:        b.persist,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha auditoria",
	})

	key := b.correlationKey
	if key == "" {
		key = b.initialState.UserID.String() + ":" + b.initialState.Channel
	}

	startResult, err := eng.Start(s.ctx, def, key, b.initialState)
	if err != nil {
		return parityResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	if startResult.Status != platform.RunStatusSuspended {
		return parityResult{Outcome: startResult.State.Outcome, Kind: startResult.State.Kind}
	}

	snap, found, loadErr := store.Load(s.ctx, def.ID, key)
	s.Require().NoError(loadErr)
	s.Require().True(found, "snapshot must exist after suspend")

	storedState, decErr := platform.NewCodec[steps.ExpenseState]().Decode(snap.State)
	s.Require().NoError(decErr)
	storedState.ResumeText = b.resumeText

	resumeBytes, encErr := platform.NewCodec[steps.ExpenseState]().Encode(storedState)
	s.Require().NoError(encErr)

	resumeResult, err := eng.Resume(s.ctx, def, key, resumeBytes)
	if err != nil {
		return parityResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	return parityResult{Outcome: resumeResult.State.Outcome, Kind: resumeResult.State.Kind}
}

func baseParityState() steps.ExpenseState {
	return steps.ExpenseState{
		UserID:          uuid.New(),
		Channel:         "whatsapp",
		MessageID:       "msg-parity-1",
		Kind:            intent.KindRecordExpense,
		TransactionKind: pendingexpense.TransactionKindExpense,
		AmountCents:     8000,
		Merchant:        "Mercado",
		CategoryHint:    "Alimentação",
		PaymentMethod:   "debit",
		Direction:       "outcome",
	}
}

func alwaysAuthorize(_ context.Context, _ steps.ExpenseState) bool         { return true }
func noReplayFn(_ context.Context, _ steps.ExpenseState) (string, bool)    { return "", false }
func allowPolicyFn(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" }
func successAuditFn(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
	return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
}
func autoCategoryFn(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
	st.CategoryID = "alimentacao"
	st.CategoryPath = "Alimentação"
	return st, nil
}
func successPersistFn(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
	return steps.PersistResult{AmountCents: 8000, CategoryPath: "Alimentação"}, nil
}

func (s *ParitySuite) TestParity_AutoLog() {
	b := parityBehaviors{
		authorize:    func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:       func() steps.ReplayFunc { return noReplayFn }(),
		policy:       func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeRouted, kernel.Outcome)
}

func (s *ParitySuite) TestParity_AuthzDenied() {
	b := parityBehaviors{
		authorize: func() steps.AuthorizeFunc {
			return func(_ context.Context, _ steps.ExpenseState) bool { return false }
		}(),
		replay:       func() steps.ReplayFunc { return noReplayFn }(),
		policy:       func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeAuthzDenied, kernel.Outcome)
}

func (s *ParitySuite) TestParity_Replay() {
	b := parityBehaviors{
		authorize: func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay: func() steps.ReplayFunc {
			return func(_ context.Context, _ steps.ExpenseState) (string, bool) {
				return "resposta anterior", true
			}
		}(),
		policy:       func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeReplay, kernel.Outcome)
}

func (s *ParitySuite) TestParity_PolicyBlocked() {
	b := parityBehaviors{
		authorize: func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return noReplayFn }(),
		policy: func() steps.PolicyFunc {
			return func(_ context.Context, _ steps.ExpenseState) (bool, string) {
				return true, "baixa confiança"
			}
		}(),
		auditBegin:   func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomePolicyBlocked, kernel.Outcome)
}

func (s *ParitySuite) TestParity_AmbiguousChoiceResume() {
	candidates := []string{"Alimentação > Restaurante", "Alimentação > Mercado"}
	b := parityBehaviors{
		authorize:  func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return noReplayFn }(),
		policy:     func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver: func() steps.CategoryResolverFunc {
			return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				if st.ForceCategory == nil && st.AwaitingKind == "" {
					return st, &tools.CategoryAmbiguousError{Hint: st.CategoryHint, Candidates: candidates}
				}
				st.CategoryID = candidates[1]
				st.CategoryPath = candidates[1]
				return st, nil
			}
		}(),
		persist:        func() steps.PersistFunc { return successPersistFn }(),
		initialState:   baseParityState(),
		resumeText:     "2",
		correlationKey: "user-ambig:whatsapp",
	}

	kernelClarify := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, kernelClarify.Outcome, "ambiguous initial: expected clarify")

	kernelResume := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, kernelResume.Outcome, "ambiguous resume: expected routed after choice")
}

func (s *ParitySuite) TestParity_NeedsConfirmResume() {
	candidates := []string{"Alimentação > Supermercado"}
	b := parityBehaviors{
		authorize:  func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return noReplayFn }(),
		policy:     func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver: func() steps.CategoryResolverFunc {
			return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				if st.ForceCategory == nil && st.AwaitingKind == "" {
					return st, &tools.CategoryNeedsConfirmationError{Hint: st.CategoryHint, Candidates: candidates}
				}
				st.CategoryID = candidates[0]
				st.CategoryPath = candidates[0]
				return st, nil
			}
		}(),
		persist:        func() steps.PersistFunc { return successPersistFn }(),
		initialState:   baseParityState(),
		resumeText:     "sim",
		correlationKey: "user-confirm:whatsapp",
	}

	kernelClarify := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, kernelClarify.Outcome, "needs_confirm initial: expected clarify")

	kernelResume := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, kernelResume.Outcome, "needs_confirm resume: expected routed after confirm")
}

func (s *ParitySuite) TestParity_NeedsConfirmCancel() {
	candidates := []string{"Alimentação > Supermercado"}
	b := parityBehaviors{
		authorize:  func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return noReplayFn }(),
		policy:     func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver: func() steps.CategoryResolverFunc {
			return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				if st.ForceCategory == nil && st.AwaitingKind == "" {
					return st, &tools.CategoryNeedsConfirmationError{Hint: st.CategoryHint, Candidates: candidates}
				}
				st.CategoryID = candidates[0]
				st.CategoryPath = candidates[0]
				return st, nil
			}
		}(),
		persist:        func() steps.PersistFunc { return successPersistFn }(),
		initialState:   baseParityState(),
		resumeText:     "não",
		correlationKey: "user-cancel:whatsapp",
	}

	kernelClarify := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, kernelClarify.Outcome, "cancel initial: expected clarify")

	kernelResume := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, kernelResume.Outcome, "cancel resume: expected routed after cancel")
}

func (s *ParitySuite) TestParity_UsecaseError() {
	b := parityBehaviors{
		authorize:  func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return noReplayFn }(),
		policy:     func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver:   func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist: func() steps.PersistFunc {
			return func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
				return steps.PersistResult{}, errors.New("db error")
			}
		}(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeUsecaseError, kernel.Outcome)
}

func (s *ParitySuite) TestParity_AuditConflictMapsToReplay() {
	b := parityBehaviors{
		authorize: func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return noReplayFn }(),
		policy:    func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc {
			return func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
				return steps.AuditBeginResult{Conflicted: true}
			}
		}(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeReplay, kernel.Outcome)
}

func (s *ParitySuite) TestParity_AuditFailMapsToUsecaseError() {
	b := parityBehaviors{
		authorize: func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return noReplayFn }(),
		policy:    func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc {
			return func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
				return steps.AuditBeginResult{Failed: true}
			}
		}(),
		resolver:     func() steps.CategoryResolverFunc { return autoCategoryFn }(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeUsecaseError, kernel.Outcome)
}

func (s *ParitySuite) TestParity_ResumeMinimalDeltaPreservesRichState() {
	candidates := []string{"Alimentação > Restaurante", "Alimentação > Mercado"}
	def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
		Authorize:  alwaysAuthorize,
		Replay:     noReplayFn,
		Policy:     allowPolicyFn,
		AuditBegin: successAuditFn,
		OnSettle:   nil,
		Resolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
			if st.ForceCategory == nil && st.AwaitingKind == "" {
				return st, &tools.CategoryAmbiguousError{Hint: st.CategoryHint, Candidates: candidates}
			}
			st.CategoryID = candidates[1]
			st.CategoryPath = candidates[1]
			return st, nil
		},
		Persist:        successPersistFn,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha auditoria",
	})

	store := newParityStore()
	eng := platform.NewEngine[steps.ExpenseState](store, s.obs)
	key := "user-mergepatch:whatsapp"
	initial := baseParityState()
	initial.AmountCents = 8000
	initial.Merchant = "Mercado"

	startResult, err := eng.Start(s.ctx, def, key, initial)
	s.Require().NoError(err)
	s.Require().Equal(platform.RunStatusSuspended, startResult.Status, "estado ambíguo deve suspender")

	snap, found, loadErr := store.Load(s.ctx, def.ID, key)
	s.Require().NoError(loadErr)
	s.Require().True(found, "snapshot deve existir após suspender")
	suspended, decErr := platform.NewCodec[steps.ExpenseState]().Decode(snap.State)
	s.Require().NoError(decErr)
	s.Require().Equal(int64(8000), suspended.AmountCents, "snapshot suspenso deve guardar o estado rico")
	s.Require().NotEmpty(suspended.Candidates, "snapshot suspenso deve guardar os candidates")

	resumeDelta := []byte(`{"ResumeText":"2"}`)
	resumeResult, err := eng.Resume(s.ctx, def, key, resumeDelta)
	s.Require().NoError(err)
	s.Equal(tools.OutcomeRouted, resumeResult.State.Outcome, "delta mínimo deve retomar e rotear (prova de merge-patch, não substituição)")
	s.Equal(int64(8000), resumeResult.State.AmountCents, "merge-patch deve preservar AmountCents do snapshot")
	s.Equal("Mercado", resumeResult.State.Merchant, "merge-patch deve preservar Merchant do snapshot")
	s.Require().NotNil(resumeResult.State.ForceCategory, "escolha deve resolver via Candidates preservados no snapshot")
	s.Equal(candidates[1], *resumeResult.State.ForceCategory, "escolha 2 deve casar o candidate preservado pelo merge-patch")
	s.Equal("2", resumeResult.State.ResumeText, "delta deve ter aplicado ResumeText")
}

func (s *ParitySuite) TestParity_MissingResolver() {
	b := parityBehaviors{
		authorize:  func() steps.AuthorizeFunc { return alwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return noReplayFn }(),
		policy:     func() steps.PolicyFunc { return allowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return successAuditFn }(),
		resolver: func() steps.CategoryResolverFunc {
			return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				return st, tools.ErrCategoryHintMissing
			}
		}(),
		persist:      func() steps.PersistFunc { return successPersistFn }(),
		initialState: baseParityState(),
	}
	kernel := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, kernel.Outcome)
}
