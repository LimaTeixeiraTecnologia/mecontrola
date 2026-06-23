package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/budgetdraft"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/pendingexpense"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	cardinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	cardoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
)

const defaultPolicyMinConfidence = 0.8

type DailyLedgerAgent struct {
	parser                     IntentParser
	monthlySummary             MonthlySummaryReader
	cardLister                 CardLister
	cardInvoice                CardInvoiceReader
	cardCreator                CardCreator
	cardCounter                CardCounter
	cardUpdater                CardUpdater
	cardDeleter                CardDeleter
	categoryPercentageEditor   CategoryPercentageEditor
	expenseRecorder            ExpenseRecorder
	cardPurchaseLog            CardPurchaseLogger
	transactionLister          TransactionLister
	lastDeleter                LastTransactionDeleter
	lastEditor                 LastTransactionEditor
	recurringCreator           RecurringCreator
	recurringLister            RecurringLister
	budgetConfig               BudgetConfigurator
	budgetConvo                BudgetConversation
	budgetCommitter            BudgetConfigCommitter
	budgetSession              BudgetSessionGateway
	pendingExpenseConfirmation PendingExpenseConfirmationGateway
	fallback                   Fallback
	auditor                    *decisionAuditor
	policy                     domainservices.PolicyEvaluator
	o11y                       observability.Observability
	routedTotal                observability.Counter
	authzDeniedTotal           observability.Counter
	policyBlockedTotal         observability.Counter
	idempotencyReplayTotal     observability.Counter
	loc                        *time.Location
	registry                   *workflow.Registry
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

	if a.budgetSessionEnabled() {
		if handled, result := a.continuePendingBudgetSession(ctx, principal.UserID, channel, text); handled {
			return result
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
		a.record(ctx, intent.KindUnknown.String(), channel, OutcomeParseError)
		return RouteResult{Reply: reply, Outcome: OutcomeParseError, Kind: intent.KindUnknown}
	}

	kind := parsed.Intent.Kind()
	if span := a.o11y.Tracer().SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
	}

	if kind == intent.KindUnknown && strings.TrimSpace(parsed.DirectReply) != "" {
		a.record(ctx, intent.KindUnknown.String(), channel, OutcomeRouted)
		return RouteResult{Reply: parsed.DirectReply, Outcome: OutcomeRouted, Kind: intent.KindUnknown}
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
	a.record(ctx, kind.String(), channel, OutcomeFallback)
	return RouteResult{Reply: reply, Outcome: OutcomeFallback, Kind: kind}
}

func (a *DailyLedgerAgent) dispatchWrite(ctx context.Context, principal Principal, channel, messageID, trimmed string, parsed ParsedIntent) RouteResult {
	kind := parsed.Intent.Kind()

	wf, ok := a.registry.Resolve(kind)
	if !ok {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.unknown_write_kind",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		a.record(ctx, kind.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: OutcomeUsecaseError, Kind: kind}
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
		a.record(ctx, kind.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: auditWriteFailedText, Outcome: OutcomeUsecaseError, Kind: kind}
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
	a.record(ctx, kind.String(), channel, OutcomeAuthzDenied)
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
	a.record(ctx, kind.String(), channel, OutcomeReplay)
	reply := strings.TrimSpace(priorReply)
	if reply == "" {
		reply = alreadyProcessedText
	}
	return RouteResult{Reply: reply, Outcome: OutcomeReplay, Kind: kind}, true
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

func (a *DailyLedgerAgent) routeLogExpense(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.expenseRecorder == nil {
		a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindRecordExpense}
	}
	result, err := a.expenseRecorder.Execute(ctx, ExpenseRecorderInput{UserID: userID.String(), Intent: in})
	if err != nil {
		if clarify, ok := a.categoryClarification(ctx, userID, channel, intent.KindRecordExpense, in, err); ok {
			return clarify
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.log_expense_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindRecordExpense}
	}
	a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeRouted)
	reply := formatPersistedExpense(result.AmountCents, in.Merchant(), result.CategoryPath)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindRecordExpense}
}

