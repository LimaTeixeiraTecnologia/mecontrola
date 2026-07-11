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
	confirmEngine := workflow.NewEngine[workflows.ConfirmState](workflowStore, deps.O11y)
	pendingEntryEngine := workflow.NewEngine[workflows.PendingEntryState](workflowStore, deps.O11y)
	cardCreateEngine := workflow.NewEngine[workflows.CardCreateState](workflowStore, deps.O11y)
	budgetCreationEngine := workflow.NewEngine[workflows.BudgetCreationState](workflowStore, deps.O11y)
	pendingEntryDef := workflows.BuildPendingEntryWorkflowWithObservability(txLedger, cardManager, categoriesReader, idemAdapter, deps.O11y)
	registerAttempt := usecases.NewRegisterAttempt(categoriesReader, txLedger, pendingEntryEngine, pendingEntryDef, deps.O11y)

	threadGateway := memorypostgres.NewThreadRepository(deps.DB, deps.O11y)
	rawMessageStore := memorypostgres.NewMessageRepository(deps.DB, deps.O11y)
	workingMem := memorypostgres.NewWorkingMemoryRepository(deps.DB, deps.O11y)
	semanticRecall := memorypostgres.NewEmbeddingRepository(deps.DB, deps.O11y)

	onboardingAgent := agentapplication.BuildMeControlaAgent(provider, nil, scoringHooks, deps.O11y, deps.AgentMaxTokens)
	confirmDef := workflows.BuildDestructiveConfirmWorkflow(txLedger, cardManager, categoriesReader, recurrenceManager)
	cardCreateDef := workflows.BuildCardCreateConfirmWorkflow(idemAdapter, cardManager)
	budgetCreationAgent := agentapplication.BuildMeControlaAgent(provider, nil, scoringHooks, deps.O11y, deps.AgentMaxTokens)
	budgetCreationDef := workflows.BuildBudgetCreationWorkflow(budgetCreationAgent, budgetPlanner)

	financialTools := buildFinancialTools(txLedger, cardManager, budgetPlanner, categoriesReader, recurrenceManager, confirmEngine, confirmDef, registerAttempt, cardCreateEngine, cardCreateDef, budgetCreationEngine, budgetCreationDef)
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

	onboardingDef := workflows.BuildOnboardingWorkflow(onboardingAgent, cardManager, budgetPlanner, workingMem, threadGateway, messageStore)

	runStore := agentpostgres.NewRunStore(deps.DB)
	runtime := agent.NewAgentRuntime(registry, threadGateway, messageStore, workingMem, runStore, deps.O11y,
		agent.WithWriteToolSet("register_expense", "register_income", "create_recurrence", "create_card"),
	)
	handleInbound := usecases.NewHandleInbound(runtime, deps.O11y)

	resolveOnboarding := usecases.NewResolveOnboardingOrAgent(onboardingEngine, workflowStore, workingMem, onboardingDef, deps.O11y)
	continueDestructive := usecases.NewDestructiveConfirmContinuer(confirmEngine, confirmDef, deps.O11y)
	continuePendingEntry := usecases.NewPendingEntryContinuer(pendingEntryEngine, pendingEntryDef, threadGateway, runStore, deps.O11y)
	continueCardCreate := usecases.NewCardCreateConfirmContinuer(cardCreateEngine, cardCreateDef, threadGateway, runStore, deps.O11y)
	continueBudgetCreation := usecases.NewBudgetCreationContinuer(budgetCreationEngine, budgetCreationDef, threadGateway, runStore, deps.O11y)

	purgeLedger := usecases.NewPurgeLedger(writeLedgerRepo, 0, 0, deps.O11y)
	ledgerRetentionJob := jobhandlers.NewLedgerRetentionJob(purgeLedger, "")

	confirmReaper := workflow.NewStaleSuspendedReaper(workflowStore, workflows.DestructiveConfirmWorkflowID, 10*time.Minute, 100, deps.O11y)
	confirmReaperJob := jobhandlers.NewConfirmReaperJob("agents-confirm-reaper", confirmReaper, "")
	pendingEntryReaper := workflows.BuildPendingEntryReaper(workflowStore, deps.O11y)
	pendingEntryReaperJob := jobhandlers.NewConfirmReaperJob("agents-pending-entry-reaper", pendingEntryReaper, "")
	cardCreateReaper := workflow.NewStaleSuspendedReaper(workflowStore, workflows.CardCreateConfirmWorkflowID, 15*time.Minute, 100, deps.O11y)
	cardCreateReaperJob := jobhandlers.NewCardCreateReaperJob("agents-card-create-reaper", cardCreateReaper, "")
	budgetCreationReaper := workflows.BuildBudgetCreationReaper(workflowStore, deps.O11y)
	budgetCreationReaperJob := jobhandlers.NewConfirmReaperJob("agents-budget-creation-reaper", budgetCreationReaper, "")
	onboardingReaper := workflows.BuildOnboardingReaper(workflowStore, deps.O11y)
	onboardingReaperJob := jobhandlers.NewConfirmReaperJob("agents-onboarding-reaper", onboardingReaper, "")

	var whatsAppRoute func(ctx context.Context, msg wapayload.Message) wadispatcher.RouteOutcome
	if deps.OutboxPublisher != nil && deps.WhatsAppGateway != nil {
		consumerOpts := []consumers.ConsumerOption{
			consumers.WithPendingEntryContinuer(continuePendingEntry),
			consumers.WithOnboardingResolver(resolveOnboarding),
			consumers.WithDestructiveConfirmResolver(continueDestructive),
			consumers.WithCardCreateResolver(continueCardCreate),
			consumers.WithBudgetCreationResolver(continueBudgetCreation),
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
		Jobs:               []worker.Job{ledgerRetentionJob, confirmReaperJob, pendingEntryReaperJob, cardCreateReaperJob, budgetCreationReaperJob, onboardingReaperJob},
		scorerRunner:       scorerRunner,
	}, nil
}

