package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/capability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const defaultPolicyMinConfidence = 0.8

const settleRegistryTTL = 30 * 24 * time.Hour

type settleEntry struct {
	fn        steps.AuditSettleFunc
	expiresAt time.Time
}

type SettleRegistry struct {
	mu      sync.Mutex
	entries map[uuid.UUID]settleEntry
}

func NewSettleRegistry() *SettleRegistry {
	return &SettleRegistry{entries: make(map[uuid.UUID]settleEntry)}
}

func (r *SettleRegistry) Register(id uuid.UUID, fn steps.AuditSettleFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for k, e := range r.entries {
		if now.After(e.expiresAt) {
			delete(r.entries, k)
		}
	}
	r.entries[id] = settleEntry{fn: fn, expiresAt: now.Add(settleRegistryTTL)}
}

func (r *SettleRegistry) pop(id uuid.UUID) (steps.AuditSettleFunc, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[id]
	if !ok {
		return nil, false
	}
	delete(r.entries, id)
	if time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.fn, true
}

type DailyLedgerAgent struct {
	parser                   IntentParser
	monthlySummary           tools.MonthlySummaryReader
	cardLister               tools.CardLister
	cardInvoice              tools.CardInvoiceReader
	cardCreator              tools.CardCreator
	cardCounter              tools.CardCounter
	cardUpdater              tools.CardUpdater
	cardDeleter              tools.CardDeleter
	categoryPercentageEditor tools.CategoryPercentageEditor
	expenseRecorder          tools.ExpenseRecorder
	cardPurchaseLog          tools.CardPurchaseLogger
	transactionLister        tools.TransactionLister
	incomeSummaryReader      tools.IncomeSummaryReader
	lastDeleter              tools.LastTransactionDeleter
	lastEditor               tools.LastTransactionEditor
	recurringCreator         tools.RecurringCreator
	recurringLister          tools.RecurringLister
	budgetRecurrenceCreator  tools.BudgetRecurrenceCreator
	budgetConvo              tools.BudgetConversation
	budgetCommitter          tools.BudgetConfigCommitter
	budgetSession            tools.BudgetSessionGateway
	fallback                 tools.Fallback
	auditor                  *decisionAuditor
	policy                   domainservices.PolicyEvaluator
	o11y                     observability.Observability
	routedTotal              observability.Counter
	authzDeniedTotal         observability.Counter
	policyBlockedTotal       observability.Counter
	idempotencyReplayTotal   observability.Counter
	loc                      *time.Location
	catalog                  *capability.Catalog
	registry                 *workflow.IntentRegistry
	recorder                 *tools.Recorder
	clarification            *tools.ClarificationResolver
	budgetRunner             *tools.BudgetSessionRunner
	conversational           *tools.Conversational
	kernelEngine             platform.Engine[steps.ExpenseState]
	kernelDef                platform.Definition[steps.ExpenseState]
	settleReg                *SettleRegistry
	confirmEngine            platform.Engine[confirmation.ConfirmState]
	confirmDef               platform.Definition[confirmation.ConfirmState]
	planExecutor             *workflow.PlanExecutor
}