func (a *DailyLedgerAgent) routeLogIncome(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.expenseRecorder == nil {
		a.record(ctx, intent.KindRecordIncome.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindRecordIncome}
	}
	result, err := a.expenseRecorder.Execute(ctx, ExpenseRecorderInput{UserID: userID.String(), Intent: in})
	if err != nil {
		if clarify, ok := a.categoryClarification(ctx, userID, channel, intent.KindRecordIncome, in, err); ok {
			return clarify
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.log_income_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindRecordIncome.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindRecordIncome}
	}
	a.record(ctx, intent.KindRecordIncome.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedIncome(result.AmountCents, in.Merchant(), result.CategoryPath), Outcome: OutcomeRouted, Kind: intent.KindRecordIncome}
}

func (a *DailyLedgerAgent) categoryClarification(ctx context.Context, userID uuid.UUID, channel string, kind intent.Kind, in intent.Intent, err error) (RouteResult, bool) {
	var ambiguous *CategoryAmbiguousError
	if errors.As(err, &ambiguous) {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.category_ambiguous",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: formatCategoryAmbiguous(ambiguous.Candidates), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if needsConfirmation, ok := errors.AsType[*CategoryNeedsConfirmationError](err); ok {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.category_needs_confirmation",
			observability.String("kind", kind.String()),
			observability.String("channel", channel),
		)
		if a.pendingExpenseConfirmation != nil && len(needsConfirmation.Candidates) > 0 {
			draft := a.buildPendingExpenseDraft(in, kind, needsConfirmation.Candidates[0])
			if saveErr := a.pendingExpenseConfirmation.Save(ctx, userID, channel, draft); saveErr != nil {
				a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_save_failed",
					observability.String("channel", channel),
					observability.Error(saveErr),
				)
			}
		}
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: formatCategoryNeedsConfirmation(needsConfirmation.Candidates), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrCategoryNotFound) {
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: formatCategoryNotFound(resolveCategoryHint(in)), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrCategoryHintMissing) {
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: categoryNoHintText, Outcome: OutcomeClarify, Kind: kind}, true
	}
	return RouteResult{}, false
}

func resolveCategoryHint(in intent.Intent) string {
	hint := strings.TrimSpace(in.CategoryHint())
	if hint == "" {
		hint = strings.TrimSpace(in.Merchant())
	}
	return hint
}

func (a *DailyLedgerAgent) routeMonthlySummary(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.monthlySummary == nil {
		a.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindMonthlySummary}
	}
	competence := in.RefMonth()
	if competence == "" {
		now := time.Now().UTC().In(a.loc)
		competence = fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	}
	summary, err := withReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return a.monthlySummary.Execute(ctx, userID.String(), competence)
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.monthly_summary_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		a.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindMonthlySummary}
	}
	a.record(ctx, intent.KindMonthlySummary.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatMonthlySummary(summary), Outcome: OutcomeRouted, Kind: intent.KindMonthlySummary}
}

func (a *DailyLedgerAgent) routeQueryCategory(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.monthlySummary == nil {
		a.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindQueryCategory}
	}
	now := time.Now().UTC().In(a.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := withReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return a.monthlySummary.Execute(ctx, userID.String(), competence)
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.query_category_failed",
			observability.String("category", in.CategoryName()),
			observability.Error(err),
		)
		a.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCategory}
	}
	a.record(ctx, intent.KindQueryCategory.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCategoryAllocation(summary, in.CategoryName()), Outcome: OutcomeRouted, Kind: intent.KindQueryCategory}
}

func (a *DailyLedgerAgent) routeQueryGoal(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.monthlySummary == nil {
		a.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: formatGoalUnavailable(in.GoalName()), Outcome: OutcomeMissingResolver, Kind: intent.KindQueryGoal}
	}
	now := time.Now().UTC().In(a.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := withReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return a.monthlySummary.Execute(ctx, userID.String(), competence)
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.query_goal_failed",
			observability.String("competence", competence),
			observability.Error(err),
		)
		a.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryGoal}
	}
	a.record(ctx, intent.KindQueryGoal.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatGoalProgress(summary, in.GoalName()), Outcome: OutcomeRouted, Kind: intent.KindQueryGoal}
}

