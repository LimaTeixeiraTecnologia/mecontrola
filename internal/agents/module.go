package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	agentapplication "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/agents"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	agentscorers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/scorers"
	agenttools "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/binding"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/infrastructure/persistence"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	agentpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/indexer"
	memorypostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	scorerpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wadispatcher "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	wapayload "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
	workflowpostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow/infrastructure/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions"
)

const EventTypeWhatsAppInbound = "agents.whatsapp.inbound.v1"
const eventTypeSubscriptionBound = "onboarding.subscription_bound"

var errLLMAPIKeyRequired = errors.New("agents.module: llm api_key is required")

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type Module struct {
	HandleInbound      *usecases.HandleInbound
	WhatsAppAgentRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	EventHandlers      []EventHandlerRegistration
	Jobs               []worker.Job
	scorerRunner       scorer.ScorerRunner
}

func (m Module) Shutdown(ctx context.Context) {
	if m.scorerRunner != nil {
		m.scorerRunner.Shutdown(ctx)
	}
}

type LLMConfig struct {
	Model       string
	EmbedModel  string
	APIKey      string
	BaseURL     string
	MaxTokens   int
	Temperature float64
}

type Deps struct {
	DB                 database.DBTX
	O11y               observability.Observability
	OutboxPublisher    outbox.Publisher
	LLM                LLMConfig
	CategoriesModule   *categories.CategoriesModule
	CardModule         card.CardModule
	BudgetsModule      *budgets.BudgetsModule
	TransactionsModule transactions.TransactionsModule
	WhatsAppGateway    whatsAppGateway
	WelcomeDedup       consumers.WelcomeDedupStore
	InboundTimeout     time.Duration
	AgentMaxTokens     int
}

type whatsAppInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