func newDailyLedgerAgent(o11y observability.Observability, routedTotal, authzDeniedTotal, policyBlockedTotal, idempotencyReplayTotal observability.Counter, loc *time.Location, deps IntentRouterDeps) (*DailyLedgerAgent, error) {
	catalog := deps.CapabilityCatalog
	if catalog == nil {
		var err error
		catalog, err = capability.BuildCatalog()
		if err != nil {
			return nil, fmt.Errorf("construir capability catalog: %w", err)
		}
	}
	agent := &DailyLedgerAgent{
		parser:                   deps.Parser,
		monthlySummary:           deps.MonthlySummary,
		cardLister:               deps.CardLister,
		cardInvoice:              deps.CardInvoice,
		cardCreator:              deps.CardCreator,
		cardCounter:              deps.CardCounter,
		cardUpdater:              deps.CardUpdater,
		cardDeleter:              deps.CardDeleter,
		categoryPercentageEditor: deps.CategoryPercentageEditor,
		expenseRecorder:          deps.ExpenseRecorder,
		cardPurchaseLog:          deps.CardPurchaseLog,
		transactionLister:        deps.TransactionLister,
		incomeSummaryReader:      deps.IncomeSummaryReader,
		lastDeleter:              deps.LastDeleter,
		lastEditor:               deps.LastEditor,
		recurringCreator:         deps.RecurringCreator,
		recurringLister:          deps.RecurringLister,
		budgetRecurrenceCreator:  deps.BudgetRecurrenceCreator,
		budgetConvo:              deps.BudgetConvo,
		budgetCommitter:          deps.BudgetCommitter,
		budgetSession:            deps.BudgetSession,
		fallback:                 deps.Fallback,
		auditor:                  newDecisionAuditor(o11y, deps.Decision, deps.Redactor),
		policy:                   domainservices.NewPolicyEvaluator(resolvePolicyThreshold(deps.PolicyMinConfidence)),
		o11y:                     o11y,
		routedTotal:              routedTotal,
		authzDeniedTotal:         authzDeniedTotal,
		policyBlockedTotal:       policyBlockedTotal,
		idempotencyReplayTotal:   idempotencyReplayTotal,
		loc:                      loc,
		catalog:                  catalog,
	}
	agent.recorder = tools.NewRecorder(routedTotal)
	agent.clarification = tools.NewClarificationResolver(agent.recorder, o11y)
	agent.budgetRunner = tools.NewBudgetSessionRunner(agent.recorder, deps.BudgetSession, deps.BudgetConvo, deps.BudgetCommitter, loc, o11y)
	agent.conversational = tools.NewConversational(agent.recorder, deps.Fallback, o11y)
	if deps.Kernel != nil && deps.Kernel.Engine != nil {
		agent.kernelEngine = deps.Kernel.Engine
		agent.settleReg = deps.Kernel.SettleReg
		agent.kernelDef = agent.buildKernelDefinition(deps.Kernel)
	}
	if deps.Kernel != nil && deps.Kernel.ConfirmEngine != nil {
		agent.confirmEngine = deps.Kernel.ConfirmEngine
		agent.confirmDef = deps.Kernel.ConfirmDef
		agent.wireBudgetCommitGate()
	}
	agent.planExecutor = deps.PlanExecutor
	if agent.planExecutor == nil && deps.Kernel != nil && deps.Kernel.PlanEngine != nil {
		planExecutor, err := workflow.NewPlanExecutor(deps.Kernel.PlanEngine, agent.executePlanStep, o11y)
		if err != nil {
			return nil, fmt.Errorf("construir plan executor: %w", err)
		}
		agent.planExecutor = planExecutor
	}
	registry, err := agent.buildRegistry()
	if err != nil {
		return nil, fmt.Errorf("construir workflow registry: %w", err)
	}
	agent.registry = registry
	return agent, nil
}

func resolvePolicyThreshold(raw float64) valueobjects.Confidence {
	value := raw
	if value <= 0 || value > 1 {
		value = defaultPolicyMinConfidence
	}
	confidence, err := valueobjects.NewConfidence(value)
	if err != nil {
		fallback, _ := valueobjects.NewConfidence(defaultPolicyMinConfidence)
		return fallback
	}
	return confidence
}

func (a *DailyLedgerAgent) tryResumeInbound(ctx context.Context, userID uuid.UUID, channel, text, messageID string) (bool, RouteResult) {
	if a.kernelEngine != (platform.Engine[steps.ExpenseState])(nil) {
		if handled, result := a.continuePendingExpenseConfirmation(ctx, userID, channel, text); handled {
			return true, result
		}
	}
	if a.planExecutor != nil {
		if handled, result := a.continuePendingPlan(ctx, userID, channel, text); handled {
			return true, result
		}
	}
	if a.confirmEngine != (platform.Engine[confirmation.ConfirmState])(nil) {
		if handled, result := a.continuePendingApproval(ctx, userID, channel, text, messageID); handled {
			return true, result
		}
	}
	if a.budgetRunner.Enabled() {
		if _, active := a.budgetRunner.Active(ctx, userID, channel); active {
			if handled, result := a.budgetRunner.Cancel(ctx, userID, channel, text); handled {
				return true, toRouteResult(result)
			}
		}
	}
	return false, RouteResult{}
}