func (a *DailyLedgerAgent) routeQueryCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.cardLister == nil || a.cardInvoice == nil {
		a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindQueryCard}
	}
	cards, err := withReadRetry(ctx, func(ctx context.Context) (cardoutput.CardList, error) {
		return a.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_list_failed",
			observability.String("card_name", in.CardName()),
			observability.Error(err),
		)
		a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	resolved, ok := resolveCardByName(cards, in.CardName())
	if !ok {
		a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{
			Reply:   formatCardNotFound(in.CardName()),
			Outcome: OutcomeMissingResolver,
			Kind:    intent.KindQueryCard,
		}
	}
	cardID, parseErr := uuid.Parse(resolved.ID)
	if parseErr != nil {
		a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	now := time.Now().UTC().In(a.loc)
	invoice, err := withReadRetry(ctx, func(ctx context.Context) (cardoutput.Invoice, error) {
		return a.cardInvoice.Execute(ctx, cardinput.InvoiceFor{
			CardID:   cardID,
			UserID:   userID,
			Purchase: now,
		})
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.query_card_invoice_failed",
			observability.String("card_name", in.CardName()),
			observability.Error(err),
		)
		a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindQueryCard}
	}
	a.record(ctx, intent.KindQueryCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardInvoice(resolved, invoice), Outcome: OutcomeRouted, Kind: intent.KindQueryCard}
}

func (a *DailyLedgerAgent) routeConfigureBudget(ctx context.Context, userID uuid.UUID, channel, text string) RouteResult {
	if a.budgetSessionEnabled() {
		return a.startBudgetSession(ctx, userID, channel, text)
	}
	if a.budgetConfig == nil {
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindConfigureBudget}
	}
	reply, err := a.budgetConfig.Start(ctx, userID, channel)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.configure_budget_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}
	if strings.TrimSpace(reply) == "" {
		reply = "Beleza! Qual a sua renda mensal? Pode me dizer o valor."
	}
	a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
}

func (a *DailyLedgerAgent) budgetSessionEnabled() bool {
	return a.budgetSession != nil && a.budgetConvo != nil && a.budgetCommitter != nil
}

func (a *DailyLedgerAgent) startBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string) RouteResult {
	now := time.Now().UTC().In(a.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	return a.advanceBudgetSession(ctx, userID, channel, text, budgetdraft.New(competence))
}

func (a *DailyLedgerAgent) continuePendingBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string) (bool, RouteResult) {
	draft, found, err := a.budgetSession.Load(ctx, userID, channel)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_load_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return false, RouteResult{}
	}
	if !found {
		return false, RouteResult{}
	}
	if matchesBudgetCancel(text) {
		if clearErr := a.budgetSession.Clear(ctx, userID, channel); clearErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
				observability.String("channel", channel),
				observability.Error(clearErr),
			)
		}
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return true, RouteResult{Reply: budgetCancelledText, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}
	return true, a.advanceBudgetSession(ctx, userID, channel, text, draft)
}

func (a *DailyLedgerAgent) advanceBudgetSession(ctx context.Context, userID uuid.UUID, channel, text string, draft budgetdraft.Draft) RouteResult {
	result, err := a.budgetConvo.Configure(ctx, text, draft)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_configure_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if !result.Complete {
		if saveErr := a.budgetSession.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
			a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
			return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
		}
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
		return RouteResult{Reply: result.Reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
	}

	reply, commitErr := a.budgetCommitter.Commit(ctx, userID, result.Draft)
	if commitErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_commit_failed",
			observability.String("channel", channel),
			observability.Error(commitErr),
		)
		if saveErr := a.budgetSession.Save(ctx, userID, channel, result.Draft); saveErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_save_failed",
				observability.String("channel", channel),
				observability.Error(saveErr),
			)
		}
		a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: reply, Outcome: OutcomeUsecaseError, Kind: intent.KindConfigureBudget}
	}

	if clearErr := a.budgetSession.Clear(ctx, userID, channel); clearErr != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.budget_session_clear_failed",
			observability.String("channel", channel),
			observability.Error(clearErr),
		)
	}
	a.record(ctx, intent.KindConfigureBudget.String(), channel, OutcomeRouted)
	return RouteResult{Reply: reply, Outcome: OutcomeRouted, Kind: intent.KindConfigureBudget}
}