func NewModule(deps Deps) (Module, error) { //nolint:revive // composition root do módulo de agentes; construção sequencial bindings→tools→workflows→runtime é crítica para R-AGENT-WF-001
	if deps.DB == nil {
		return Module{}, fmt.Errorf("agents.module: db is required")
	}
	if deps.O11y == nil {
		return Module{}, fmt.Errorf("agents.module: o11y is required")
	}
	if deps.LLM.APIKey == "" {
		return Module{}, errLLMAPIKeyRequired
	}

	httpClient, err := httpclient.NewClient(deps.O11y,
		httpclient.WithBaseURL(deps.LLM.BaseURL),
		httpclient.WithTarget("openrouter"),
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: http client: %w", err)
	}

	provider := llm.NewOpenRouterProvider(httpClient, llm.Config{
		Model:       deps.LLM.Model,
		EmbedModel:  deps.LLM.EmbedModel,
		APIKey:      deps.LLM.APIKey,
		BaseURL:     deps.LLM.BaseURL,
		MaxTokens:   deps.LLM.MaxTokens,
		Temperature: deps.LLM.Temperature,
	}, deps.O11y)

	scorerResultStore := scorerpostgres.NewResultStore(deps.DB)
	scorerEntries := agentscorers.BuildMeControlaScorers(provider)
	scorerRunner := scorer.NewScorerRunner(scorerEntries, scorerResultStore, deps.O11y)
	scoringHooks := agentapplication.NewScoringHooks(scorerRunner, deps.O11y)

	writeLedgerRepo := persistence.NewWriteLedgerRepository(deps.DB, deps.O11y)
	idempotentWrite := usecases.NewIdempotentWrite(writeLedgerRepo, deps.O11y)
	idemAdapter := idempotentWriterAdapter{uc: idempotentWrite}

	categoriesReader := binding.NewCategoriesReaderAdapter(
		deps.CategoriesModule.SearchDictionaryUC,
		deps.CategoriesModule.ResolveCategoryForWriteUC,
		deps.CategoriesModule.ListCategoriesUC,
		deps.O11y,
	)
	cardManager := binding.NewCardManagerAdapter(
		deps.CardModule.CreateCardUC,
		deps.CardModule.ListCardsUC,
		deps.CardModule.GetCardUC,
		deps.CardModule.ResolveCardByNicknameUC,
		deps.CardModule.CountCardsUC,
		deps.CardModule.BestPurchaseDayUC,
		deps.CardModule.UpdateCardUC,
		deps.CardModule.SoftDeleteCardUC,
		deps.TransactionsModule.HasOpenInstallmentsUC,
		deps.CardModule.IsBankRecognizedUC,
		deps.O11y,
	)
	budgetPlanner := binding.NewBudgetPlannerAdapter(
		deps.BudgetsModule.CreateBudgetUC,
		deps.BudgetsModule.DeleteDraftBudgetUC,
		deps.BudgetsModule.ActivateBudgetUC,
		deps.BudgetsModule.CreateRecurrenceUC,
		deps.BudgetsModule.EditCategoryPercentageUC,
		deps.BudgetsModule.EditBudgetTotalUC,
		deps.BudgetsModule.GetMonthlySummaryUC,
		deps.BudgetsModule.ListAlertsUC,
		deps.BudgetsModule.SuggestAllocationUC,
		deps.O11y,
	)
	txLedger := binding.NewTransactionsLedgerAdapter(
		deps.TransactionsModule.CreateTransactionUC,
		deps.TransactionsModule.UpdateTransactionUC,
		deps.TransactionsModule.DeleteTransactionUC,
		deps.TransactionsModule.ListMonthlyEntriesUC,
		deps.TransactionsModule.GetMonthlySummaryUC,
		deps.TransactionsModule.GetTransactionUC,
		deps.TransactionsModule.GetCardInvoiceUC,
		deps.TransactionsModule.SearchTransactionsUC,
		deps.TransactionsModule.SearchEditCandidatesUC,
		deps.TransactionsModule.CreateRecurringTemplateUC,
		deps.O11y,
	)
	recurrenceManager := binding.NewRecurrenceManagerAdapter(
		deps.TransactionsModule.CreateRecurringTemplateUC,
		deps.TransactionsModule.UpdateRecurringTemplateUC,
		deps.TransactionsModule.DeleteRecurringTemplateUC,
		deps.TransactionsModule.ListRecurringTemplatesUC,
		deps.O11y,
	)
	workflowStore := workflowpostgres.NewPostgresStore(deps.O11y, deps.DB)
	onboardingEngine := workflow.NewEngine[workflows.OnboardingState](workflowStore, deps.O11y)

	threadGateway := memorypostgres.NewThreadRepository(deps.DB, deps.O11y)
	rawMessageStore := memorypostgres.NewMessageRepository(deps.DB, deps.O11y)
	workingMem := memorypostgres.NewWorkingMemoryRepository(deps.DB, deps.O11y)
	semanticRecall := memorypostgres.NewEmbeddingRepository(deps.DB, deps.O11y)

	onboardingAgent := agentapplication.BuildMeControlaAgent(provider, nil, scoringHooks, deps.O11y, deps.AgentMaxTokens)
	budgetManageAgent := agentapplication.BuildMeControlaAgent(provider, nil, scoringHooks, deps.O11y, deps.AgentMaxTokens)

	transactionWriteEngine := workflow.NewEngine[workflows.TransactionWriteState](workflowStore, deps.O11y)
	budgetManageEngine := workflow.NewEngine[workflows.BudgetManageState](workflowStore, deps.O11y)
	cardManageEngine := workflow.NewEngine[workflows.CardManageState](workflowStore, deps.O11y)
	goalEditEngine := workflow.NewEngine[workflows.GoalEditState](workflowStore, deps.O11y)
	destructiveManageEngine := workflow.NewEngine[workflows.DestructiveManageState](workflowStore, deps.O11y)
	treatmentNameEditEngine := workflow.NewEngine[workflows.TreatmentNameEditState](workflowStore, deps.O11y)

	transactionWriteDef := workflows.BuildTransactionWriteWorkflowWithObservability(txLedger, cardManager, categoriesReader, idemAdapter, deps.O11y)
	budgetManageDef := workflows.BuildBudgetManageWorkflow(budgetManageAgent, budgetPlanner)
	cardManageDef := workflows.BuildCardManageWorkflow(cardManager, idemAdapter)
	goalEditDef := workflows.BuildGoalEditWorkflow(workingMem)
	destructiveManageDef := workflows.BuildDestructiveManageWorkflow(cardManager, recurrenceManager, txLedger)
	treatmentNameEditDef := workflows.BuildTreatmentNameEditWorkflow(workingMem, onboardingAgent)

	transactionWriteStarter := usecases.NewTransactionWriteStarter(categoriesReader, txLedger, transactionWriteEngine, transactionWriteDef, deps.O11y)

	financialTools := buildFinancialTools(
		txLedger, cardManager, budgetPlanner, categoriesReader, recurrenceManager, transactionWriteStarter,
		destructiveManageEngine, destructiveManageDef,
		cardManageEngine, cardManageDef,
		budgetManageEngine, budgetManageDef,
		goalEditEngine, goalEditDef,
		treatmentNameEditEngine, treatmentNameEditDef,
	)
	meControlaAgent := agentapplication.BuildMeControlaAgent(provider, financialTools, scoringHooks, deps.O11y, deps.AgentMaxTokens)

	registry := agent.NewAgentRegistry()
	registry.Register(meControlaAgent)

	messageStore := rawMessageStore
	var eventHandlers []EventHandlerRegistration
	if deps.OutboxPublisher != nil {
		indexPublisher := indexer.NewOutboxMessageIndexPublisher(deps.OutboxPublisher)
		messageStore = memory.NewPublishingMessageStore(rawMessageStore, indexPublisher, deps.LLM.EmbedModel, deps.O11y)
		indexHandler := indexer.NewEmbeddingIndexHandler(provider, semanticRecall, deps.O11y)
		eventHandlers = append(eventHandlers, EventHandlerRegistration{
			EventType: memory.EventTypeEmbeddingIndex,
			Handler:   indexHandler,
		})
	}

	onboardingDef := workflows.BuildOnboardingWorkflow(onboardingAgent, cardManager, budgetPlanner, workingMem, threadGateway, messageStore, deps.O11y)

	runStore := agentpostgres.NewRunStore(deps.DB)
	runtime := agent.NewAgentRuntime(registry, threadGateway, messageStore, workingMem, runStore, deps.O11y,
		agent.WithWriteToolSet(
			"register_expense", "register_income", "create_recurrence", "edit_entry",
			"create_card", "update_card",
			"create_budget", "edit_budget_total", "adjust_allocation",
			"edit_goal", "edit_treatment_name",
			"delete_entry", "delete_recurrence", "update_recurrence",
		),
	)
	handleInbound := usecases.NewHandleInbound(runtime, deps.O11y)

	resolveOnboarding := usecases.NewResolveOnboardingOrAgent(onboardingEngine, workflowStore, workingMem, onboardingDef, deps.O11y)

	transactionWriteRegistry := agent.NewWorkflowRegistry[workflows.TransactionWriteState]()
	transactionWriteRegistry.Register(transactionWriteDef)
	budgetManageRegistry := agent.NewWorkflowRegistry[workflows.BudgetManageState]()
	budgetManageRegistry.Register(budgetManageDef)
	cardManageRegistry := agent.NewWorkflowRegistry[workflows.CardManageState]()
	cardManageRegistry.Register(cardManageDef)
	goalEditRegistry := agent.NewWorkflowRegistry[workflows.GoalEditState]()
	goalEditRegistry.Register(goalEditDef)
	destructiveManageRegistry := agent.NewWorkflowRegistry[workflows.DestructiveManageState]()
	destructiveManageRegistry.Register(destructiveManageDef)
	treatmentNameEditRegistry := agent.NewWorkflowRegistry[workflows.TreatmentNameEditState]()
	treatmentNameEditRegistry.Register(treatmentNameEditDef)

	transactionWriteResumer, err := usecases.NewWorkflowResumer(
		workflows.TransactionWriteWorkflowID, transactionWriteRegistry, transactionWriteEngine,
		workflows.TransactionWriteKey, workflows.ContinueTransactionWrite,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: transaction_write resumer: %w", err)
	}
	budgetManageResumer, err := usecases.NewWorkflowResumer(
		workflows.BudgetManageWorkflowID, budgetManageRegistry, budgetManageEngine,
		workflows.BudgetManageKey, workflows.ContinueBudgetManage,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: budget_manage resumer: %w", err)
	}
	cardManageResumer, err := usecases.NewWorkflowResumer(
		workflows.CardManageWorkflowID, cardManageRegistry, cardManageEngine,
		workflows.CardManageKey, workflows.ContinueCardManage,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: card_manage resumer: %w", err)
	}
	goalEditResumer, err := usecases.NewWorkflowResumer(
		workflows.GoalEditWorkflowID, goalEditRegistry, goalEditEngine,
		workflows.GoalEditKey, workflows.ContinueGoalEdit,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: goal_edit resumer: %w", err)
	}
	destructiveManageResumer, err := usecases.NewWorkflowResumer(
		workflows.DestructiveManageWorkflowID, destructiveManageRegistry, destructiveManageEngine,
		workflows.DestructiveManageKey, workflows.ContinueDestructiveManage,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: destructive_manage resumer: %w", err)
	}
	treatmentNameEditResumer, err := usecases.NewWorkflowResumer(
		workflows.TreatmentNameEditWorkflowID, treatmentNameEditRegistry, treatmentNameEditEngine,
		workflows.TreatmentNameEditKey, workflows.ContinueTreatmentNameEdit,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: treatment_name_edit resumer: %w", err)
	}

	suspendedRunIndex := usecases.NewSuspendedRunIndex(
		workflowStore,
		workflows.TransactionWriteWorkflowID,
		workflows.BudgetManageWorkflowID,
		workflows.CardManageWorkflowID,
		workflows.GoalEditWorkflowID,
		workflows.DestructiveManageWorkflowID,
		workflows.TreatmentNameEditWorkflowID,
	)
	resumeDispatcher, err := usecases.NewResumeDispatcher(
		suspendedRunIndex, threadGateway, runStore, deps.O11y,
		transactionWriteResumer, budgetManageResumer, cardManageResumer, goalEditResumer, destructiveManageResumer,
		treatmentNameEditResumer,
	)
	if err != nil {
		return Module{}, fmt.Errorf("agents.module: resume dispatcher: %w", err)
	}

	purgeLedger := usecases.NewPurgeLedger(writeLedgerRepo, 0, 0, deps.O11y)
	ledgerRetentionJob := jobhandlers.NewLedgerRetentionJob(purgeLedger, "")

	transactionWriteReaper := workflows.BuildTransactionWriteReaper(workflowStore, deps.O11y)
	transactionWriteReaperJob := jobhandlers.NewConfirmReaperJob("agents-transaction-write-reaper", transactionWriteReaper, "")
	budgetManageReaper := workflows.BuildBudgetManageReaper(workflowStore, deps.O11y)
	budgetManageReaperJob := jobhandlers.NewConfirmReaperJob("agents-budget-manage-reaper", budgetManageReaper, "")
	cardManageReaper := workflows.BuildCardManageReaper(workflowStore, deps.O11y)
	cardManageReaperJob := jobhandlers.NewConfirmReaperJob("agents-card-manage-reaper", cardManageReaper, "")
	goalEditReaper := workflows.BuildGoalEditReaper(workflowStore, deps.O11y)
	goalEditReaperJob := jobhandlers.NewConfirmReaperJob("agents-goal-edit-reaper", goalEditReaper, "")
	destructiveManageReaper := workflows.BuildDestructiveManageReaper(workflowStore, deps.O11y)
	destructiveManageReaperJob := jobhandlers.NewConfirmReaperJob("agents-destructive-manage-reaper", destructiveManageReaper, "")
	treatmentNameEditReaper := workflows.BuildTreatmentNameEditReaper(workflowStore, deps.O11y)
	treatmentNameEditReaperJob := jobhandlers.NewConfirmReaperJob("agents-treatment-name-edit-reaper", treatmentNameEditReaper, "")
	onboardingReaper := workflows.BuildOnboardingReaper(workflowStore, deps.O11y)
	onboardingReaperJob := jobhandlers.NewConfirmReaperJob("agents-onboarding-reaper", onboardingReaper, "")

	var whatsAppRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	if deps.OutboxPublisher != nil && deps.WhatsAppGateway != nil {
		consumerOpts := []consumers.ConsumerOption{
			consumers.WithOnboardingResolver(resolveOnboarding),
			consumers.WithResumeDispatcher(resumeDispatcher),
		}
		if deps.InboundTimeout > 0 {
			consumerOpts = append(consumerOpts, consumers.WithInboundTimeout(deps.InboundTimeout))
		}
		inboundConsumer := consumers.NewWhatsAppInboundConsumer(
			handleInbound,
			deps.WhatsAppGateway,
			deps.O11y,
			consumerOpts...,
		)
		eventHandlers = append(eventHandlers, EventHandlerRegistration{
			EventType: EventTypeWhatsAppInbound,
			Handler:   inboundConsumer,
		})
		whatsAppRoute = buildWhatsAppAgentRoute(deps.OutboxPublisher, deps.O11y)

		if deps.WelcomeDedup != nil {
			welcomeConsumer := consumers.NewSubscriptionBoundWelcomeConsumer(
				resolveOnboarding,
				deps.WelcomeDedup,
				deps.WhatsAppGateway,
				deps.O11y,
			)
			eventHandlers = append(eventHandlers, EventHandlerRegistration{
				EventType: eventTypeSubscriptionBound,
				Handler:   welcomeConsumer,
			})
		}
	}

	return Module{
		HandleInbound:      handleInbound,
		WhatsAppAgentRoute: whatsAppRoute,
		EventHandlers:      eventHandlers,
		Jobs: []worker.Job{
			ledgerRetentionJob,
			transactionWriteReaperJob, budgetManageReaperJob, cardManageReaperJob, goalEditReaperJob, destructiveManageReaperJob,
			treatmentNameEditReaperJob,
			onboardingReaperJob,
		},
		scorerRunner: scorerRunner,
	}, nil
}