func (a *DailyLedgerAgent) Handle(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	if handled, result := a.tryResumeInbound(ctx, principal.UserID, channel, text, messageID); handled {
		return result
	}

	parsed, err := a.parser.Parse(ctx, principal.UserID, text)
	if err != nil {
		if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
			span.RecordError(err)
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.parse_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		reply := a.delegateFallback(ctx, principal.UserID, channel, text)
		a.record(ctx, intent.KindUnknown.String(), channel, tools.OutcomeParseError)
		return RouteResult{Reply: reply, Outcome: tools.OutcomeParseError, Kind: intent.KindUnknown}
	}

	kind := parsed.Intent.Kind()
	if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
	}

	if kind == intent.KindUnknown && strings.TrimSpace(parsed.DirectReply) != "" {
		a.record(ctx, intent.KindUnknown.String(), channel, tools.OutcomeRouted)
		return RouteResult{Reply: parsed.DirectReply, Outcome: tools.OutcomeRouted, Kind: intent.KindUnknown}
	}

	if a.budgetRunner.Enabled() {
		if draft, active := a.budgetRunner.Active(ctx, principal.UserID, channel); active {
			change := budgetdraft.Change{
				TotalCents:  parsed.Intent.BudgetTotalCents(),
				Allocations: parsed.Intent.BudgetAllocations(),
			}
			return toRouteResult(a.budgetRunner.Resume(ctx, principal.UserID, channel, messageID, change, draft))
		}
	}

	if a.planExecutor != nil && !parsed.Plan.IsSingle() {
		return a.dispatchPlan(ctx, principal, channel, messageID, text, parsed)
	}

	if kind.IsWrite() {
		return a.dispatchWrite(ctx, principal, channel, messageID, text, parsed)
	}

	wf, ok := a.registry.Resolve(kind)
	if !ok {
		return a.routeFallback(ctx, principal.UserID, channel, kind, text)
	}
	result, execErr := wf.Execute(ctx, tools.ToolInput{UserID: principal.UserID, Channel: channel, Intent: parsed.Intent, Text: text})
	if execErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.workflow_execute_failed",
			observability.String("kind", kind.String()),
			observability.String("workflow", wf.ID()),
			observability.Error(execErr),
		)
		return a.routeFallback(ctx, principal.UserID, channel, kind, text)
	}
	return toRouteResult(result)
}

func (a *DailyLedgerAgent) dispatchPlan(ctx context.Context, principal Principal, channel, messageID, text string, parsed ParsedIntent) RouteResult {
	steps := make([]workflow.PlanStepItem, len(parsed.Plan.Steps))
	for i, s := range parsed.Plan.Steps {
		steps[i] = workflow.PlanStepItem{
			Intent:     s.Intent,
			Confidence: s.Confidence,
			Index:      s.Index,
		}
	}
	result, err := a.planExecutor.Execute(ctx, workflow.PlanInput{
		UserID:       principal.UserID,
		Channel:      channel,
		MessageID:    messageID,
		Text:         text,
		LLMModel:     parsed.LLMModel,
		PromptSHA256: parsed.PromptSHA256,
		DirectReply:  parsed.DirectReply,
		RawResponse:  string(parsed.Raw),
		Plan:         workflow.PlanSteps{Steps: steps},
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.plan_execute_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		reply := a.delegateFallback(ctx, principal.UserID, channel, text)
		a.record(ctx, parsed.Intent.Kind().String(), channel, tools.OutcomeFallback)
		return RouteResult{Reply: reply, Outcome: tools.OutcomeFallback, Kind: parsed.Intent.Kind()}
	}
	if result.Outcome == tools.OutcomeReplay && result.Reply == "" {
		result.Reply = alreadyProcessedText
	}
	a.record(ctx, parsed.Intent.Kind().String(), channel, result.Outcome)
	return RouteResult{Reply: result.Reply, Outcome: result.Outcome, Kind: parsed.Intent.Kind()}
}

func (a *DailyLedgerAgent) continuePendingPlan(ctx context.Context, userID uuid.UUID, channel, text string) (bool, RouteResult) {
	result, handled, err := a.planExecutor.Resume(ctx, userID, channel, text)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.plan_resume_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, RouteResult{}
	}
	if !handled {
		return false, RouteResult{}
	}
	if result.Outcome == tools.OutcomeReplay && result.Reply == "" {
		result.Reply = alreadyProcessedText
	}
	a.record(ctx, intent.KindUnknown.String(), channel, result.Outcome)
	return true, RouteResult{Reply: result.Reply, Outcome: result.Outcome, Kind: intent.KindUnknown}
}

func (a *DailyLedgerAgent) routeFallback(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, text string) RouteResult {
	reply := a.delegateFallback(ctx, userID, channel, text)
	a.record(ctx, kind.String(), channel, tools.OutcomeFallback)
	return RouteResult{Reply: reply, Outcome: tools.OutcomeFallback, Kind: kind}
}

