//go:build integration || e2e

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	agentvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	agentbinding "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/binding"
	agentrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/repositories"
	identityauth "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	identityrepos "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	wfpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
)

type Reply struct {
	To   string
	Text string
}

type CapturingGateway struct {
	mu      sync.Mutex
	replies []Reply
}

func (g *CapturingGateway) SendTextMessage(_ context.Context, toE164, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = append(g.replies, Reply{To: toE164, Text: text})
	return nil
}

func (g *CapturingGateway) LastReply() (Reply, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.replies) == 0 {
		return Reply{}, false
	}
	return g.replies[len(g.replies)-1], true
}

func (g *CapturingGateway) All() []Reply {
	g.mu.Lock()
	defer g.mu.Unlock()
	cp := make([]Reply, len(g.replies))
	copy(cp, g.replies)
	return cp
}

func (g *CapturingGateway) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.replies = nil
}

type parserAdapter struct{ uc *usecases.ParseInbound }

func (a *parserAdapter) Parse(ctx context.Context, userID uuid.UUID, text string) (appservices.ParsedIntent, error) {
	out, err := a.uc.Execute(ctx, usecases.ParseInboundInput{UserID: userID, Text: text})
	if err != nil {
		return appservices.ParsedIntent{}, err
	}
	return appservices.ParsedIntent{Intent: out.Intent, Confidence: out.Confidence, Raw: out.Raw, Plan: out.Plan}, nil
}

func fullConfidence() agentvo.Confidence {
	confidence, _ := agentvo.NewConfidence(1)
	return confidence
}

func strPtr(v string) *string { return &v }

func intPtr(v int) *int { return &v }

type StubParser struct {
	table     map[string]intent.Intent
	defaultFn func() intent.Intent
}

func NewStubParser(table map[string]intent.Intent, defaultFn func() intent.Intent) *StubParser {
	return &StubParser{table: table, defaultFn: defaultFn}
}

func (s *StubParser) Parse(_ context.Context, _ uuid.UUID, text string) (appservices.ParsedIntent, error) {
	if in, ok := s.table[text]; ok {
		return stubParsed(in), nil
	}
	if s.defaultFn != nil {
		return stubParsed(s.defaultFn()), nil
	}
	unknown, _ := intent.NewUnknown(text)
	return stubParsed(unknown), nil
}

func stubParsed(in intent.Intent) appservices.ParsedIntent {
	confidence := fullConfidence()
	plan, _ := intent.NewIntentPlan([]intent.IntentStep{
		{Intent: in, Confidence: confidence.Value(), Index: 0},
	})
	return appservices.ParsedIntent{Intent: in, Confidence: confidence, Plan: plan}
}

type StubFallback struct {
	DefaultReply string
}

func (s *StubFallback) Reply(_ context.Context, _ uuid.UUID, _, _ string) (string, error) {
	if s.DefaultReply != "" {
		return s.DefaultReply, nil
	}
	return "Não entendi. Pode reformular?", nil
}

func SeedActiveUserWA(t *testing.T, db database.DBTX, waNumber string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	o11y := noop.NewProvider()
	factory := identityrepos.NewRepositoryFactory(o11y)

	wa, err := identityvo.NewWhatsAppNumber(waNumber)
	if err != nil {
		t.Fatalf("e2e.seed: invalid wa number %q: %v", waNumber, err)
	}
	candidate := entities.New(wa)
	user, err := factory.UserRepository(db).UpsertByWhatsAppNumber(ctx, candidate, time.Now().UTC())
	if err != nil {
		t.Fatalf("e2e.seed: upsert user: %v", err)
	}
	userID, err := uuid.Parse(user.ID())
	if err != nil {
		t.Fatalf("e2e.seed: parse user id: %v", err)
	}

	entitlement := interfaces.EntitlementRecord{
		UserID:         userID.String(),
		SubscriptionID: uuid.New().String(),
		Status:         "ACTIVE",
		PeriodEnd:      time.Now().UTC().Add(365 * 24 * time.Hour),
	}
	if upsertErr := factory.EntitlementRepository(db).Upsert(ctx, entitlement); upsertErr != nil {
		t.Fatalf("e2e.seed: upsert entitlement: %v", upsertErr)
	}
	return userID
}

