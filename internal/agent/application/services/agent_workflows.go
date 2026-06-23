package services

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

func mustOutcome(raw string) tools.ToolOutcome {
	outcome, err := tools.ParseOutcome(raw)
	if err != nil {
		return tools.OutcomeUsecaseError
	}
	return outcome
}

func toToolResult(result RouteResult) tools.ToolResult {
	return tools.ToolResult{Reply: result.Reply, Outcome: mustOutcome(result.Outcome), Kind: result.Kind}
}

func toRouteResult(result tools.ToolResult) RouteResult {
	return RouteResult{Reply: result.Reply, Outcome: result.Outcome.String(), Kind: result.Kind}
}

type writeContext struct {
	messageID  string
	parsed     ParsedIntent
	confidence valueobjects.Confidence
}

func (a *DailyLedgerAgent) buildRegistry(text string, wctx writeContext) (*workflow.Registry, error) {
	guard := a.newWriteGuard(wctx)

	listExpenseTool := a.routeTool("record_expense", intent.KindRecordExpense, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeLogExpense(ctx, in.UserID, in.Channel, in.Intent)
	})
	recordIncomeTool := a.routeTool("record_income", intent.KindRecordIncome, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeLogIncome(ctx, in.UserID, in.Channel, in.Intent)
	})
	cardPurchaseTool := a.routeTool("record_card_purchase", intent.KindRecordCardPurchase, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeLogCardPurchase(ctx, in.UserID, in.Channel, in.Intent)
	})
	listTransactionsTool := a.routeTool("list_transactions", intent.KindListTransactions, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeListTransactions(ctx, in.UserID, in.Channel, in.Intent)
	})
	deleteLastTool := a.routeTool("delete_last_transaction", intent.KindDeleteLastTransaction, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeDeleteLastTransaction(ctx, in.UserID, in.Channel)
	})
	editLastTool := a.routeTool("edit_last_transaction", intent.KindEditLastTransaction, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeEditLastTransaction(ctx, in.UserID, in.Channel, in.Intent)
	})
	createRecurringTool := a.routeTool("create_recurring", intent.KindCreateRecurring, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeCreateRecurring(ctx, in.UserID, in.Channel, in.Intent)
	})
	listRecurringTool := a.routeTool("list_recurring", intent.KindListRecurring, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeListRecurring(ctx, in.UserID, in.Channel)
	})

	monthlySummaryTool := a.routeTool("monthly_summary", intent.KindMonthlySummary, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeMonthlySummary(ctx, in.UserID, in.Channel, in.Intent)
	})
	howAmIDoingTool := a.routeTool("how_am_i_doing", intent.KindHowAmIDoing, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeHowAmIDoing(ctx, in.UserID, in.Channel)
	})
	queryCategoryTool := a.routeTool("query_category", intent.KindQueryCategory, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeQueryCategory(ctx, in.UserID, in.Channel, in.Intent)
	})
	queryGoalTool := a.routeTool("query_goal", intent.KindQueryGoal, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeQueryGoal(ctx, in.UserID, in.Channel, in.Intent)
	})
	queryCardTool := a.routeTool("query_card", intent.KindQueryCard, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeQueryCard(ctx, in.UserID, in.Channel, in.Intent)
	})
	configureBudgetTool := a.routeTool("configure_budget", intent.KindConfigureBudget, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeConfigureBudget(ctx, in.UserID, in.Channel, text)
	})
	editCategoryPercentTool := a.routeTool("edit_category_percentage", intent.KindEditCategoryPercentage, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeEditCategoryPercentage(ctx, in.UserID, in.Channel, in.Intent)
	})

	listCardsTool := a.routeTool("list_cards", intent.KindListCards, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeListCards(ctx, in.UserID, in.Channel)
	})
	createCardTool := a.routeTool("create_card", intent.KindCreateCard, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeCreateCard(ctx, in.UserID, in.Channel, in.Intent)
	})
	countCardsTool := a.routeTool("count_cards", intent.KindCountCards, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeCountCards(ctx, in.UserID, in.Channel)
	})
	updateCardTool := a.routeTool("update_card", intent.KindUpdateCard, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeUpdateCard(ctx, in.UserID, in.Channel, in.Intent)
	})
	deleteCardTool := a.routeTool("delete_card", intent.KindDeleteCard, func(ctx context.Context, in tools.ToolInput) RouteResult {
		return a.routeDeleteCard(ctx, in.UserID, in.Channel, in.Intent)
	})

	conversationalTool := a.routeTool("conversational", intent.KindUnknown, func(ctx context.Context, in tools.ToolInput) RouteResult {
		reply := a.delegateFallback(ctx, in.UserID, in.Channel, in.Intent.RawText())
		a.record(ctx, intent.KindUnknown.String(), in.Channel, OutcomeFallback)
		return RouteResult{Reply: reply, Outcome: OutcomeFallback, Kind: intent.KindUnknown}
	})

	transactionsWorkflow, err := workflow.NewWorkflow("transactions", guard,
		workflow.KindTool{Kind: intent.KindRecordExpense, Tool: listExpenseTool},
		workflow.KindTool{Kind: intent.KindRecordIncome, Tool: recordIncomeTool},
		workflow.KindTool{Kind: intent.KindRecordCardPurchase, Tool: cardPurchaseTool},
		workflow.KindTool{Kind: intent.KindListTransactions, Tool: listTransactionsTool},
		workflow.KindTool{Kind: intent.KindDeleteLastTransaction, Tool: deleteLastTool},
		workflow.KindTool{Kind: intent.KindEditLastTransaction, Tool: editLastTool},
		workflow.KindTool{Kind: intent.KindCreateRecurring, Tool: createRecurringTool},
		workflow.KindTool{Kind: intent.KindListRecurring, Tool: listRecurringTool},
	)
	if err != nil {
		return nil, err
	}

	budgetWorkflow, err := workflow.NewWorkflow("budget", guard,
		workflow.KindTool{Kind: intent.KindMonthlySummary, Tool: monthlySummaryTool},
		workflow.KindTool{Kind: intent.KindHowAmIDoing, Tool: howAmIDoingTool},
		workflow.KindTool{Kind: intent.KindQueryCategory, Tool: queryCategoryTool},
		workflow.KindTool{Kind: intent.KindQueryGoal, Tool: queryGoalTool},
		workflow.KindTool{Kind: intent.KindQueryCard, Tool: queryCardTool},
		workflow.KindTool{Kind: intent.KindConfigureBudget, Tool: configureBudgetTool},
		workflow.KindTool{Kind: intent.KindEditCategoryPercentage, Tool: editCategoryPercentTool},
	)
	if err != nil {
		return nil, err
	}

	cardsWorkflow, err := workflow.NewWorkflow("cards", guard,
		workflow.KindTool{Kind: intent.KindListCards, Tool: listCardsTool},
		workflow.KindTool{Kind: intent.KindCreateCard, Tool: createCardTool},
		workflow.KindTool{Kind: intent.KindCountCards, Tool: countCardsTool},
		workflow.KindTool{Kind: intent.KindUpdateCard, Tool: updateCardTool},
		workflow.KindTool{Kind: intent.KindDeleteCard, Tool: deleteCardTool},
	)
	if err != nil {
		return nil, err
	}

	conversationalWorkflow, err := workflow.NewWorkflow("conversational", nil,
		workflow.KindTool{Kind: intent.KindUnknown, Tool: conversationalTool},
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

func (a *DailyLedgerAgent) routeTool(name string, kind intent.Kind, route func(ctx context.Context, in tools.ToolInput) RouteResult) tools.Tool {
	spec := tools.ToolSpec{Name: name, IntentKind: kind, Description: name}
	return tools.NewTool(spec, func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, error) {
		return toToolResult(route(ctx, in)), nil
	})
}