func (a *DailyLedgerAgent) dispatchWrite(ctx context.Context, principal Principal, channel, messageID, trimmed string, parsed ParsedIntent) RouteResult {
	kind := parsed.Intent.Kind()

	if a.isDestructiveKind(kind) {
		if a.confirmEngine == (platform.Engine[confirmation.ConfirmState])(nil) {
			a.o11y.Logger().Error(ctx, "agent.intent_router.confirm_engine_missing_for_destructive",
				observability.String("kind", kind.String()),
			)
			a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
			return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
		}
		return a.dispatchWriteDestructive(ctx, principal, channel, messageID, parsed)
	}

	if kind.IsKernelWrite() {
		if a.kernelEngine == (platform.Engine[steps.ExpenseState])(nil) {
			a.o11y.Logger().Error(ctx, "agent.intent_router.kernel_engine_missing_for_write",
				observability.String("kind", kind.String()),
			)
			a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
			return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
		}
		return a.dispatchWriteKernel(ctx, principal, channel, messageID, trimmed, parsed, kind)
	}

	wf, ok := a.registry.Resolve(kind)
	if !ok {
		return a.routeFallback(ctx, principal.UserID, channel, kind, trimmed)
	}
	result, execErr := wf.Execute(ctx, tools.ToolInput{UserID: principal.UserID, Channel: channel, Intent: parsed.Intent, Text: trimmed})
	if execErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.write_workflow_execute_failed",
			observability.String("kind", kind.String()),
			observability.String("workflow", wf.ID()),
			observability.Error(execErr),
		)
		return a.routeFallback(ctx, principal.UserID, channel, kind, trimmed)
	}
	return toRouteResult(result)
}

func (a *DailyLedgerAgent) dispatchWriteKernel(ctx context.Context, principal Principal, channel, messageID, trimmed string, parsed ParsedIntent, kind intent.Kind) RouteResult {
	ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: principal.UserID, Source: auth.SourceWhatsApp})
	in := tools.ToolInput{
		UserID:       principal.UserID,
		Channel:      channel,
		Intent:       parsed.Intent,
		StepIndex:    parsed.StepIndex,
		MessageID:    messageID,
		Text:         trimmed,
		Confidence:   parsed.Confidence,
		Parsed:       parsed,
		LLMModel:     parsed.LLMModel,
		PromptSHA256: parsed.PromptSHA256,
		DirectReply:  parsed.DirectReply,
		RawResponse:  string(parsed.Raw),
	}
	initial := workflow.ExpenseStateFromToolInput(in)
	correlationKey := fmt.Sprintf("%s:%s", principal.UserID.String(), channel)
	result, err := a.kernelEngine.Start(ctx, a.kernelDef, correlationKey, initial)
	if err != nil {
		if errors.Is(err, platform.ErrRunConflict) {
			a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind.String()))
			a.record(ctx, kind.String(), channel, tools.OutcomeReplay)
			return RouteResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: kind}
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.kernel_start_failed",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
			observability.Error(err),
		)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	if result.Status == platform.RunStatusSuspended {
		if result.Suspend != nil {
			a.record(ctx, kind.String(), channel, tools.OutcomeClarify)
			return RouteResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify, Kind: kind}
		}
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	if result.Status == platform.RunStatusFailed {
		a.callSettle(ctx, result.State.DecisionID, false)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	a.callSettle(ctx, result.State.DecisionID, result.State.Outcome == tools.OutcomeRouted)
	toolResult := workflow.ExpenseStateToToolResult(result.State)
	a.record(ctx, kind.String(), channel, toolResult.Outcome)
	return RouteResult{Reply: toolResult.Reply, Outcome: toolResult.Outcome, Kind: toolResult.Kind}
}

func (a *DailyLedgerAgent) callSettle(ctx context.Context, decisionID uuid.UUID, executed bool) {
	if a.settleReg == nil || decisionID == uuid.Nil {
		return
	}
	fn, ok := a.settleReg.pop(decisionID)
	if !ok {
		return
	}
	fn(ctx, executed)
}

func (a *DailyLedgerAgent) authorizeWrite(ctx context.Context, principal Principal, effectiveUserID uuid.UUID, kind intent.Kind, channel string) bool {
	if effectiveUserID == principal.UserID && effectiveUserID != uuid.Nil {
		return true
	}
	a.o11y.Logger().Warn(ctx, "agent.intent_router.authz_denied",
		observability.String("kind", kind.String()),
		observability.String("channel", channel),
	)
	a.authzDeniedTotal.Add(ctx, 1, observability.String("kind", kind.String()))
	a.record(ctx, kind.String(), channel, tools.OutcomeAuthzDenied)
	return false
}