func buildConfirmKernelDeps(
	o11y observability.Observability,
	db *sqlx.DB,
	cfg *configs.Config,
	lister tools.TransactionLister,
	searcher tools.TransactionSearcher,
	lastEditor tools.LastTransactionEditor,
	lastDeleter tools.LastTransactionDeleter,
	cardLister tools.CardLister,
	cardDeleter tools.CardDeleter,
	categoryResolver steps.CategoryResolverFunc,
	persistFn steps.PersistFunc,
) (*appservices.KernelDeps, *appservices.SettleRegistry, error) {
	decisionRepoFact := agentrepo.NewDecisionRepositoryFactory(o11y)
	decisionUoW := uow.NewUnitOfWork(db)
	wfStoreFactory := wfpostgres.NewStoreFactory(o11y)
	store := wfStoreFactory.Store(db)
	settleReg := appservices.NewSettleRegistry()

	retryPolicy := platform.RetryPolicy{
		MaxAttempts: cfg.WorkflowKernelConfig.MaxAttempts,
		BaseBackoff: cfg.WorkflowKernelConfig.RetryBaseBackoff,
		MaxBackoff:  cfg.WorkflowKernelConfig.RetryMaxBackoff,
	}

	confirmEngine := platform.NewEngine[confirmation.ConfirmState](store, o11y)
	expenseEngine := platform.NewEngine[steps.ExpenseState](store, o11y)
	planEngine := platform.NewEngine[agentwf.PlanState](store, o11y)

	redactor, err := sanitize.NewSanitizer(sanitize.DefaultMaxRunes)
	if err != nil {
		return nil, nil, fmt.Errorf("sanitizer: %w", err)
	}
	decisionDeps := appservices.DecisionAuditDeps{Factory: decisionRepoFact, UoW: decisionUoW}

	targets := map[confirmation.OperationKind]steps.TargetResolver{
		confirmation.OperationDeleteLast:  agentbinding.NewLastTransactionDeleterResolver(lister),
		confirmation.OperationEditLast:    agentbinding.NewLastTransactionEditorResolver(lister),
		confirmation.OperationDeleteCard:  agentbinding.NewCardDeleterResolver(cardLister),
		confirmation.OperationDeleteByRef: agentbinding.NewDeleteByRefResolver(),
		confirmation.OperationEditByRef:   agentbinding.NewEditByRefResolver(),
	}
	executors := map[confirmation.OperationKind]steps.DestructiveExecutor{
		confirmation.OperationDeleteLast:  agentbinding.NewLastTransactionDeleterExecutor(lastDeleter),
		confirmation.OperationEditLast:    agentbinding.NewLastTransactionEditorExecutor(lastEditor),
		confirmation.OperationDeleteCard:  agentbinding.NewCardDeleterExecutorFn(cardDeleter),
		confirmation.OperationDeleteByRef: agentbinding.NewDeleteByRefExecutor(lastDeleter),
		confirmation.OperationEditByRef:   agentbinding.NewEditByRefExecutor(lastEditor),
	}

	confirmDef := agentwf.NewDestructiveConfirmDefinition(agentwf.DestructiveConfirmDeps{
		Authorize: func(ctx context.Context, state confirmation.ConfirmState) bool {
			principal, ok := identityauth.FromContext(ctx)
			if !ok {
				return false
			}
			uid, err := uuid.Parse(state.UserID)
			if err != nil {
				return false
			}
			return uid != uuid.Nil && principal.UserID == uid
		},
		Replay:     appservices.NewConfirmReplayFunc(o11y, decisionDeps, redactor),
		Policy:     func(_ context.Context, _ confirmation.ConfirmState) (bool, string) { return false, "" },
		AuditBegin: appservices.NewConfirmAuditBeginFunc(o11y, decisionDeps, redactor),
		OnSettle: func(id uuid.UUID, fn steps.ConfirmAuditSettleFunc) {
			settleReg.Register(id, func(ctx context.Context, executed bool) { fn(ctx, executed) })
		},
		Searcher:       searcher,
		Targets:        targets,
		Executors:      executors,
		TTL:            10 * time.Minute,
		DenyReply:      "Não consegui concluir essa ação agora. Tente de novo em instantes 🙏",
		ReplayReply:    "Essa mensagem já foi processada ✅",
		AuditFailReply: "Não foi possível processar sua mensagem agora. Pode tentar de novo em instantes? 🙏",
		RetryPolicy:    retryPolicy,
		MaxAttempts:    cfg.WorkflowKernelConfig.MaxAttempts,
		Observability:  o11y,
	})

	return &appservices.KernelDeps{
		Engine:           expenseEngine,
		PlanEngine:       planEngine,
		CategoryResolver: categoryResolver,
		PersistFn:        persistFn,
		SettleReg:        settleReg,
		ConfirmEngine:    confirmEngine,
		ConfirmDef:       confirmDef,
		RetryPolicy:      retryPolicy,
		MaxAttempts:      cfg.WorkflowKernelConfig.MaxAttempts,
	}, settleReg, nil
}
