package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
)

func toRouteResult(result tools.ToolResult) RouteResult {
	return RouteResult{Reply: result.Reply, Outcome: result.Outcome, Kind: result.Kind}
}

func (a *DailyLedgerAgent) buildRegistry() (*workflow.Registry, error) {
	guard := a.newWriteGuard()

	transactionsWorkflow, err := workflow.NewWorkflow("transactions", guard,
		workflow.KindTool{Kind: intent.KindRecordExpense, Tool: tools.NewRecordExpense(a.recorder, a.clarification, a.expenseRecorder, a.o11y)},
		workflow.KindTool{Kind: intent.KindRecordIncome, Tool: tools.NewRecordIncome(a.recorder, a.clarification, a.expenseRecorder, a.o11y)},
		workflow.KindTool{Kind: intent.KindRecordCardPurchase, Tool: tools.NewRecordCardPurchase(a.recorder, a.clarification, a.cardPurchaseLog, a.o11y)},
		workflow.KindTool{Kind: intent.KindListTransactions, Tool: tools.NewListTransactions(a.recorder, a.transactionLister, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindDeleteLastTransaction, Tool: tools.NewDeleteLastTransaction(a.recorder, a.transactionLister, a.lastDeleter, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindEditLastTransaction, Tool: tools.NewEditLastTransaction(a.recorder, a.transactionLister, a.lastEditor, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindCreateRecurring, Tool: tools.NewCreateRecurring(a.recorder, a.clarification, a.recurringCreator, a.o11y)},
		workflow.KindTool{Kind: intent.KindListRecurring, Tool: tools.NewListRecurring(a.recorder, a.recurringLister, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	budgetWorkflow, err := workflow.NewWorkflow("budget", guard,
		workflow.KindTool{Kind: intent.KindMonthlySummary, Tool: tools.NewMonthlySummary(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindHowAmIDoing, Tool: tools.NewHowAmIDoing(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindQueryCategory, Tool: tools.NewQueryCategory(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindQueryGoal, Tool: tools.NewQueryGoal(a.recorder, a.monthlySummary, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindQueryCard, Tool: tools.NewQueryCard(a.recorder, a.cardLister, a.cardInvoice, a.loc, a.o11y)},
		workflow.KindTool{Kind: intent.KindConfigureBudget, Tool: tools.NewConfigureBudget(a.recorder, a.budgetRunner, a.budgetConfig, a.o11y)},
		workflow.KindTool{Kind: intent.KindEditCategoryPercentage, Tool: tools.NewEditCategoryPercentage(a.recorder, a.categoryPercentageEditor, a.loc, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	cardsWorkflow, err := workflow.NewWorkflow("cards", guard,
		workflow.KindTool{Kind: intent.KindListCards, Tool: tools.NewListCards(a.recorder, a.cardLister, a.o11y)},
		workflow.KindTool{Kind: intent.KindCreateCard, Tool: tools.NewCreateCard(a.recorder, a.cardCreator, a.o11y)},
		workflow.KindTool{Kind: intent.KindCountCards, Tool: tools.NewCountCards(a.recorder, a.cardCounter, a.o11y)},
		workflow.KindTool{Kind: intent.KindUpdateCard, Tool: tools.NewUpdateCard(a.recorder, a.clarification, a.cardUpdater, a.o11y)},
		workflow.KindTool{Kind: intent.KindDeleteCard, Tool: tools.NewDeleteCard(a.recorder, a.clarification, a.cardDeleter, a.o11y)},
	)
	if err != nil {
		return nil, err
	}

	conversationalWorkflow, err := workflow.NewWorkflow("conversational", nil,
		workflow.KindTool{Kind: intent.KindUnknown, Tool: a.conversational},
	)
	if err != nil {
		return nil, err
	}

	return workflow.NewRegistry(routableKinds(), transactionsWorkflow, budgetWorkflow, cardsWorkflow, conversationalWorkflow)
}

func routableKinds() []intent.Kind {
	return []intent.Kind{
		intent.KindRecordExpense,
		intent.KindRecordIncome,
		intent.KindRecordCardPurchase,
		intent.KindListTransactions,
		intent.KindDeleteLastTransaction,
		intent.KindEditLastTransaction,
		intent.KindCreateRecurring,
		intent.KindListRecurring,
		intent.KindMonthlySummary,
		intent.KindHowAmIDoing,
		intent.KindQueryCategory,
		intent.KindQueryGoal,
		intent.KindQueryCard,
		intent.KindConfigureBudget,
		intent.KindEditCategoryPercentage,
		intent.KindListCards,
		intent.KindCreateCard,
		intent.KindCountCards,
		intent.KindUpdateCard,
		intent.KindDeleteCard,
		intent.KindUnknown,
	}
}

func (a *DailyLedgerAgent) newWriteGuard() *workflow.WriteGuard {
	return workflow.NewWriteGuard(workflow.GuardSteps{
		Authorize: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			principal := Principal{UserID: in.UserID}
			if a.authorizeWrite(ctx, principal, in.UserID, in.Intent.Kind(), in.Channel) {
				return tools.ToolResult{}, false
			}
			return tools.ToolResult{Reply: authzDeniedText, Outcome: tools.OutcomeAuthzDenied, Kind: in.Intent.Kind()}, true
		},
		Replay: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			replay, replayed := a.replayDecision(ctx, in.UserID, in.Channel, in.MessageID, in.Intent.Kind())
			if !replayed {
				return tools.ToolResult{}, false
			}
			return tools.ToolResult{Reply: replay.Reply, Outcome: replay.Outcome, Kind: replay.Kind}, true
		},
		Policy: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			if a.policy.Evaluate(in.Intent.Kind(), in.Confidence) != domainservices.PolicyDecisionClarify {
				return tools.ToolResult{}, false
			}
			kind := in.Intent.Kind()
			a.policyBlockedTotal.Add(ctx, 1, observability.String("kind", kind.String()))
			a.o11y.Logger().Warn(ctx, "agent.intent_router.policy_blocked",
				observability.String("kind", kind.String()),
				observability.String("channel", in.Channel),
			)
			a.record(ctx, kind.String(), in.Channel, tools.OutcomePolicyBlocked)
			return tools.ToolResult{Reply: policyLowConfidenceText, Outcome: tools.OutcomePolicyBlocked, Kind: kind}, true
		},
		Audit: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, workflow.SettleFunc, bool) {
			kind := in.Intent.Kind()
			principal := Principal{UserID: in.UserID}
			parsed, _ := in.Parsed.(ParsedIntent)
			auditCtx := a.beginDecisionAudit(ctx, principal, in.Channel, in.MessageID, kind, parsed)
			if auditCtx.conflicted {
				a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind.String()))
				a.o11y.Logger().Info(ctx, "agent.intent_router.idempotent_conflict_replay",
					observability.String("kind", kind.String()),
					observability.String("channel", in.Channel),
				)
				a.record(ctx, kind.String(), in.Channel, tools.OutcomeReplay)
				return tools.ToolResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: kind}, nil, true
			}
			if auditCtx.failed {
				a.record(ctx, kind.String(), in.Channel, tools.OutcomeUsecaseError)
				return tools.ToolResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}, nil, true
			}
			return tools.ToolResult{}, func(ctx context.Context, executed bool) {
				auditCtx.settle(ctx, executed)
			}, false
		},
	})
}
