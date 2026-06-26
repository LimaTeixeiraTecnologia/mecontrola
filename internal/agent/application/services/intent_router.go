package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const ChannelWhatsApp = "whatsapp"

const authzDeniedText = "Não consegui concluir essa ação agora. Tente de novo em instantes 🙏"

const policyLowConfidenceText = "Não tenho certeza se entendi direito pra registrar isso. Pode reescrever com mais detalhes (valor, descrição e categoria)? 🙂"

const alreadyProcessedText = "Essa mensagem já foi processada ✅"

const auditWriteFailedText = "Não foi possível processar sua mensagem agora. Pode tentar de novo em instantes? 🙏"

var (
	ErrIntentParserNil    = errors.New("agent.intent_router: intent parser is nil")
	ErrObservabilityNil   = errors.New("agent.intent_router: observability is nil")
	ErrFallbackNil        = errors.New("agent.intent_router: fallback is nil")
	ErrWhatsAppGatewayNil = errors.New("agent.intent_router: whatsapp gateway is nil")
)

const fallbackMissingText = "Não recebi nenhuma mensagem. Me conta o que você precisa nas suas finanças 😊"

type ParsedIntent struct {
	Intent       intent.Intent
	Confidence   valueobjects.Confidence
	Raw          []byte
	DirectReply  string
	LLMModel     string
	PromptSHA256 string
	Plan         intent.IntentPlan
	StepIndex    int
}

type IntentParser interface {
	Parse(ctx context.Context, userID uuid.UUID, text string) (ParsedIntent, error)
}

type WhatsAppOutbound interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type Principal struct {
	UserID uuid.UUID
}

type InboundMessage struct {
	Text       string
	WhatsAppTo string
	MessageID  string
}

type RouteResult struct {
	Reply     string
	Outcome   tools.ToolOutcome
	Kind      intent.Kind
	Delivered bool
}

type IntentRouter struct {
	onboarding      onboardingHandler
	daily           *DailyLedgerAgent
	whatsAppGateway WhatsAppOutbound
	o11y            observability.Observability
	routedTotal     observability.Counter
	runtime         *AgentRuntime
}

type onboardingHandler interface {
	Handle(ctx context.Context, userID uuid.UUID, channel, peer, text, messageID string) (RouteResult, bool)
}

func (r *IntentRouter) EnableRuntime(catalog *capability.Catalog, threads ThreadGateway, runs RunGateway) {
	r.runtime = NewAgentRuntime(r.o11y, catalog, r, threads, runs)
}

func (r *IntentRouter) dispatch(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	if r.runtime != nil {
		return r.runtime.Execute(ctx, principal, channel, peer, text, messageID)
	}
	return r.route(ctx, principal, channel, peer, text, messageID)
}

type KernelDeps struct {
	Engine           platform.Engine[steps.ExpenseState]
	SettleReg        *SettleRegistry
	CategoryResolver steps.CategoryResolverFunc
	PersistFn        steps.PersistFunc
	AuditBeginFn     steps.AuditBeginFunc
	ConfirmEngine    platform.Engine[confirmation.ConfirmState]
	ConfirmDef       platform.Definition[confirmation.ConfirmState]
	PlanEngine       platform.Engine[workflow.PlanState]
	RetryPolicy      platform.RetryPolicy
	MaxAttempts      int
}

