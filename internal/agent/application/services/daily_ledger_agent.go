package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

const defaultPolicyMinConfidence = 0.8

type DailyLedgerAgent struct {
	parser                     IntentParser
	monthlySummary             tools.MonthlySummaryReader
	cardLister                 tools.CardLister
	cardInvoice                tools.CardInvoiceReader
	cardCreator                tools.CardCreator
	cardCounter                tools.CardCounter
	cardUpdater                tools.CardUpdater
	cardDeleter                tools.CardDeleter
	categoryPercentageEditor   tools.CategoryPercentageEditor
	expenseRecorder            tools.ExpenseRecorder
	cardPurchaseLog            tools.CardPurchaseLogger
	transactionLister          tools.TransactionLister
	lastDeleter                tools.LastTransactionDeleter
	lastEditor                 tools.LastTransactionEditor
	recurringCreator           tools.RecurringCreator
	recurringLister            tools.RecurringLister
	budgetConfig               tools.BudgetConfigurator
	budgetConvo                tools.BudgetConversation
	budgetCommitter            tools.BudgetConfigCommitter
	budgetSession              tools.BudgetSessionGateway
	pendingExpenseConfirmation tools.PendingExpenseConfirmationGateway
	fallback                   tools.Fallback
	auditor                    *decisionAuditor
	policy                     domainservices.PolicyEvaluator
	o11y                       observability.Observability
	routedTotal                observability.Counter
	authzDeniedTotal           observability.Counter
	policyBlockedTotal         observability.Counter
	idempotencyReplayTotal     observability.Counter
	loc                        *time.Location
	registry                   *workflow.Registry
	recorder                   *tools.Recorder
	clarification              *tools.ClarificationResolver
	budgetRunner               *tools.BudgetSessionRunner
	conversational             *tools.Conversational
}