func (a *DailyLedgerAgent) routeHowAmIDoing(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if a.monthlySummary == nil {
		a.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindHowAmIDoing}
	}
	now := time.Now().UTC().In(a.loc)
	competence := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	summary, err := withReadRetry(ctx, func(ctx context.Context) (budgetsoutput.MonthlySummaryOutput, error) {
		return a.monthlySummary.Execute(ctx, userID.String(), competence)
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.how_am_i_doing_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindHowAmIDoing}
	}
	a.record(ctx, intent.KindHowAmIDoing.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatHowAmIDoing(summary), Outcome: OutcomeRouted, Kind: intent.KindHowAmIDoing}
}

func (a *DailyLedgerAgent) routeLogCardPurchase(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.cardPurchaseLog == nil {
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindRecordCardPurchase}
	}
	result, err := a.cardPurchaseLog.Execute(ctx, CardPurchaseLoggerInput{UserID: userID.String(), Intent: in})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.log_card_purchase_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindRecordCardPurchase}
	}
	if !result.CardFound {
		a.record(ctx, intent.KindRecordCardPurchase.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: formatCardPurchaseCardMissing(in.CardHint()), Outcome: OutcomeMissingResolver, Kind: intent.KindRecordCardPurchase}
	}
	a.record(ctx, intent.KindRecordCardPurchase.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedCardPurchase(result), Outcome: OutcomeRouted, Kind: intent.KindRecordCardPurchase}
}

func (a *DailyLedgerAgent) routeListTransactions(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.transactionLister == nil {
		a.record(ctx, intent.KindListTransactions.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindListTransactions}
	}
	refMonth := in.RefMonth()
	if refMonth == "" {
		refMonth = a.currentCompetence()
	}
	list, err := withReadRetry(ctx, func(ctx context.Context) (TransactionListResult, error) {
		return a.transactionLister.Execute(ctx, TransactionListInput{UserID: userID.String(), RefMonth: refMonth})
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.list_transactions_failed",
			observability.String("ref_month", refMonth),
			observability.Error(err),
		)
		a.record(ctx, intent.KindListTransactions.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListTransactions}
	}
	a.record(ctx, intent.KindListTransactions.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatTransactionList(list), Outcome: OutcomeRouted, Kind: intent.KindListTransactions}
}

func (a *DailyLedgerAgent) routeDeleteLastTransaction(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if a.transactionLister == nil || a.lastDeleter == nil {
		a.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindDeleteLastTransaction}
	}
	last, found, err := a.mostRecentTransaction(ctx, userID)
	if err != nil {
		a.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindDeleteLastTransaction}
	}
	if !found {
		a.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeRouted)
		return RouteResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: intent.KindDeleteLastTransaction}
	}
	if err := a.lastDeleter.Execute(ctx, userID.String(), last.ID, last.Version); err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.delete_last_transaction_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindDeleteLastTransaction}
	}
	a.record(ctx, intent.KindDeleteLastTransaction.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatDeletedTransaction(last), Outcome: OutcomeRouted, Kind: intent.KindDeleteLastTransaction}
}

func (a *DailyLedgerAgent) routeEditLastTransaction(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.transactionLister == nil || a.lastEditor == nil {
		a.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindEditLastTransaction}
	}
	last, found, err := a.mostRecentTransaction(ctx, userID)
	if err != nil {
		a.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindEditLastTransaction}
	}
	if !found {
		a.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeRouted)
		return RouteResult{Reply: noTransactionsText, Outcome: OutcomeRouted, Kind: intent.KindEditLastTransaction}
	}
	result, err := a.lastEditor.Execute(ctx, EditTransactionInput{UserID: userID.String(), Current: last, NewAmount: in.AmountCents()})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.edit_last_transaction_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindEditLastTransaction}
	}
	a.record(ctx, intent.KindEditLastTransaction.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatEditedTransaction(result), Outcome: OutcomeRouted, Kind: intent.KindEditLastTransaction}
}