type IntentRouterDeps struct {
	CapabilityCatalog        *capability.Catalog
	Parser                   IntentParser
	MonthlySummary           tools.MonthlySummaryReader
	CardLister               tools.CardLister
	CardInvoice              tools.CardInvoiceReader
	CardCreator              tools.CardCreator
	CardCounter              tools.CardCounter
	CardUpdater              tools.CardUpdater
	CardDeleter              tools.CardDeleter
	CategoryPercentageEditor tools.CategoryPercentageEditor
	ExpenseRecorder          tools.ExpenseRecorder
	CardPurchaseLog          tools.CardPurchaseLogger
	TransactionLister        tools.TransactionLister
	TransactionSearcher      tools.TransactionSearcher
	IncomeSummaryReader      tools.IncomeSummaryReader
	LastDeleter              tools.LastTransactionDeleter
	LastEditor               tools.LastTransactionEditor
	RecurringCreator         tools.RecurringCreator
	RecurringLister          tools.RecurringLister
	BudgetRecurrenceCreator  tools.BudgetRecurrenceCreator
	BudgetConvo              tools.BudgetConversation
	BudgetCommitter          tools.BudgetConfigCommitter
	BudgetSession            tools.BudgetSessionGateway
	OnboardingEngine         platform.Engine[workflow.OnboardingState]
	OnboardingDef            platform.Definition[workflow.OnboardingState]
	OnboardingStore          platform.Store
	OnboardingStateChecker   OnboardingStateChecker
	OnboardingHistoryGateway workflow.HistoryGateway
	OnboardingHandler        onboardingHandler
	Fallback                 tools.Fallback
	WhatsAppGateway          WhatsAppOutbound
	Decision                 DecisionAuditDeps
	Redactor                 DecisionRedactor
	Location                 *time.Location
	PolicyMinConfidence      float64
	Kernel                   *KernelDeps
	PlanExecutor             *workflow.PlanExecutor
}

type DecisionRedactor interface {
	Clean(raw string) (string, error)
}

func NewIntentRouter(o11y observability.Observability, deps IntentRouterDeps) (*IntentRouter, error) {
	if o11y == nil {
		return nil, ErrObservabilityNil
	}
	if deps.Parser == nil {
		return nil, ErrIntentParserNil
	}
	if deps.Fallback == nil {
		return nil, ErrFallbackNil
	}
	if deps.WhatsAppGateway == nil {
		return nil, ErrWhatsAppGatewayNil
	}
	if deps.CapabilityCatalog == nil {
		catalog, err := capability.BuildCatalog()
		if err != nil {
			return nil, err
		}
		deps.CapabilityCatalog = catalog
	}
	loc := deps.Location
	if loc == nil {
		loc = time.UTC
	}
	routedTotal := o11y.Metrics().Counter(
		"agent_intent_routed_total",
		"Total de intents roteados pelo IntentRouter por kind, channel e outcome",
		"1",
	)
	authzDeniedTotal := o11y.Metrics().Counter(
		"agent_authz_denied_total",
		"Total de dispatches de escrita negados pela guarda de autorizacao por kind",
		"1",
	)
	policyBlockedTotal := o11y.Metrics().Counter(
		"agent_policy_blocks_total",
		"Total de dispatches de escrita bloqueados pela politica de confianca por kind",
		"1",
	)
	idempotencyReplayTotal := o11y.Metrics().Counter(
		"agent_idempotency_replay_total",
		"Total de dispatches de escrita servidos por replay idempotente por kind",
		"1",
	)
	daily, err := newDailyLedgerAgent(o11y, routedTotal, authzDeniedTotal, policyBlockedTotal, idempotencyReplayTotal, loc, deps)
	if err != nil {
		return nil, err
	}
	warnMissingToolBindings(o11y, deps)
	onboarding := deps.OnboardingHandler
	if onboarding == nil {
		onboarding = NewOnboardingAgent(o11y, routedTotal, deps.OnboardingEngine, deps.OnboardingDef, deps.OnboardingStore, deps.OnboardingStateChecker, deps.OnboardingHistoryGateway)
	}
	return &IntentRouter{
		onboarding:      onboarding,
		daily:           daily,
		whatsAppGateway: deps.WhatsAppGateway,
		o11y:            o11y,
		routedTotal:     routedTotal,
	}, nil
}

type toolBinding struct {
	name    string
	kind    intent.Kind
	present bool
}