func newDailyLedgerAgent(o11y observability.Observability, routedTotal, authzDeniedTotal, policyBlockedTotal, idempotencyReplayTotal observability.Counter, loc *time.Location, deps IntentRouterDeps) (*DailyLedgerAgent, error) {
	agent := &DailyLedgerAgent{
		parser:                     deps.Parser,
		monthlySummary:             deps.MonthlySummary,
		cardLister:                 deps.CardLister,
		cardInvoice:                deps.CardInvoice,
		cardCreator:                deps.CardCreator,
		cardCounter:                deps.CardCounter,
		cardUpdater:                deps.CardUpdater,
		cardDeleter:                deps.CardDeleter,
		categoryPercentageEditor:   deps.CategoryPercentageEditor,
		expenseRecorder:            deps.ExpenseRecorder,
		cardPurchaseLog:            deps.CardPurchaseLog,
		transactionLister:          deps.TransactionLister,
		lastDeleter:                deps.LastDeleter,
		lastEditor:                 deps.LastEditor,
		recurringCreator:           deps.RecurringCreator,
		recurringLister:            deps.RecurringLister,
		budgetConfig:               deps.BudgetConfig,
		budgetConvo:                deps.BudgetConvo,
		budgetCommitter:            deps.BudgetCommitter,
		budgetSession:              deps.BudgetSession,
		pendingExpenseConfirmation: deps.PendingExpenseConfirmation,
		fallback:                   deps.Fallback,
		auditor:                    newDecisionAuditor(o11y, deps.Decision, deps.Redactor),
		policy:                     domainservices.NewPolicyEvaluator(resolvePolicyThreshold(deps.PolicyMinConfidence)),
		o11y:                       o11y,
		routedTotal:                routedTotal,
		authzDeniedTotal:           authzDeniedTotal,
		policyBlockedTotal:         policyBlockedTotal,
		idempotencyReplayTotal:     idempotencyReplayTotal,
		loc:                        loc,
	}
	agent.recorder = tools.NewRecorder(routedTotal)
	agent.clarification = tools.NewClarificationResolver(deps.PendingExpenseConfirmation, agent.recorder, o11y)
	agent.budgetRunner = tools.NewBudgetSessionRunner(agent.recorder, deps.BudgetSession, deps.BudgetConvo, deps.BudgetCommitter, loc, o11y)
	agent.conversational = tools.NewConversational(agent.recorder, deps.Fallback, o11y)
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

func (a *DailyLedgerAgent) Handle(ctx context.Context, principal Principal, channel, peer, text, messageID string) RouteResult {
	if a.pendingExpenseConfirmation != nil {
		if handled, result := a.continuePendingExpenseConfirmation(ctx, principal.UserID, channel, text); handled {
			return result
		}
	}

	if a.budgetRunner.Enabled() {
		if handled, result := a.budgetRunner.Continue(ctx, principal.UserID, channel, text); handled {
			return toRouteResult(result)
		}
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

func (a *DailyLedgerAgent) routeFallback(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, text string) RouteResult {
	reply := a.delegateFallback(ctx, userID, channel, text)
	a.record(ctx, kind.String(), channel, tools.OutcomeFallback)
	return RouteResult{Reply: reply, Outcome: tools.OutcomeFallback, Kind: kind}
}

func (a *DailyLedgerAgent) dispatchWrite(ctx context.Context, principal Principal, channel, messageID, trimmed string, parsed ParsedIntent) RouteResult {
	kind := parsed.Intent.Kind()

	wf, ok := a.registry.Resolve(kind)
	if !ok {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.unknown_write_kind",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	result, execErr := wf.Execute(ctx, tools.ToolInput{
		UserID:     principal.UserID,
		Channel:    channel,
		Intent:     parsed.Intent,
		MessageID:  messageID,
		Text:       trimmed,
		Confidence: parsed.Confidence,
		Parsed:     parsed,
	})
	if execErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.workflow_execute_failed",
			observability.String("kind", kind.String()),
			observability.String("workflow", wf.ID()),
			observability.Error(execErr),
		)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	return toRouteResult(result)
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

func (a *DailyLedgerAgent) replayDecision(ctx context.Context, userID uuid.UUID, channel, messageID string, kind intent.Kind) (RouteResult, bool) {
	if a.auditor == nil || strings.TrimSpace(messageID) == "" {
		return RouteResult{}, false
	}
	priorReply, found := a.auditor.lookup(ctx, userID, channel, messageID)
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

const expenseCancelledText = "Ok, cancelei o lançamento. Quando quiser registrar, é só me dizer. 😊"

func (a *DailyLedgerAgent) continuePendingExpenseConfirmation(ctx context.Context, userID uuid.UUID, channel, text string) (bool, RouteResult) {
	draft, found, err := a.pendingExpenseConfirmation.Load(ctx, userID, channel)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_load_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, RouteResult{}
	}
	if !found {
		return false, RouteResult{}
	}
	if draft.AwaitingKind == pendingexpense.AwaitingCategoryChoice {
		return a.resolvePendingCategoryChoice(ctx, userID, channel, text, draft)
	}
	return a.resolvePendingCategoryConfirm(ctx, userID, channel, text, draft)
}

func (a *DailyLedgerAgent) resolvePendingCategoryConfirm(ctx context.Context, userID uuid.UUID, channel, text string, draft pendingexpense.Draft) (bool, RouteResult) {
	if matchesExpenseConfirmation(text) {
		a.clearPendingDraft(ctx, userID, channel)
		return true, a.executePendingDraft(ctx, userID, channel, draft)
	}
	if matchesExpenseCancellation(text) {
		a.clearPendingDraft(ctx, userID, channel)
		kind := resolveIntentKindFromDraft(draft)
		a.record(ctx, kind.String(), channel, tools.OutcomeRouted)
		return true, RouteResult{Reply: expenseCancelledText, Outcome: tools.OutcomeRouted, Kind: kind}
	}
	return false, RouteResult{}
}

func (a *DailyLedgerAgent) resolvePendingCategoryChoice(ctx context.Context, userID uuid.UUID, channel, text string, draft pendingexpense.Draft) (bool, RouteResult) {
	matched := matchCandidateByText(text, draft.Candidates)
	if matched == "" {
		return false, RouteResult{}
	}
	draft.CategoryID = matched
	draft.CategoryPath = matched
	a.clearPendingDraft(ctx, userID, channel)
	return true, a.executePendingDraft(ctx, userID, channel, draft)
}

func (a *DailyLedgerAgent) executePendingDraft(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) RouteResult {
	if draft.TransactionKind == pendingexpense.TransactionKindCardPurchase {
		return a.executePendingCardPurchase(ctx, userID, channel, draft)
	}
	return a.executePendingExpense(ctx, userID, channel, draft)
}

func (a *DailyLedgerAgent) executePendingExpense(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) RouteResult {
	categoryID := draft.CategoryID
	result, err := a.expenseRecorder.Execute(ctx, tools.ExpenseRecorderInput{
		UserID:        userID.String(),
		ForceCategory: &categoryID,
		AmountCents:   draft.AmountCents,
		Merchant:      draft.Merchant,
		PaymentMethod: draft.PaymentMethod,
		Direction:     draft.Direction,
		OccurredAt:    draft.OccurredAt,
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_execute_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		kind := resolveIntentKindFromDraft(draft)
		a.record(ctx, kind.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: kind}
	}
	kind := resolveIntentKindFromDraft(draft)
	a.record(ctx, kind.String(), channel, tools.OutcomeRouted)
	return RouteResult{
		Reply:   tools.FormatPersistedExpense(result.AmountCents, draft.Merchant, result.CategoryPath),
		Outcome: tools.OutcomeRouted,
		Kind:    kind,
	}
}

func (a *DailyLedgerAgent) executePendingCardPurchase(ctx context.Context, userID uuid.UUID, channel string, draft pendingexpense.Draft) RouteResult {
	if a.cardPurchaseLog == nil {
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, tools.OutcomeMissingResolver)
		return RouteResult{Reply: tools.RegisterUnavailableText, Outcome: tools.OutcomeMissingResolver, Kind: intent.KindRecordCardPurchase}
	}
	categoryID := draft.CategoryID
	result, err := a.cardPurchaseLog.Execute(ctx, tools.CardPurchaseLoggerInput{
		UserID:        userID.String(),
		ForceCategory: &categoryID,
		AmountCents:   draft.AmountCents,
		Merchant:      draft.Merchant,
		PaymentMethod: draft.PaymentMethod,
		CardHint:      draft.CardHint,
		Installments:  draft.Installments,
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_card_purchase_execute_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, tools.OutcomeUsecaseError)
		return RouteResult{Reply: tools.FallbackUsecaseError, Outcome: tools.OutcomeUsecaseError, Kind: intent.KindRecordCardPurchase}
	}
	if !result.CardFound {
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, tools.OutcomeMissingResolver)
		return RouteResult{Reply: tools.FormatCardPurchaseCardMissing(draft.CardHint), Outcome: tools.OutcomeMissingResolver, Kind: intent.KindRecordCardPurchase}
	}
	a.record(ctx, intent.KindRecordCardPurchase.String(), channel, tools.OutcomeRouted)
	return RouteResult{Reply: tools.FormatPersistedCardPurchase(result), Outcome: tools.OutcomeRouted, Kind: intent.KindRecordCardPurchase}
}

func (a *DailyLedgerAgent) clearPendingDraft(ctx context.Context, userID uuid.UUID, channel string) {
	if clearErr := a.pendingExpenseConfirmation.Clear(ctx, userID, channel); clearErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_clear_failed",
			observability.String("channel", channel),
			observability.Error(clearErr),
		)
	}
}

func resolveIntentKindFromDraft(draft pendingexpense.Draft) intent.Kind {
	switch draft.TransactionKind {
	case pendingexpense.TransactionKindIncome:
		return intent.KindRecordIncome
	case pendingexpense.TransactionKindCardPurchase:
		return intent.KindRecordCardPurchase
	default:
		return intent.KindRecordExpense
	}
}

func matchCandidateByText(text string, candidates []string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return ""
	}
	if idx, err := strconv.Atoi(normalized); err == nil && idx >= 1 && idx <= len(candidates) {
		return candidates[idx-1]
	}
	for _, candidate := range candidates {
		for _, segment := range strings.Split(strings.ToLower(candidate), " > ") {
			if strings.HasPrefix(strings.TrimSpace(segment), normalized) {
				return candidate
			}
		}
	}
	return ""
}

func matchesExpenseConfirmation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, word := range []string{"sim", "s", "confirma", "confirmado", "pode", "ok", "yes"} {
		if t == word || strings.HasPrefix(t, word+",") || strings.HasPrefix(t, word+" ") {
			return true
		}
	}
	return false
}

func matchesExpenseCancellation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "não" || t == "nao" || t == "n" || t == "no" || t == "cancela" || t == "cancelar"
}