func (a *DailyLedgerAgent) routeCreateRecurring(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.recurringCreator == nil {
		a.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindCreateRecurring}
	}
	result, err := a.recurringCreator.Execute(ctx, RecurringCreatorInput{UserID: userID.String(), Intent: in})
	if err != nil {
		if clarify, ok := a.categoryClarification(ctx, userID, channel, intent.KindCreateRecurring, in, err); ok {
			return clarify
		}
		if errors.Is(err, ErrRecurringInvalidDay) {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.create_recurring_invalid_day",
				observability.Error(err),
			)
			a.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeClarify)
			return RouteResult{Reply: recurringInvalidDayText, Outcome: OutcomeClarify, Kind: intent.KindCreateRecurring}
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.create_recurring_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: registerFailedText(in.AmountCents(), in.Merchant()), Outcome: OutcomeUsecaseError, Kind: intent.KindCreateRecurring}
	}
	a.record(ctx, intent.KindCreateRecurring.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatPersistedRecurring(result), Outcome: OutcomeRouted, Kind: intent.KindCreateRecurring}
}

func (a *DailyLedgerAgent) routeListRecurring(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if a.recurringLister == nil {
		a.record(ctx, intent.KindListRecurring.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindListRecurring}
	}
	items, err := withReadRetry(ctx, func(ctx context.Context) ([]RecurringView, error) {
		return a.recurringLister.Execute(ctx, userID.String())
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.list_recurring_failed",
			observability.Error(err),
		)
		a.record(ctx, intent.KindListRecurring.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListRecurring}
	}
	a.record(ctx, intent.KindListRecurring.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatRecurringList(items), Outcome: OutcomeRouted, Kind: intent.KindListRecurring}
}

func (a *DailyLedgerAgent) routeListCards(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if a.cardLister == nil {
		a.record(ctx, intent.KindListCards.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindListCards}
	}
	cards, err := withReadRetry(ctx, func(ctx context.Context) (cardoutput.CardList, error) {
		return a.cardLister.Execute(ctx, cardinput.ListCards{UserID: userID, Limit: defaultListCardsLimit})
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.list_cards_failed", observability.Error(err))
		a.record(ctx, intent.KindListCards.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindListCards}
	}
	a.record(ctx, intent.KindListCards.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardList(cards), Outcome: OutcomeRouted, Kind: intent.KindListCards}
}

func (a *DailyLedgerAgent) routeCreateCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.cardCreator == nil {
		a.record(ctx, intent.KindCreateCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindCreateCard}
	}
	result, err := a.cardCreator.Execute(ctx, userID, in)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.create_card_failed", observability.Error(err))
		a.record(ctx, intent.KindCreateCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: createCardErrorText(err), Outcome: OutcomeUsecaseError, Kind: intent.KindCreateCard}
	}
	a.record(ctx, intent.KindCreateCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCreatedCard(result), Outcome: OutcomeRouted, Kind: intent.KindCreateCard}
}

func (a *DailyLedgerAgent) routeCountCards(ctx context.Context, userID uuid.UUID, channel string) RouteResult {
	if a.cardCounter == nil {
		a.record(ctx, intent.KindCountCards.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: fallbackParseError, Outcome: OutcomeMissingResolver, Kind: intent.KindCountCards}
	}
	total, err := withReadRetry(ctx, func(ctx context.Context) (int64, error) {
		return a.cardCounter.Execute(ctx, userID)
	})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.count_cards_failed", observability.Error(err))
		a.record(ctx, intent.KindCountCards.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindCountCards}
	}
	a.record(ctx, intent.KindCountCards.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCardCount(total), Outcome: OutcomeRouted, Kind: intent.KindCountCards}
}