func buildToolBindingEntries(deps IntentRouterDeps) []toolBinding {
	return []toolBinding{
		{name: "record_transaction", kind: intent.KindRecordExpense, present: deps.ExpenseRecorder != nil},
		{name: "record_income", kind: intent.KindRecordIncome, present: deps.ExpenseRecorder != nil},
		{name: "record_card_purchase", kind: intent.KindRecordCardPurchase, present: deps.CardPurchaseLog != nil},
		{name: "list_transactions", kind: intent.KindListTransactions, present: deps.TransactionLister != nil},
		{name: "create_recurring", kind: intent.KindCreateRecurring, present: deps.RecurringCreator != nil},
		{name: "list_recurring", kind: intent.KindListRecurring, present: deps.RecurringLister != nil},
		{name: "monthly_summary", kind: intent.KindMonthlySummary, present: deps.MonthlySummary != nil},
		{name: "how_am_i_doing", kind: intent.KindHowAmIDoing, present: deps.MonthlySummary != nil},
		{name: "query_category", kind: intent.KindQueryCategory, present: deps.MonthlySummary != nil},
		{name: "query_goal", kind: intent.KindQueryGoal, present: deps.MonthlySummary != nil},
		{name: "query_card", kind: intent.KindQueryCard, present: deps.CardLister != nil},
		{name: "configure_budget", kind: intent.KindConfigureBudget, present: deps.BudgetSession != nil},
		{name: "edit_category_percentage", kind: intent.KindEditCategoryPercentage, present: deps.CategoryPercentageEditor != nil},
		{name: "budget_recurrence", kind: intent.KindBudgetRecurrence, present: deps.BudgetRecurrenceCreator != nil},
		{name: "list_cards", kind: intent.KindListCards, present: deps.CardLister != nil},
		{name: "create_card", kind: intent.KindCreateCard, present: deps.CardCreator != nil},
		{name: "count_cards", kind: intent.KindCountCards, present: deps.CardCounter != nil},
		{name: "update_card", kind: intent.KindUpdateCard, present: deps.CardUpdater != nil},
		{name: "query_income_summary", kind: intent.KindQueryIncomeSummary, present: deps.IncomeSummaryReader != nil},
	}
}

func warnMissingToolBindingsKinds() []intent.Kind {
	entries := buildToolBindingEntries(IntentRouterDeps{})
	kinds := make([]intent.Kind, 0, len(entries))
	for _, e := range entries {
		kinds = append(kinds, e.kind)
	}
	return kinds
}

func warnMissingToolBindings(o11y observability.Observability, deps IntentRouterDeps) {
	for _, e := range buildToolBindingEntries(deps) {
		if !e.present {
			o11y.Logger().Warn(context.Background(), "agent.intent_router.tool_binding_ausente",
				observability.String("tool", e.name),
				observability.String("kind", e.kind.String()),
			)
		}
	}
}

func (r *IntentRouter) RouteWhatsApp(ctx context.Context, principal Principal, msg InboundMessage) RouteResult {
	result := r.dispatch(ctx, principal, ChannelWhatsApp, msg.WhatsAppTo, msg.Text, msg.MessageID)
	if result.Reply == "" {
		return result
	}
	if err := r.whatsAppGateway.SendTextMessage(ctx, msg.WhatsAppTo, result.Reply); err != nil {
		r.o11y.Logger().Warn(ctx, "agent.intent_router.whatsapp_send_failed",
			observability.String("kind", result.Kind.String()),
			observability.Error(err),
		)
		r.record(ctx, result.Kind.String(), ChannelWhatsApp, tools.OutcomeReplyFailed)
		return result
	}
	result.Delivered = true
	return result
}

func (r *IntentRouter) route(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	ctx, span := r.o11y.Tracer().Start(ctx, "agent.intent_router.route")
	defer span.End()

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		r.record(ctx, intent.KindUnknown.String(), channel, tools.OutcomeEmptyText)
		return RouteResult{Reply: fallbackMissingText, Outcome: tools.OutcomeEmptyText, Kind: intent.KindUnknown}
	}

	if handled, result := r.daily.tryResumeInbound(ctx, principal.UserID, channel, trimmed, messageID); handled {
		return result
	}

	if result, ok := r.onboarding.Handle(ctx, principal.UserID, channel, peer, trimmed, messageID); ok {
		return result
	}

	return r.daily.Handle(ctx, principal, channel, peer, trimmed, messageID)
}

func (r *IntentRouter) authorizeWrite(ctx context.Context, principal Principal, effectiveUserID uuid.UUID, kind intent.Kind, channel string) bool {
	return r.daily.authorizeWrite(ctx, principal, effectiveUserID, kind, channel)
}

func (r *IntentRouter) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	r.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}