func (a *DailyLedgerAgent) replayDecision(ctx context.Context, userID uuid.UUID, channel, messageID string, stepIndex int, kind intent.Kind) (RouteResult, bool) {
	if a.auditor == nil || strings.TrimSpace(messageID) == "" {
		return RouteResult{}, false
	}
	priorReply, found := a.auditor.lookup(ctx, userID, channel, messageID, stepIndex)
	if !found {
		return RouteResult{}, false
	}
	a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind.String()))
	a.o11y.Logger().Info(ctx, "agent.intent_router.idempotent_replay",
		observability.String("kind", kind.String()),
		observability.String("channel", channel),
	)
	a.record(ctx, kind.String(), channel, tools.OutcomeReplay)
	reply := strings.TrimSpace(priorReply)
	if reply == "" {
		reply = alreadyProcessedText
	}
	return RouteResult{Reply: reply, Outcome: tools.OutcomeReplay, Kind: kind}, true
}

func (a *DailyLedgerAgent) beginDecisionAudit(ctx context.Context, principal Principal, channel, messageID string, kind intent.Kind, parsed ParsedIntent) decisionContext {
	if a.auditor == nil {
		return decisionContext{}
	}
	traceID := ""
	if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
		traceID = span.TraceID()
	}
	return a.auditor.begin(ctx, decisionRecordInput{
		UserID:       principal.UserID,
		Channel:      channel,
		MessageID:    messageID,
		IntentKind:   kind.String(),
		PromptSHA256: parsed.PromptSHA256,
		LLMModel:     parsed.LLMModel,
		TraceID:      traceID,
		DirectReply:  parsed.DirectReply,
		RawResponse:  parsed.Raw,
		StepIndex:    parsed.StepIndex,
	})
}

func (a *DailyLedgerAgent) executePlanStep(ctx context.Context, in workflow.PlanDispatchInput) (tools.ToolResult, error) {
	confidence, err := valueobjects.NewConfidence(min(max(in.Confidence, 0), 1))
	if err != nil {
		confidence, _ = valueobjects.NewConfidence(defaultPolicyMinConfidence)
	}
	parsed := ParsedIntent{
		Intent:       in.Intent,
		Confidence:   confidence,
		Raw:          []byte(in.RawResponse),
		DirectReply:  in.DirectReply,
		LLMModel:     in.LLMModel,
		PromptSHA256: in.PromptSHA256,
		StepIndex:    in.StepIndex,
	}
	principal := Principal{UserID: in.UserID}
	if in.Resuming && a.isDestructiveKind(in.Intent.Kind()) {
		handled, result := a.continuePendingApproval(ctx, in.UserID, in.Channel, in.ResumeText, in.MessageID)
		if !handled {
			return tools.ToolResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: in.Intent.Kind()}, nil
		}
		return toRouteToolResult(result), nil
	}
	if in.Intent.Kind().IsWrite() {
		return toRouteToolResult(a.dispatchWrite(ctx, principal, in.Channel, in.MessageID, in.Text, parsed)), nil
	}
	wf, ok := a.registry.Resolve(in.Intent.Kind())
	if !ok {
		return tools.ToolResult{Reply: a.delegateFallback(ctx, in.UserID, in.Channel, in.Text), Outcome: tools.OutcomeFallback, Kind: in.Intent.Kind()}, nil
	}
	return wf.Execute(ctx, tools.ToolInput{
		UserID:       in.UserID,
		Channel:      in.Channel,
		Intent:       in.Intent,
		StepIndex:    in.StepIndex,
		MessageID:    in.MessageID,
		Text:         in.Text,
		Confidence:   confidence,
		Parsed:       parsed,
		LLMModel:     in.LLMModel,
		PromptSHA256: in.PromptSHA256,
		DirectReply:  in.DirectReply,
		RawResponse:  in.RawResponse,
	})
}

func (a *DailyLedgerAgent) delegateFallback(ctx context.Context, userID uuid.UUID, channel, text string) string {
	return a.conversational.Reply(ctx, userID, channel, text)
}

func (a *DailyLedgerAgent) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	a.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}