func buildFinancialTools(
	ledger interfaces.TransactionsLedger,
	cards interfaces.CardManager,
	planner interfaces.BudgetPlanner,
	reader interfaces.CategoriesReader,
	recurrences interfaces.RecurrenceManager,
	transactionWriteStarter *usecases.TransactionWriteStarter,
	destructiveManageEngine workflow.Engine[workflows.DestructiveManageState],
	destructiveManageDef workflow.Definition[workflows.DestructiveManageState],
	cardManageEngine workflow.Engine[workflows.CardManageState],
	cardManageDef workflow.Definition[workflows.CardManageState],
	budgetManageEngine workflow.Engine[workflows.BudgetManageState],
	budgetManageDef workflow.Definition[workflows.BudgetManageState],
	goalEditEngine workflow.Engine[workflows.GoalEditState],
	goalEditDef workflow.Definition[workflows.GoalEditState],
	treatmentNameEditEngine workflow.Engine[workflows.TreatmentNameEditState],
	treatmentNameEditDef workflow.Definition[workflows.TreatmentNameEditState],
) []tool.ToolHandle {
	return []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(transactionWriteStarter, cards),
		agenttools.BuildRegisterIncomeTool(transactionWriteStarter),
		agenttools.BuildQueryMonthTool(ledger),
		agenttools.BuildQueryPlanTool(planner),
		agenttools.BuildEditEntryTool(transactionWriteStarter),
		agenttools.BuildDeleteEntryTool(destructiveManageEngine, destructiveManageDef, cards),
		agenttools.BuildAdjustAllocationTool(budgetManageEngine, budgetManageDef),
		agenttools.BuildClassifyCategoryTool(reader),
		agenttools.BuildUpdateRecurrenceTool(destructiveManageEngine, destructiveManageDef),
		agenttools.BuildDeleteRecurrenceTool(destructiveManageEngine, destructiveManageDef),
		agenttools.BuildUpdateCardTool(cardManageEngine, cardManageDef),
		agenttools.BuildListCardsTool(cards),
		agenttools.BuildGetCardTool(cards),
		agenttools.BuildResolveCardTool(cards),
		agenttools.BuildCountCardsTool(cards),
		agenttools.BuildBestPurchaseDayTool(cards),
		agenttools.BuildQueryCardInvoiceTool(ledger, cards),
		agenttools.BuildGetTransactionTool(ledger),
		agenttools.BuildSearchTransactionsTool(ledger),
		agenttools.BuildListRecurrencesTool(recurrences),
		agenttools.BuildCreateRecurrenceTool(transactionWriteStarter, cards),
		agenttools.BuildSuggestAllocationTool(planner),
		agenttools.BuildListCategoriesTool(reader),
		agenttools.BuildCreateCardTool(cardManageEngine, cardManageDef, cards),
		agenttools.BuildCreateBudgetTool(budgetManageEngine, budgetManageDef),
		agenttools.BuildEditBudgetTotalTool(budgetManageEngine, budgetManageDef),
		agenttools.BuildCategoryDetailTool(planner, ledger, reader),
		agenttools.BuildCancelPlanInfoTool(),
		agenttools.BuildSupportInfoTool(),
		agenttools.BuildEditGoalTool(goalEditEngine, goalEditDef),
		agenttools.BuildEditTreatmentNameTool(treatmentNameEditEngine, treatmentNameEditDef),
	}
}