func buildFinancialTools(
	ledger interfaces.TransactionsLedger,
	cards interfaces.CardManager,
	planner interfaces.BudgetPlanner,
	reader interfaces.CategoriesReader,
	recurrences interfaces.RecurrenceManager,
	confirmEngine workflow.Engine[workflows.ConfirmState],
	confirmDef workflow.Definition[workflows.ConfirmState],
	registerAttempt *usecases.RegisterAttempt,
	cardCreateEngine workflow.Engine[workflows.CardCreateState],
	cardCreateDef workflow.Definition[workflows.CardCreateState],
	budgetCreationEngine workflow.Engine[workflows.BudgetCreationState],
	budgetCreationDef workflow.Definition[workflows.BudgetCreationState],
) []tool.ToolHandle {
	return []tool.ToolHandle{
		agenttools.BuildRegisterExpenseTool(registerAttempt, cards),
		agenttools.BuildRegisterIncomeTool(registerAttempt),
		agenttools.BuildQueryMonthTool(ledger),
		agenttools.BuildQueryPlanTool(planner),
		agenttools.BuildEditEntryTool(registerAttempt),
		agenttools.BuildDeleteEntryTool(confirmEngine, confirmDef, cards),
		agenttools.BuildAdjustAllocationTool(planner),
		agenttools.BuildClassifyCategoryTool(reader),
		agenttools.BuildUpdateRecurrenceTool(confirmEngine, confirmDef),
		agenttools.BuildDeleteRecurrenceTool(confirmEngine, confirmDef),
		agenttools.BuildUpdateCardTool(confirmEngine, confirmDef, cards),
		agenttools.BuildListCardsTool(cards),
		agenttools.BuildGetCardTool(cards),
		agenttools.BuildResolveCardTool(cards),
		agenttools.BuildCountCardsTool(cards),
		agenttools.BuildBestPurchaseDayTool(cards),
		agenttools.BuildQueryCardInvoiceTool(ledger, cards),
		agenttools.BuildGetTransactionTool(ledger),
		agenttools.BuildSearchTransactionsTool(ledger),
		agenttools.BuildListRecurrencesTool(recurrences),
		agenttools.BuildCreateRecurrenceTool(registerAttempt, cards),
		agenttools.BuildSuggestAllocationTool(planner),
		agenttools.BuildListCategoriesTool(reader),
		agenttools.BuildCreateCardTool(cardCreateEngine, cardCreateDef, cards),
		agenttools.BuildCreateBudgetTool(budgetCreationEngine, budgetCreationDef),
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