func (a *DailyLedgerAgent) newWriteGuard(wctx writeContext) *workflow.WriteGuard {
	return workflow.NewWriteGuard(workflow.GuardSteps{
		Authorize: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			principal := Principal{UserID: in.UserID}
			if a.authorizeWrite(ctx, principal, in.UserID, in.Intent.Kind(), in.Channel) {
				return tools.ToolResult{}, false
			}
			return tools.ToolResult{Reply: authzDeniedText, Outcome: tools.OutcomeAuthzDenied, Kind: in.Intent.Kind()}, true
		},
		Replay: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			replay, replayed := a.replayDecision(ctx, in.UserID, in.Channel, wctx.messageID, in.Intent.Kind())
			if !replayed {
				return tools.ToolResult{}, false
			}
			return toToolResult(replay), true
		},
		Policy: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool) {
			if a.policy.Evaluate(in.Intent.Kind(), wctx.confidence) != domainservices.PolicyDecisionClarify {
				return tools.ToolResult{}, false
			}
			kind := in.Intent.Kind()
			a.policyBlockedTotal.Add(ctx, 1, observability.String("kind", kind.String()))
			a.o11y.Logger().Warn(ctx, "agent.intent_router.policy_blocked",
				observability.String("kind", kind.String()),
				observability.String("channel", in.Channel),
			)
			a.record(ctx, kind.String(), in.Channel, OutcomePolicyBlocked)
			return tools.ToolResult{Reply: policyLowConfidenceText, Outcome: tools.OutcomePolicyBlocked, Kind: kind}, true
		},
		Audit: func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, workflow.SettleFunc, bool) {
			kind := in.Intent.Kind()
			principal := Principal{UserID: in.UserID}
			auditCtx := a.beginDecisionAudit(ctx, principal, in.Channel, wctx.messageID, kind, wctx.parsed)
			if auditCtx.conflicted {
				a.idempotencyReplayTotal.Add(ctx, 1, observability.String("kind", kind.String()))
				a.o11y.Logger().Info(ctx, "agent.intent_router.idempotent_conflict_replay",
					observability.String("kind", kind.String()),
					observability.String("channel", in.Channel),
				)
				a.record(ctx, kind.String(), in.Channel, OutcomeReplay)
				return tools.ToolResult{Reply: alreadyProcessedText, Outcome: tools.OutcomeReplay, Kind: kind}, nil, true
			}
			if auditCtx.failed {
				a.record(ctx, kind.String(), in.Channel, OutcomeUsecaseError)
				return tools.ToolResult{Reply: auditWriteFailedText, Outcome: tools.OutcomeUsecaseError, Kind: kind}, nil, true
			}
			return tools.ToolResult{}, func(ctx context.Context, executed bool) {
				auditCtx.settle(ctx, executed)
			}, false
		},
	})
}