func buildWhatsAppAgentRoute(publisher outbox.Publisher, o11y observability.Observability) func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
	routeTotal := o11y.Metrics().Counter(
		"agents_whatsapp_route_total",
		"Total de mensagens roteadas para o agente via WhatsApp",
		"1",
	)
	tsFailback := o11y.Metrics().Counter(
		"agents_whatsapp_timestamp_fallback_total",
		"Total de mensagens com timestamp ausente ou invalido que usaram fallback para time.Now",
		"1",
	)
	return func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome {
		ctx, span := o11y.Tracer().Start(ctx, "agents.route.whatsapp_inbound")
		defer span.End()

		principal, ok := auth.FromContext(ctx)
		if !ok {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: principal ausente no contexto")
			routeTotal.Add(ctx, 1, observability.String("outcome", "no_principal"))
			return wadispatcher.OutcomeInvalid
		}

		occurredAt, tsOK := wapayload.ParseEpochTimestamp(msg.Timestamp)
		if !tsOK {
			occurredAt = time.Now().UTC()
			tsFailback.Add(ctx, 1)
			o11y.Logger().Warn(ctx, "agents.route.whatsapp_inbound: timestamp ausente ou invalido; usando now",
				observability.String("wamid", msg.WAMID),
			)
		}

		p := whatsAppInboundPayload{
			UserID:    principal.UserID.String(),
			Peer:      msg.From,
			Text:      msg.Text,
			MessageID: msg.WAMID,
		}

		raw, err := json.Marshal(p)
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: marshal payload", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		eventID, err := uuid.NewV7()
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: gerar event id", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		evt, err := outbox.NewEvent(outbox.EventInput{
			ID:              eventID.String(),
			Type:            EventTypeWhatsAppInbound,
			AggregateType:   "whatsapp.message",
			AggregateID:     msg.WAMID,
			AggregateUserID: principal.UserID.String(),
			Payload:         raw,
			OccurredAt:      occurredAt,
		})
		if err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: criar evento", observability.Error(err))
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		if err := publisher.Publish(ctx, evt); err != nil {
			o11y.Logger().Error(ctx, "agents.route.whatsapp_inbound: publicar evento", observability.Error(err))
			span.RecordError(err)
			routeTotal.Add(ctx, 1, observability.String("outcome", "error"))
			return wadispatcher.OutcomeInvalid
		}

		routeTotal.Add(ctx, 1, observability.String("outcome", "routed"))
		return wadispatcher.OutcomeAgent
	}
}

type idempotentWriterAdapter struct {
	uc *usecases.IdempotentWrite
}

func (a idempotentWriterAdapter) Execute(
	ctx context.Context,
	userID uuid.UUID,
	wamid string,
	itemSeq int,
	operation string,
	resourceKind string,
	write workflows.IdempotentWriteFn,
	isDomainErr workflows.DomainErrorClassifier,
) (uuid.UUID, agent.ToolOutcome, error) {
	res, err := a.uc.Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, usecases.WriteFn(write), isDomainErr)
	return res.ResourceID, res.Outcome, err
}