func (a *DailyLedgerAgent) routeUpdateCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.cardUpdater == nil {
		a.record(ctx, intent.KindUpdateCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindUpdateCard}
	}
	result, err := a.cardUpdater.Execute(ctx, userID, in)
	if err != nil {
		if clarify, ok := a.cardResolutionClarification(ctx, channel, intent.KindUpdateCard, in.CardName(), err); ok {
			return clarify
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.update_card_failed", observability.Error(err))
		a.record(ctx, intent.KindUpdateCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: createCardErrorText(err), Outcome: OutcomeUsecaseError, Kind: intent.KindUpdateCard}
	}
	a.record(ctx, intent.KindUpdateCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatUpdatedCard(result), Outcome: OutcomeRouted, Kind: intent.KindUpdateCard}
}

func (a *DailyLedgerAgent) routeDeleteCard(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.cardDeleter == nil {
		a.record(ctx, intent.KindDeleteCard.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindDeleteCard}
	}
	result, err := a.cardDeleter.Execute(ctx, userID, in.CardName())
	if err != nil {
		if clarify, ok := a.cardResolutionClarification(ctx, channel, intent.KindDeleteCard, in.CardName(), err); ok {
			return clarify
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.delete_card_failed", observability.Error(err))
		a.record(ctx, intent.KindDeleteCard.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindDeleteCard}
	}
	a.record(ctx, intent.KindDeleteCard.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatDeletedCard(result), Outcome: OutcomeRouted, Kind: intent.KindDeleteCard}
}

func (a *DailyLedgerAgent) routeEditCategoryPercentage(ctx context.Context, userID uuid.UUID, channel string, in intent.Intent) RouteResult {
	if a.categoryPercentageEditor == nil {
		a.record(ctx, intent.KindEditCategoryPercentage.String(), channel, OutcomeMissingResolver)
		return RouteResult{Reply: registerUnavailableText, Outcome: OutcomeMissingResolver, Kind: intent.KindEditCategoryPercentage}
	}
	result, err := a.categoryPercentageEditor.Execute(ctx, CategoryPercentageEditorInput{
		UserID:       userID,
		Competence:   a.currentCompetence(),
		CategoryName: in.CategoryName(),
		Percentage:   in.Percentage(),
	})
	if err != nil {
		if errors.Is(err, ErrCategoryPercentageUnknownCategory) {
			a.record(ctx, intent.KindEditCategoryPercentage.String(), channel, OutcomeClarify)
			return RouteResult{Reply: formatCategoryNotFound(in.CategoryName()), Outcome: OutcomeClarify, Kind: intent.KindEditCategoryPercentage}
		}
		if errors.Is(err, ErrCategoryPercentageNoBudget) {
			a.record(ctx, intent.KindEditCategoryPercentage.String(), channel, OutcomeClarify)
			return RouteResult{Reply: budgetNotActiveText, Outcome: OutcomeClarify, Kind: intent.KindEditCategoryPercentage}
		}
		a.o11y.Logger().Warn(ctx, "agent.intent_router.edit_category_percentage_failed", observability.Error(err))
		a.record(ctx, intent.KindEditCategoryPercentage.String(), channel, OutcomeUsecaseError)
		return RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindEditCategoryPercentage}
	}
	a.record(ctx, intent.KindEditCategoryPercentage.String(), channel, OutcomeRouted)
	return RouteResult{Reply: formatCategoryPercentageUpdated(in.CategoryName(), result.Percentage), Outcome: OutcomeRouted, Kind: intent.KindEditCategoryPercentage}
}

func (a *DailyLedgerAgent) cardResolutionClarification(ctx context.Context, channel string, kind intent.Kind, cardName string, err error) (RouteResult, bool) {
	if errors.Is(err, ErrAgentCardAmbiguous) {
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: formatCardAmbiguous(cardName), Outcome: OutcomeClarify, Kind: kind}, true
	}
	if errors.Is(err, ErrAgentCardNotFound) {
		a.record(ctx, kind.String(), channel, OutcomeClarify)
		return RouteResult{Reply: formatCardNotFound(cardName), Outcome: OutcomeClarify, Kind: kind}, true
	}
	return RouteResult{}, false
}