func (a *DailyLedgerAgent) continuePendingExpenseConfirmation(ctx context.Context, userID uuid.UUID, channel, text string) (bool, RouteResult) {
	ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	correlationKey := fmt.Sprintf("%s:%s", userID.String(), channel)
	resumeDelta := struct {
		ResumeText string `json:"ResumeText"`
	}{ResumeText: text}
	resumeBytes, err := json.Marshal(resumeDelta)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.kernel_resume_encode_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, RouteResult{}
	}
	result, err := a.kernelEngine.Resume(ctx, a.kernelDef, correlationKey, resumeBytes)
	if err != nil {
		if errors.Is(err, platform.ErrRunConflict) {
			a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", intent.KindRecordExpense.String()))
			a.record(ctx, intent.KindRecordExpense.String(), channel, tools.OutcomeReplay)
			return true, RouteResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: intent.KindRecordExpense}
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.kernel_resume_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return true, RouteResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindRecordExpense}
	}
	if result.Status == platform.RunStatusRunning || result.RunID == uuid.Nil {
		return false, RouteResult{}
	}
	kind := result.State.Kind
	if result.Status == platform.RunStatusSuspended {
		if result.Suspend != nil {
			a.record(ctx, kind.String(), channel, tools.OutcomeClarify)
			return true, RouteResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify, Kind: kind}
		}
		return false, RouteResult{}
	}
	if result.Status == platform.RunStatusFailed {
		a.callSettle(ctx, result.State.DecisionID, false)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return true, RouteResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	a.callSettle(ctx, result.State.DecisionID, result.State.Outcome == tools.OutcomeRouted)
	toolResult := workflow.ExpenseStateToToolResult(result.State)
	a.record(ctx, kind.String(), channel, toolResult.Outcome)
	return true, RouteResult{Reply: toolResult.Reply, Outcome: toolResult.Outcome, Kind: toolResult.Kind}
}

func (a *DailyLedgerAgent) wireBudgetCommitGate() {
	if a.budgetRunner == nil {
		return
	}
	engine := a.confirmEngine
	def := a.confirmDef
	a.budgetRunner.WithCommitGate(func(ctx context.Context, userID uuid.UUID, channel, messageID string, draft budgetdraft.Draft) (bool, tools.ToolResult) {
		correlationKey := fmt.Sprintf("%s:%s", userID.String(), channel)
		initial := confirmation.ConfirmState{
			OperationKind: confirmation.OperationBudgetCommit,
			UserID:        userID.String(),
			Channel:       channel,
			MessageID:     messageID,
			PromptText:    promptTextForOperation(confirmation.OperationBudgetCommit),
		}
		if err := initial.SetBudgetDraft(draft); err != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_commit_draft_encode_failed",
				observability.String("channel", channel),
				observability.Error(err),
			)
			return true, tools.ToolResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
		}
		result, err := engine.Start(ctx, def, correlationKey, initial)
		if err != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_commit_gate_failed",
				observability.String("channel", channel),
				observability.Error(err),
			)
			return true, tools.ToolResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
		}
		if result.Status == platform.RunStatusSuspended && result.Suspend != nil {
			a.record(ctx, intent.KindConfigureBudget.String(), channel, tools.OutcomeClarify)
			return true, tools.ToolResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify, Kind: intent.KindConfigureBudget}
		}
		return false, tools.ToolResult{}
	})
}

var intentToOperationKind = map[intent.Kind]confirmation.OperationKind{
	intent.KindDeleteLastTransaction:  confirmation.OperationDeleteLast,
	intent.KindEditLastTransaction:    confirmation.OperationEditLast,
	intent.KindDeleteCard:             confirmation.OperationDeleteCard,
	intent.KindDeleteTransactionByRef: confirmation.OperationDeleteByRef,
	intent.KindEditTransactionByRef:   confirmation.OperationEditByRef,
}

func (a *DailyLedgerAgent) isDestructiveKind(k intent.Kind) bool {
	if a == nil || a.catalog == nil {
		return false
	}
	spec, ok := a.catalog.Lookup(k)
	if !ok {
		return false
	}
	return spec.RequiresConfirmation
}

func resolveOperationKind(k intent.Kind) (confirmation.OperationKind, bool) {
	op, ok := intentToOperationKind[k]
	return op, ok
}

func (a *DailyLedgerAgent) dispatchWriteDestructive(ctx context.Context, principal Principal, channel, messageID string, parsed ParsedIntent) RouteResult {
	ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: principal.UserID, Source: auth.SourceWhatsApp})
	kind := parsed.Intent.Kind()
	opKind, ok := resolveOperationKind(kind)
	if !ok {
		a.record(ctx, kind.String(), channel, tools.OutcomeMissingResolver)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeMissingResolver, Kind: kind}
	}
	correlationKey := fmt.Sprintf("%s:%s", principal.UserID.String(), channel)
	initial := initialConfirmState(principal.UserID, channel, messageID, parsed, opKind)
	result, err := a.confirmEngine.Start(ctx, a.confirmDef, correlationKey, initial)
	if err != nil {
		return a.handleConfirmStartError(ctx, channel, kind, err)
	}
	if result.Status == platform.RunStatusSuspended {
		return a.handleConfirmSuspended(ctx, channel, kind, result)
	}
	return a.finalizeConfirmStart(ctx, principal, channel, kind, opKind, result.State)
}

