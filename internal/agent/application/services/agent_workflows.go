package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	agentwf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow/steps"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

func toRouteResult(result tools.ToolResult) RouteResult {
	return RouteResult{Reply: result.Reply, Outcome: result.Outcome, Kind: result.Kind}
}

func (a *DailyLedgerAgent) buildRegistry() (*agentwf.IntentRegistry, error) {
	transactionsWorkflow, err := agentwf.NewIntentWorkflow("transactions",
		agentwf.KindTool{Kind: intent.KindRecordExpense, Tool: tools.NewRecordExpense(a.recorder, a.clarification, a.expenseRecorder, a.o11y)},
		agentwf.KindTool{Kind: intent.KindRecordIncome, Tool: tools.NewRecordIncome(a.recorder, a.clarification, a.expenseRecorder, a.o11y)},
		agentwf.KindTool{Kind: intent.KindRecordCardPurchase, Tool: tools.NewRecordCardPurchase(a.recorder, a.clarification, a.cardPurchaseLog, a.o11y)},
		agentwf.KindTool{Kind: intent.KindListTransactions, Tool: tools.NewListTransactions(a.recorder, a.transactionLister, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindCreateRecurring, Tool: tools.NewCreateRecurring(a.recorder, a.clarification, a.recurringCreator, a.o11y)},
		agentwf.KindTool{Kind: intent.KindListRecurring, Tool: tools.NewListRecurring(a.recorder, a.recurringLister, a.o11y)},
		agentwf.KindTool{Kind: intent.KindQueryIncomeSummary, Tool: tools.NewQueryIncomeSummary(a.recorder, a.incomeSummaryReader, a.loc, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	budgetWorkflow, err := agentwf.NewIntentWorkflow("budget",
		agentwf.KindTool{Kind: intent.KindMonthlySummary, Tool: tools.NewMonthlySummary(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindHowAmIDoing, Tool: tools.NewHowAmIDoing(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindQueryCategory, Tool: tools.NewQueryCategory(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindQueryGoal, Tool: tools.NewQueryGoal(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindQueryCard, Tool: tools.NewQueryCard(a.recorder, a.cardLister, a.cardInvoice, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindConfigureBudget, Tool: tools.NewConfigureBudget(a.recorder, a.budgetRunner, a.o11y)},
		agentwf.KindTool{Kind: intent.KindEditCategoryPercentage, Tool: tools.NewEditCategoryPercentage(a.recorder, a.categoryPercentageEditor, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindBudgetRecurrence, Tool: tools.NewBudgetRecurrenceCreatorTool(a.recorder, a.budgetRecurrenceCreator, a.loc, a.o11y)},
		agentwf.KindTool{Kind: intent.KindBudgetDetails, Tool: tools.NewBudgetDetails(a.recorder, a.monthlySummary, a.loc, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	categoriesWorkflow, err := agentwf.NewIntentWorkflow("categories",
		agentwf.KindTool{Kind: intent.KindListCategories, Tool: tools.NewListCategories(a.recorder, a.categoryLister, a.o11y)},
		agentwf.KindTool{Kind: intent.KindClassifyCategory, Tool: tools.NewClassifyCategory(a.recorder, a.categoryClassifier, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	cardsWorkflow, err := agentwf.NewIntentWorkflow("cards",
		agentwf.KindTool{Kind: intent.KindListCards, Tool: tools.NewListCards(a.recorder, a.cardLister, a.o11y)},
		agentwf.KindTool{Kind: intent.KindCreateCard, Tool: tools.NewCreateCard(a.recorder, a.cardCreator, a.o11y)},
		agentwf.KindTool{Kind: intent.KindCountCards, Tool: tools.NewCountCards(a.recorder, a.cardCounter, a.o11y)},
		agentwf.KindTool{Kind: intent.KindUpdateCard, Tool: tools.NewUpdateCard(a.recorder, a.clarification, a.cardUpdater, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	conversationalWorkflow, err := agentwf.NewIntentWorkflow("conversational",
		agentwf.KindTool{Kind: intent.KindUnknown, Tool: a.conversational},
	)
	if err != nil {
		return nil, err
	}

	return agentwf.NewIntentRegistry(routableKinds(), transactionsWorkflow, budgetWorkflow, categoriesWorkflow, cardsWorkflow, conversationalWorkflow)
}

func routableKinds() []intent.Kind {
	return []intent.Kind{
		intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
		intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindQueryCategory,
		intent.KindQueryGoal,
		intent.KindQueryCard,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindBudgetRecurrence,
		intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindQueryIncomeSummary,
		intent.KindBudgetDetails,
		intent.KindListCategories,
		intent.KindClassifyCategory,
		intent.KindUnknown,
	}
}

func (a *DailyLedgerAgent) buildKernelDefinition(k *KernelDeps) platform.Definition[steps.ExpenseState] {
	auditBegin := a.defaultAuditBeginFn()
	if k.AuditBeginFn != nil {
		auditBegin = k.AuditBeginFn
	}
	return agentwf.NewTransactionsWriteDefinition(agentwf.TransactionsWriteDeps{
		Authorize: func(ctx context.Context, state steps.ExpenseState) bool {
			principal, ok := auth.FromContext(ctx)
			if !ok {
				return false
			}
			return a.authorizeWrite(ctx, Principal{UserID: principal.UserID}, state.UserID, state.Kind, state.Channel)
		},
		Replay: func(ctx context.Context, state steps.ExpenseState) (string, bool) {
			result, found := a.replayDecision(ctx, state.UserID, state.Channel, state.MessageID, state.StepIndex, state.Kind)
			if !found {
				return "", false
			}
			return result.Reply, true
		},
		Policy: func(ctx context.Context, state steps.ExpenseState) (bool, string) {
			conf, confErr := valueobjects.NewConfidence(state.Confidence)
			if confErr != nil {
				return false, ""
			}
			if a.policy.Evaluate(state.Kind, conf) != domainservices.PolicyDecisionClarify {
				return false, ""
			}
			a.policyBlockedTotal.Add(ctx, 1, observability.String("kind", state.Kind.String()))
			a.o11y.Logger().Warn(ctx, "agent.intent_router.policy_blocked",
				observability.String("kind", state.Kind.String()),
				observability.String("channel", state.Channel),
			)
			a.record(ctx, state.Kind.String(), state.Channel, tools.OutcomePolicyBlocked)
			return true, policyLowConfidenceText
		},
		RetryPolicy: k.RetryPolicy,
		MaxAttempts: k.MaxAttempts,
		AuditBegin:  auditBegin,
		OnSettle: func(id uuid.UUID, fn steps.AuditSettleFunc) {
			if a.settleReg != nil {
				a.settleReg.Register(id, fn)
			}
		},
		Resolver:       k.CategoryResolver,
		Persist:        k.PersistFn,
		DenyReply:      authzDeniedText,
		ReplayReply:    alreadyProcessedText,
		AuditFailReply: auditWriteFailedText,
	})
}

func (a *DailyLedgerAgent) defaultAuditBeginFn() steps.AuditBeginFunc {
	return func(ctx context.Context, state steps.ExpenseState) steps.AuditBeginResult {
		principal := Principal{UserID: state.UserID}
		parsed := ParsedIntent{
			LLMModel:     state.LLMModel,
			PromptSHA256: state.PromptSHA256,
			DirectReply:  state.DirectReply,
			Raw:          []byte(state.RawResponse),
		}
		parsed.StepIndex = state.StepIndex
		auditCtx := a.beginDecisionAudit(ctx, principal, state.Channel, state.MessageID, state.Kind, parsed)
		if auditCtx.conflicted {
			a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", state.Kind.String()))
			a.o11y.Logger().Info(ctx, "agent.intent_router.idempotent_conflict_replay",
				observability.String("kind", state.Kind.String()),
				observability.String("channel", state.Channel),
			)
			a.record(ctx, state.Kind.String(), state.Channel, tools.OutcomeReplay)
			return steps.AuditBeginResult{Conflicted: true}
		}
		if auditCtx.failed {
			a.record(ctx, state.Kind.String(), state.Channel, tools.OutcomeUsecaseError)
			return steps.AuditBeginResult{Failed: true}
		}
		return steps.AuditBeginResult{
			DecisionID: auditCtx.pending.ID(),
			Settle: func(ctx context.Context, executed bool) {
				auditCtx.settle(ctx, executed)
			},
		}
	}
}