func (a *DailyLedgerAgent) currentCompetence() string {
	now := time.Now().UTC().In(a.loc)
	return fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
}

func (a *DailyLedgerAgent) mostRecentTransaction(ctx context.Context, userID uuid.UUID) (TransactionView, bool, error) {
	list, err := a.transactionLister.Execute(ctx, TransactionListInput{UserID: userID.String(), RefMonth: a.currentCompetence()})
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.most_recent_transaction_failed",
			observability.Error(err),
		)
		return TransactionView{}, false, err
	}
	return pickMostRecent(list.Transactions)
}

func (a *DailyLedgerAgent) delegateFallback(ctx context.Context, userID uuid.UUID, channel, text string) string {
	reply, err := a.fallback.Reply(ctx, userID, channel, text)
	if err != nil {
		a.o11y.Logger().Warn(ctx, "agent.intent_router.fallback_failed",
			observability.String("channel", channel),
			observability.Error(err),
		)
		return fallbackParseError
	}
	if strings.TrimSpace(reply) == "" {
		return fallbackParseError
	}
	return reply
}

func (a *DailyLedgerAgent) record(ctx context.Context, kind, channel string, outcome tools.ToolOutcome) {
	a.routedTotal.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome.String()),
	)
}

const (
	expenseCancelledText  = "Ok, cancelei o lançamento. Quando quiser registrar, é só me dizer. 😊"
	directionOutcomeConst = "outcome"
	directionIncomeConst  = "income"
)

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
	if matchesExpenseConfirmation(text) {
		if clearErr := a.pendingExpenseConfirmation.Clear(ctx, userID, channel); clearErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_clear_failed",
				observability.String("channel", channel),
				observability.Error(clearErr),
			)
		}
		categoryID := draft.CategoryID
		result, err := a.expenseRecorder.Execute(ctx, ExpenseRecorderInput{
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
			a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeUsecaseError)
			return true, RouteResult{Reply: fallbackUsecaseError, Outcome: OutcomeUsecaseError, Kind: intent.KindRecordExpense}
		}
		a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeRouted)
		return true, RouteResult{
			Reply:   formatPersistedExpense(result.AmountCents, draft.Merchant, result.CategoryPath),
			Outcome: OutcomeRouted,
			Kind:    intent.KindRecordExpense,
		}
	}
	if matchesExpenseCancellation(text) {
		if clearErr := a.pendingExpenseConfirmation.Clear(ctx, userID, channel); clearErr != nil {
			a.o11y.Logger().Warn(ctx, "agent.intent_router.pending_expense_clear_failed",
				observability.String("channel", channel),
				observability.Error(clearErr),
			)
		}
		a.record(ctx, intent.KindRecordExpense.String(), channel, OutcomeRouted)
		return true, RouteResult{Reply: expenseCancelledText, Outcome: OutcomeRouted, Kind: intent.KindRecordExpense}
	}
	return false, RouteResult{}
}

func (a *DailyLedgerAgent) buildPendingExpenseDraft(in intent.Intent, kind intent.Kind, categoryPath string) pendingexpense.Draft {
	direction := directionOutcomeConst
	if kind == intent.KindRecordIncome {
		direction = directionIncomeConst
	}
	return pendingexpense.Draft{
		AmountCents:   in.AmountCents(),
		Merchant:      in.Merchant(),
		PaymentMethod: in.PaymentMethod(),
		Direction:     direction,
		OccurredAt:    "",
		CategoryID:    categoryPath,
		CategoryPath:  categoryPath,
	}
}

func matchesExpenseConfirmation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "sim" || t == "s" || t == "confirma" || t == "confirmado" || t == "pode" || t == "ok" || t == "yes"
}

func matchesExpenseCancellation(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return t == "não" || t == "nao" || t == "n" || t == "no" || t == "cancela" || t == "cancelar"
}