func (a *DailyLedgerAgent) handleConfirmStartError(ctx context.Context, channel string, kind intent.Kind, err error) RouteResult {
	if errors.Is(err, platform.ErrRunConflict) {
		a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind.String()))
		a.record(ctx, kind.String(), channel, tools.OutcomeReplay)
		return RouteResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: kind}
	}
	a.o11y.Logger().Warn(ctx, "agent.intent_router.confirm_engine_start_failed",
		observability.String("kind", kind.String()),
		observability.String("channel", channel),
		observability.Error(err),
	)
	a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
	return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
}

func (a *DailyLedgerAgent) handleConfirmSuspended(ctx context.Context, channel string, kind intent.Kind, result platform.RunResult[confirmation.ConfirmState]) RouteResult {
	if result.Suspend != nil {
		a.record(ctx, kind.String(), channel, tools.OutcomeClarify)
		return RouteResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify, Kind: kind}
	}
	a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
	return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
}

func (a *DailyLedgerAgent) finalizeConfirmStart(ctx context.Context, principal Principal, channel string, kind intent.Kind, opKind confirmation.OperationKind, state confirmation.ConfirmState) RouteResult {
	if state.Outcome == int(tools.OutcomeUsecaseError) {
		decisionID, _ := uuid.Parse(state.DecisionID)
		a.callSettle(ctx, decisionID, false)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	outcome := tools.ToolOutcome(state.Outcome)
	if outcome == 0 {
		outcome = tools.OutcomeRouted
	}
	executed := outcome == tools.OutcomeRouted && !state.ShortCircuit && !state.Expired
	decisionID, _ := uuid.Parse(state.DecisionID)
	a.callSettle(ctx, decisionID, executed)
	if opKind == confirmation.OperationBudgetCommit {
		a.clearBudgetSession(ctx, principal.UserID, channel)
	}
	a.record(ctx, kind.String(), channel, outcome)
	return RouteResult{Reply: state.Reply, Outcome: outcome, Kind: kind}
}

func initialConfirmState(userID uuid.UUID, channel, messageID string, parsed ParsedIntent, opKind confirmation.OperationKind) confirmation.ConfirmState {
	initial := confirmation.ConfirmState{
		OperationKind: opKind,
		UserID:        userID.String(),
		Channel:       channel,
		MessageID:     messageID,
		StepIndex:     parsed.StepIndex,
		PromptText:    promptTextForOperation(opKind),
	}
	if opKind == confirmation.OperationEditLast {
		initial.NewAmountCents = parsed.Intent.AmountCents()
	}
	if opKind == confirmation.OperationDeleteCard {
		initial.CardName = parsed.Intent.CardName()
	}
	if opKind == confirmation.OperationDeleteByRef {
		initial.SearchQuery = parsed.Intent.SearchQuery()
	}
	if opKind == confirmation.OperationEditByRef {
		initial.SearchQuery = parsed.Intent.SearchQuery()
		initial.NewAmountCents = parsed.Intent.AmountCents()
	}
	return initial
}

func toRouteToolResult(result RouteResult) tools.ToolResult {
	return tools.ToolResult{Reply: result.Reply, Outcome: result.Outcome, Kind: result.Kind}
}

func promptTextForOperation(op confirmation.OperationKind) string {
	switch op {
	case confirmation.OperationDeleteLast:
		return "Tem certeza que quer apagar o último lançamento? Responda *sim* para confirmar ou *não* para cancelar."
	case confirmation.OperationEditLast:
		return "Tem certeza que quer editar o último lançamento? Responda *sim* para confirmar ou *não* para cancelar."
	case confirmation.OperationDeleteCard:
		return "Tem certeza que quer remover este cartão? Responda *sim* para confirmar ou *não* para cancelar."
	case confirmation.OperationBudgetCommit:
		return "Orçamento completo! Quer ativar as configurações? Responda *sim* para confirmar ou *não* para cancelar."
	case confirmation.OperationDeleteByRef:
		return "Tem certeza que quer apagar esse lançamento? Responda *sim* para confirmar ou *não* para cancelar."
	case confirmation.OperationEditByRef:
		return "Tem certeza que quer editar esse lançamento? Responda *sim* para confirmar ou *não* para cancelar."
	default:
		return "Confirma a operação? Responda *sim* para confirmar ou *não* para cancelar."
	}
}

func (a *DailyLedgerAgent) continuePendingApproval(ctx context.Context, userID uuid.UUID, channel, text, messageID string) (bool, RouteResult) {
	ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	resumeBytes, ok := a.encodeConfirmResume(ctx, channel, text, messageID)
	if !ok {
		return false, RouteResult{}
	}
	correlationKey := fmt.Sprintf("%s:%s", userID.String(), channel)
	result, err := a.confirmEngine.Resume(ctx, a.confirmDef, correlationKey, resumeBytes)
	if err != nil {
		return a.handleConfirmResumeError(ctx, channel, err)
	}
	if result.Status == platform.RunStatusRunning || result.RunID == uuid.Nil {
		return false, RouteResult{}
	}
	if result.State.UserID != userID.String() {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.confirm_resume_user_mismatch",
			observability.String("channel", channel),
		)
		return false, RouteResult{}
	}
	opKind := result.State.OperationKind
	kind := resolveIntentKindFromOperation(opKind)
	if result.Status == platform.RunStatusSuspended {
		if result.Suspend != nil {
			a.record(ctx, kind.String(), channel, tools.OutcomeClarify)
			return true, RouteResult{Reply: result.Suspend.Prompt, Outcome: tools.OutcomeClarify, Kind: kind}
		}
		return false, RouteResult{}
	}
	if result.State.Expired {
		decisionID, _ := uuid.Parse(result.State.DecisionID)
		a.callSettle(ctx, decisionID, false)
		if opKind == confirmation.OperationBudgetCommit {
			a.clearBudgetSession(ctx, userID, channel)
		}
		return false, RouteResult{}
	}
	return a.finalizeConfirmResult(ctx, userID, channel, kind, opKind, result.State)
}

