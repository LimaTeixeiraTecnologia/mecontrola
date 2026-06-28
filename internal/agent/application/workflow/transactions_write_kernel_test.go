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

type kernelStore struct {
	mu   sync.Mutex
	runs map[string]platform.Snapshot
}

func newKernelStore() *kernelStore {
	return &kernelStore{runs: make(map[string]platform.Snapshot)}
}

func (s *kernelStore) key(workflow, correlationKey string) string {
	return workflow + ":" + correlationKey
}

func (s *kernelStore) Insert(_ context.Context, snap platform.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *kernelStore) Load(_ context.Context, workflow, key string) (platform.Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, ok := s.runs[s.key(workflow, key)]
	return snap, ok, nil
}

func (s *kernelStore) Save(_ context.Context, snap platform.Snapshot, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[s.key(snap.Workflow, snap.CorrelationKey)] = snap
	return nil
}

func (s *kernelStore) AppendStep(_ context.Context, _ platform.StepRecord) error {
	return nil
}

func (s *kernelStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	return 0, nil
}

func (s *kernelStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]platform.Snapshot, error) {
	return nil, nil
}

type kernelRunResult struct {
	Outcome tools.ToolOutcome
	Kind    intent.Kind
}

type kernelBehaviors struct {
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

type TransactionsWriteKernelSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestTransactionsWriteKernelSuite(t *testing.T) {
	suite.Run(t, new(TransactionsWriteKernelSuite))
}

func (s *TransactionsWriteKernelSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
}

func (s *TransactionsWriteKernelSuite) newEngine() platform.Engine[steps.ExpenseState] {
	return platform.NewEngine[steps.ExpenseState](newKernelStore(), s.obs)
}

func (s *TransactionsWriteKernelSuite) runKernel(b kernelBehaviors) kernelRunResult {
	eng := s.newEngine()
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
		return kernelRunResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	return kernelRunResult{Outcome: result.State.Outcome, Kind: result.State.Kind}
}

func (s *TransactionsWriteKernelSuite) runKernelResume(b kernelBehaviors) kernelRunResult {
	store := newKernelStore()
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
		return kernelRunResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	if startResult.Status != platform.RunStatusSuspended {
		return kernelRunResult{Outcome: startResult.State.Outcome, Kind: startResult.State.Kind}
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
		return kernelRunResult{Outcome: tools.OutcomeUsecaseError, Kind: b.initialState.Kind}
	}
	return kernelRunResult{Outcome: resumeResult.State.Outcome, Kind: resumeResult.State.Kind}
}

func baseKernelState() steps.ExpenseState {
	return steps.ExpenseState{
		UserID:          uuid.New(),
		Channel:         "whatsapp",
		MessageID:       "msg-kernel-1",
		Kind:            intent.KindRecordExpense,
		TransactionKind: pendingexpense.TransactionKindExpense,
		AmountCents:     8000,
		Merchant:        "Mercado",
		CategoryHint:    "Alimentação",
		PaymentMethod:   "debit",
		Direction:       "outcome",
	}
}

func kernelAlwaysAuthorize(_ context.Context, _ steps.ExpenseState) bool         { return true }
func kernelNoReplayFn(_ context.Context, _ steps.ExpenseState) (string, bool)    { return "", false }
func kernelAllowPolicyFn(_ context.Context, _ steps.ExpenseState) (bool, string) { return false, "" }
func kernelSuccessAuditFn(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
	return steps.AuditBeginResult{Settle: func(_ context.Context, _ bool) {}}
}
func kernelAutoCategoryFn(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
	st.CategoryID = "alimentacao"
	st.CategoryPath = "Alimentação"
	return st, nil
}
func kernelSuccessPersistFn(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
	return steps.PersistResult{AmountCents: 8000, CategoryPath: "Alimentação"}, nil
}

func (s *TransactionsWriteKernelSuite) TestAutoLog() {
	b := kernelBehaviors{
		authorize:    func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:       func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:       func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeRouted, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestAuthzDenied() {
	b := kernelBehaviors{
		authorize: func() steps.AuthorizeFunc {
			return func(_ context.Context, _ steps.ExpenseState) bool { return false }
		}(),
		replay:       func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:       func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeAuthzDenied, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestReplay() {
	b := kernelBehaviors{
		authorize: func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay: func() steps.ReplayFunc {
			return func(_ context.Context, _ steps.ExpenseState) (string, bool) {
				return "resposta anterior", true
			}
		}(),
		policy:       func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin:   func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeReplay, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestPolicyBlocked() {
	b := kernelBehaviors{
		authorize: func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy: func() steps.PolicyFunc {
			return func(_ context.Context, _ steps.ExpenseState) (bool, string) {
				return true, "baixa confiança"
			}
		}(),
		auditBegin:   func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomePolicyBlocked, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestAmbiguousChoiceResume() {
	candidates := []string{"Alimentação > Restaurante", "Alimentação > Mercado"}
	b := kernelBehaviors{
		authorize:  func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:     func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
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
		persist:        func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState:   baseKernelState(),
		resumeText:     "2",
		correlationKey: "user-ambig:whatsapp",
	}

	initial := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, initial.Outcome, "ambiguous initial: expected clarify")

	resumed := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, resumed.Outcome, "ambiguous resume: expected routed after choice")
}

func (s *TransactionsWriteKernelSuite) TestNeedsConfirmResume() {
	candidates := []string{"Alimentação > Supermercado"}
	b := kernelBehaviors{
		authorize:  func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:     func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
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
		persist:        func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState:   baseKernelState(),
		resumeText:     "sim",
		correlationKey: "user-confirm:whatsapp",
	}

	initial := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, initial.Outcome, "needs_confirm initial: expected clarify")

	resumed := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, resumed.Outcome, "needs_confirm resume: expected routed after confirm")
}

func (s *TransactionsWriteKernelSuite) TestNeedsConfirmCancel() {
	candidates := []string{"Alimentação > Supermercado"}
	b := kernelBehaviors{
		authorize:  func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:     func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
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
		persist:        func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState:   baseKernelState(),
		resumeText:     "não",
		correlationKey: "user-cancel:whatsapp",
	}

	initial := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, initial.Outcome, "cancel initial: expected clarify")

	resumed := s.runKernelResume(b)
	s.Equal(tools.OutcomeRouted, resumed.Outcome, "cancel resume: expected routed after cancel")
}

func (s *TransactionsWriteKernelSuite) TestUsecaseError() {
	b := kernelBehaviors{
		authorize:  func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:     func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver:   func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist: func() steps.PersistFunc {
			return func(_ context.Context, _ steps.ExpenseState) (steps.PersistResult, error) {
				return steps.PersistResult{}, errors.New("db error")
			}
		}(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestAuditConflictMapsToReplay() {
	b := kernelBehaviors{
		authorize: func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:    func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc {
			return func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
				return steps.AuditBeginResult{Conflicted: true}
			}
		}(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeReplay, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestAuditFailMapsToUsecaseError() {
	b := kernelBehaviors{
		authorize: func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:    func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:    func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc {
			return func(_ context.Context, _ steps.ExpenseState) steps.AuditBeginResult {
				return steps.AuditBeginResult{Failed: true}
			}
		}(),
		resolver:     func() steps.CategoryResolverFunc { return kernelAutoCategoryFn }(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeUsecaseError, result.Outcome)
}

func (s *TransactionsWriteKernelSuite) TestResumeMinimalDeltaPreservesRichState() {
	candidates := []string{"Alimentação > Restaurante", "Alimentação > Mercado"}
	refs := []pendingexpense.CandidateRef{
		{RootCategoryID: uuid.NewString(), SubcategoryID: uuid.NewString()},
		{RootCategoryID: uuid.NewString(), SubcategoryID: uuid.NewString()},
	}
	def := NewTransactionsWriteDefinition(TransactionsWriteDeps{
		Authorize:  kernelAlwaysAuthorize,
		Replay:     kernelNoReplayFn,
		Policy:     kernelAllowPolicyFn,
		AuditBegin: kernelSuccessAuditFn,
		OnSettle:   nil,
		Resolver: func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
			if st.ForceCategory == nil && st.AwaitingKind == "" {
				return st, &tools.CategoryAmbiguousError{Hint: st.CategoryHint, Candidates: candidates, CandidateRefs: refs}
			}
			st.CategoryID = candidates[1]
			st.CategoryPath = candidates[1]
			return st, nil
		},
		Persist:        kernelSuccessPersistFn,
		DenyReply:      "negado",
		ReplayReply:    "replay",
		AuditFailReply: "falha auditoria",
	})

	store := newKernelStore()
	eng := platform.NewEngine[steps.ExpenseState](store, s.obs)
	key := "user-mergepatch:whatsapp"
	initial := baseKernelState()
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
	s.Equal(refs[1].RootCategoryID, *resumeResult.State.ForceCategory, "escolha 2 deve resolver o root UUID do ref preservado pelo merge-patch")
	s.Equal(refs[1].SubcategoryID, resumeResult.State.SubcategoryID, "escolha 2 deve resolver a subcategoria do ref preservado")
	s.Equal("2", resumeResult.State.ResumeText, "delta deve ter aplicado ResumeText")
}

func (s *TransactionsWriteKernelSuite) TestMissingResolver() {
	b := kernelBehaviors{
		authorize:  func() steps.AuthorizeFunc { return kernelAlwaysAuthorize }(),
		replay:     func() steps.ReplayFunc { return kernelNoReplayFn }(),
		policy:     func() steps.PolicyFunc { return kernelAllowPolicyFn }(),
		auditBegin: func() steps.AuditBeginFunc { return kernelSuccessAuditFn }(),
		resolver: func() steps.CategoryResolverFunc {
			return func(_ context.Context, st steps.ExpenseState) (steps.ExpenseState, error) {
				return st, tools.ErrCategoryHintMissing
			}
		}(),
		persist:      func() steps.PersistFunc { return kernelSuccessPersistFn }(),
		initialState: baseKernelState(),
	}
	result := s.runKernel(b)
	s.Equal(tools.OutcomeClarify, result.Outcome)
}