func (a *DailyLedgerAgent) encodeConfirmResume(ctx context.Context, channel, text, messageID string) ([]byte, bool) {
	resumeDelta := struct {
		ResumeText      string `json:"resume_text"`
		ResumeMessageID string `json:"resume_message_id"`
	}{ResumeText: text, ResumeMessageID: messageID}
	resumeBytes, err := json.Marshal(resumeDelta)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.confirm_resume_encode_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return nil, false
	}
	return resumeBytes, true
}

func (a *DailyLedgerAgent) handleConfirmResumeError(ctx context.Context, channel string, err error) (bool, RouteResult) {
	if errors.Is(err, platform.ErrRunConflict) {
		kind := intent.KindUnknown.String()
		a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind))
		a.record(ctx, kind, channel, tools.OutcomeReplay)
		return true, RouteResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: intent.KindUnknown}
	}
	a.o11y.Logger().Warn(ctx, "agent.intent_router.confirm_resume_failed",
		observability.String("channel", channel),
		observability.Error(err),
	)
	return false, RouteResult{}
}

func (a *DailyLedgerAgent) finalizeConfirmResult(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, opKind confirmation.OperationKind, state confirmation.ConfirmState) (bool, RouteResult) {
	if state.Outcome == int(tools.OutcomeUsecaseError) {
		decisionID, _ := uuid.Parse(state.DecisionID)
		a.callSettle(ctx, decisionID, false)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return true, RouteResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	outcome := tools.ToolOutcome(state.Outcome)
	if outcome == 0 {
		outcome = tools.OutcomeRouted
	}
	executed := outcome == tools.OutcomeRouted && !state.ShortCircuit && !state.Expired
	decisionID, _ := uuid.Parse(state.DecisionID)
	a.callSettle(ctx, decisionID, executed)
	if opKind == confirmation.OperationBudgetCommit {
		a.clearBudgetSession(ctx, userID, channel)
	}
	a.record(ctx, kind.String(), channel, outcome)
	return true, RouteResult{Reply: state.Reply, Outcome: outcome, Kind: kind}
}

func resolveIntentKindFromOperation(op confirmation.OperationKind) intent.Kind {
	switch op {
	case confirmation.OperationDeleteLast:
		return intent.KindDeleteLastTransaction
	case confirmation.OperationEditLast:
		return intent.KindEditLastTransaction
	case confirmation.OperationDeleteCard:
		return intent.KindDeleteCard
	case confirmation.OperationBudgetCommit:
		return intent.KindConfigureBudget
	case confirmation.OperationDeleteByRef:
		return intent.KindDeleteTransactionByRef
	case confirmation.OperationEditByRef:
		return intent.KindEditTransactionByRef
	default:
		return intent.KindUnknown
	}
}

func (a *DailyLedgerAgent) clearBudgetSession(ctx context.Context, userID uuid.UUID, channel string) {
	if a.budgetSession == nil {
		return
	}
	if clearErr := a.budgetSession.Clear(ctx, userID, channel); clearErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
			observability.String("channel", channel),
			observability.Error(clearErr),
		)
	}
}
