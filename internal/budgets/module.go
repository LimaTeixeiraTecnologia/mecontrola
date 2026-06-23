package budgets

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	budgetsconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
	budgetsserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	budgetsjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type BudgetsEventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type BudgetsModule struct {
	BudgetsRouter               *budgetsserver.BudgetsRouter
	AbandonedDraftReaper        *budgetsjobs.AbandonedDraftReaper
	PendingEventsReaper         *budgetsjobs.PendingEventsReaper
	RetentionPurge              *budgetsjobs.RetentionPurge
	ThresholdAlertsJob          *budgetsjobs.ThresholdAlertsJob
	ExpenseCommittedConsumer    *consumers.ExpenseCommittedConsumer
	ExternalExpenseConsumer     *consumers.ExternalExpenseConsumer
	ThresholdAlertNotifier      *consumers.ThresholdAlertNotifier
	OnboardingBudgetConsumer    *consumers.OnboardingBudgetConsumer
	TransactionCreatedConsumer  *consumers.TransactionCreatedConsumer
	TransactionDeletedConsumer  *consumers.TransactionDeletedConsumer
	CardPurchaseCreatedConsumer *consumers.CardPurchaseCreatedConsumer
	EventHandlers               []BudgetsEventHandlerRegistration
	CreateBudgetUC              *usecases.CreateBudget
	ActivateBudgetUC            *usecases.ActivateBudget
	CreateRecurrenceUC          *usecases.CreateRecurrence
	DeleteDraftBudgetUC         *usecases.DeleteDraftBudget
	DeleteExpenseUC             *usecases.DeleteExpense
	ListAlertsUC                *usecases.ListAlerts
	GetMonthlySummaryUC         *usecases.GetMonthlySummary
	UpsertExpenseUC             *usecases.UpsertExpense
	EditCategoryPercentageUC    *usecases.EditCategoryPercentage
}

type moduleBuilder struct {
	cfg                      *configs.Config
	o11y                     observability.Observability
	db                       *sqlx.DB
	categoriesModule         *categories.CategoriesModule
	publisher                *producers.ExpenseCommittedPublisher
	budgetActivatedPublisher *producers.BudgetActivatedPublisher
	thresholdAlertPublisher  *producers.ThresholdAlertPublisher
	gatewayAuth              func(http.Handler) http.Handler
	channelGateway           notification.ChannelGateway
	channelResolver          appinterfaces.UserChannelResolver
}

type moduleRepositories struct {
	factory appinterfaces.RepositoryFactory
}

type moduleUseCases struct {
	signalAbandonedDrafts   *usecases.SignalAbandonedDrafts
	runPendingEventsReaper  *usecases.RunPendingEventsReaper
	purgeRetention          *usecases.PurgeRetention
	createBudget            *usecases.CreateBudget
	activateBudget          *usecases.ActivateBudget
	deleteDraftBudget       *usecases.DeleteDraftBudget
	createRecurrence        *usecases.CreateRecurrence
	upsertExpense           *usecases.UpsertExpense
	deleteExpense           *usecases.DeleteExpense
	getMonthlySummary       *usecases.GetMonthlySummary
	editCategoryPercentage  *usecases.EditCategoryPercentage
	listAlerts              *usecases.ListAlerts
	evaluateAlert           *usecases.EvaluateAlert
	ingestExternalExpense   *usecases.IngestExternalExpense
	evaluateThresholdAlerts *usecases.EvaluateThresholdAlerts
}

func NewBudgetsModule(
	cfg *configs.Config,
	o11y observability.Observability,
	db *sqlx.DB,
	categoriesModule *categories.CategoriesModule,
	gatewayAuth func(http.Handler) http.Handler,
	channelGateway notification.ChannelGateway,
	channelResolver appinterfaces.UserChannelResolver,
) (*BudgetsModule, error) {
	outboxFactory := outbox.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()
	builder := moduleBuilder{
		cfg:                      cfg,
		o11y:                     o11y,
		db:                       db,
		categoriesModule:         categoriesModule,
		publisher:                producers.NewExpenseCommittedPublisher(outboxFactory, cfg.OutboxConfig, idGen, o11y),
		budgetActivatedPublisher: producers.NewBudgetActivatedPublisher(outboxFactory, cfg.OutboxConfig, idGen, o11y),
		thresholdAlertPublisher:  producers.NewThresholdAlertPublisher(outboxFactory, cfg.OutboxConfig, idGen, o11y),
		gatewayAuth:              gatewayAuth,
		channelGateway:           channelGateway,
		channelResolver:          channelResolver,
	}
	return builder.Build()
}

func (b *moduleBuilder) Build() (*BudgetsModule, error) {
	repositories := b.buildRepositories()
	categoriesCache, err := b.buildCategoriesCache()
	if err != nil {
		return nil, err
	}
	useCases, err := b.buildUseCases(repositories, categoriesCache)
	if err != nil {
		return nil, err
	}

	expenseCommittedConsumer := consumers.NewExpenseCommittedConsumer(useCases.evaluateAlert, b.o11y)
	externalExpenseConsumer := consumers.NewExternalExpenseConsumer(useCases.ingestExternalExpense, b.o11y)
	onboardingBudgetConsumer := consumers.NewOnboardingBudgetConsumer(useCases.createBudget, useCases.activateBudget, b.o11y)
	transactionCreatedConsumer := consumers.NewTransactionCreatedConsumer(useCases.upsertExpense, b.o11y)
	transactionDeletedConsumer := consumers.NewTransactionDeletedConsumer(useCases.deleteExpense, b.o11y)
	cardPurchaseCreatedConsumer := consumers.NewCardPurchaseCreatedConsumer(useCases.upsertExpense, b.o11y)

	mode := strings.ToLower(strings.TrimSpace(b.cfg.BudgetsConfig.ThresholdAlertsMode))
	if mode == "" {
		mode = configs.ThresholdAlertsModeLegacy
	}
	legacyEnabled := mode == configs.ThresholdAlertsModeLegacy || mode == configs.ThresholdAlertsModeBoth
	jobEnabled := mode == configs.ThresholdAlertsModeJob || mode == configs.ThresholdAlertsModeBoth

	eventHandlers := []BudgetsEventHandlerRegistration{
		{EventType: "external.expense.v1", Handler: externalExpenseConsumer},
		{EventType: "onboarding.splits_calculated", Handler: onboardingBudgetConsumer},
		{EventType: "transactions.transaction.created.v1", Handler: transactionCreatedConsumer},
		{EventType: "transactions.transaction.deleted.v1", Handler: transactionDeletedConsumer},
		{EventType: "transactions.card_purchase.created.v1", Handler: cardPurchaseCreatedConsumer},
	}
	if legacyEnabled {
		eventHandlers = append([]BudgetsEventHandlerRegistration{
			{EventType: "budgets.expense.committed.v1", Handler: expenseCommittedConsumer},
		}, eventHandlers...)
	}

	var thresholdAlertNotifier *consumers.ThresholdAlertNotifier
	if b.channelGateway != nil && b.channelResolver != nil {
		notifyAlertUC := usecases.NewNotifyThresholdAlert(repositories.factory.ThresholdAlertSentRepository(b.db), b.channelResolver, b.channelGateway, b.o11y)
		thresholdAlertNotifier = consumers.NewThresholdAlertNotifier(notifyAlertUC, b.o11y)
		eventHandlers = append(eventHandlers, BudgetsEventHandlerRegistration{
			EventType: "budgets.threshold_alert_triggered.v1",
			Handler:   thresholdAlertNotifier,
		})
	}

	var thresholdAlertsJob *budgetsjobs.ThresholdAlertsJob
	if jobEnabled {
		thresholdAlertsJob = budgetsjobs.NewThresholdAlertsJob(useCases.evaluateThresholdAlerts, b.cfg.BudgetsConfig)
	}

	return &BudgetsModule{
		BudgetsRouter:               b.buildRouter(useCases),
		AbandonedDraftReaper:        budgetsjobs.NewAbandonedDraftReaper(useCases.signalAbandonedDrafts, b.cfg.BudgetsConfig),
		PendingEventsReaper:         budgetsjobs.NewPendingEventsReaper(useCases.runPendingEventsReaper, b.cfg.BudgetsConfig),
		RetentionPurge:              budgetsjobs.NewRetentionPurge(useCases.purgeRetention, b.cfg.BudgetsConfig),
		ThresholdAlertsJob:          thresholdAlertsJob,
		ExpenseCommittedConsumer:    expenseCommittedConsumer,
		ExternalExpenseConsumer:     externalExpenseConsumer,
		ThresholdAlertNotifier:      thresholdAlertNotifier,
		OnboardingBudgetConsumer:    onboardingBudgetConsumer,
		TransactionCreatedConsumer:  transactionCreatedConsumer,
		TransactionDeletedConsumer:  transactionDeletedConsumer,
		CardPurchaseCreatedConsumer: cardPurchaseCreatedConsumer,
		EventHandlers:               eventHandlers,
		CreateBudgetUC:              useCases.createBudget,
		ActivateBudgetUC:            useCases.activateBudget,
		CreateRecurrenceUC:          useCases.createRecurrence,
		DeleteDraftBudgetUC:         useCases.deleteDraftBudget,
		DeleteExpenseUC:             useCases.deleteExpense,
		ListAlertsUC:                useCases.listAlerts,
		GetMonthlySummaryUC:         useCases.getMonthlySummary,
		UpsertExpenseUC:             useCases.upsertExpense,
		EditCategoryPercentageUC:    useCases.editCategoryPercentage,
	}, nil
}

func (b *moduleBuilder) buildRepositories() moduleRepositories {
	return moduleRepositories{
		factory: repositories.NewRepositoryFactory(b.o11y),
	}
}

func (b *moduleBuilder) buildCategoriesCache() (*budgetsconfig.CategoriesCache, error) {
	categoriesReader := postgres.NewCategoriesReaderAdapter(
		b.categoriesModule.ResolveBySlug,
		b.categoriesModule.ValidateSubcategory,
		b.categoriesModule.VersionReader,
		b.o11y,
	)
	categoriesCache := budgetsconfig.NewCategoriesCache(categoriesReader)
	if err := categoriesCache.Boot(context.Background()); err != nil {
		return nil, fmt.Errorf("budgets: resolver raízes editoriais no boot: %w", err)
	}
	return categoriesCache, nil
}

func (b *moduleBuilder) buildUseCases(repositories moduleRepositories, categoriesCache *budgetsconfig.CategoriesCache) (moduleUseCases, error) {
	location, err := b.resolveLocation()
	if err != nil {
		return moduleUseCases{}, err
	}

	budgetUoW := uow.NewUnitOfWork(b.db)
	editCategoryUoW := uow.NewUnitOfWork(b.db)
	expenseUoW := uow.NewUnitOfWork(b.db)
	voidUoW := uow.NewUnitOfWork(b.db)
	listAlertsUoW := uow.NewUnitOfWork(b.db)
	monthlySummaryUoW := uow.NewUnitOfWork(b.db)

	thresholdConfig, err := b.buildThresholdConfig()
	if err != nil {
		return moduleUseCases{}, err
	}

	thresholdAlertsUoW := uow.NewUnitOfWork(b.db)

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(repositories.factory)
	upsertExpense := usecases.NewUpsertExpense(
		repositories.factory,
		categoriesCache,
		b.publisher,
		autoDraft,
		expenseUoW,
		b.o11y,
		location,
	)
	deleteExpense := usecases.NewDeleteExpense(repositories.factory, b.publisher, voidUoW, b.o11y, location)
	applyPending := usecases.NewApplyPendingEvent(repositories.factory, upsertExpense, deleteExpense, b.pendingTTL(), b.o11y)

	return moduleUseCases{
		signalAbandonedDrafts:  usecases.NewSignalAbandonedDrafts(repositories.factory, voidUoW, location, b.o11y),
		runPendingEventsReaper: usecases.NewRunPendingEventsReaper(repositories.factory, applyPending, voidUoW, b.o11y),
		purgeRetention:         usecases.NewPurgeRetention(repositories.factory, voidUoW, b.retentionBatchSize(), b.o11y),
		createBudget:           usecases.NewCreateBudget(repositories.factory, budgetUoW, b.o11y),
		activateBudget:         usecases.NewActivateBudget(repositories.factory, b.budgetActivatedPublisher, budgetUoW, b.o11y),
		deleteDraftBudget:      usecases.NewDeleteDraftBudget(repositories.factory, voidUoW, b.o11y),
		createRecurrence:       usecases.NewCreateRecurrence(repositories.factory, voidUoW, b.o11y),
		upsertExpense:          upsertExpense,
		deleteExpense:          deleteExpense,
		getMonthlySummary:      usecases.NewGetMonthlySummary(repositories.factory, monthlySummaryUoW, b.o11y),
		editCategoryPercentage: usecases.NewEditCategoryPercentage(repositories.factory, b.budgetActivatedPublisher, editCategoryUoW, b.o11y),
		listAlerts:             usecases.NewListAlerts(repositories.factory, listAlertsUoW, b.o11y),
		evaluateAlert:          usecases.NewEvaluateAlert(repositories.factory, voidUoW, b.o11y),
		ingestExternalExpense:  usecases.NewIngestExternalExpense(repositories.factory, upsertExpense, deleteExpense, voidUoW, b.o11y),
		evaluateThresholdAlerts: usecases.NewEvaluateThresholdAlerts(
			repositories.factory,
			b.thresholdAlertPublisher,
			thresholdAlertsUoW,
			thresholdConfig,
			location,
			b.cfg.BudgetsConfig.ThresholdAlertsScanLimit,
			b.o11y,
		),
	}, nil
}

func (b *moduleBuilder) buildThresholdConfig() (services.ThresholdConfig, error) {
	category := b.cfg.BudgetsConfig.ThresholdCategoryRatio
	if category <= 0 {
		category = 0.80
	}
	goal := b.cfg.BudgetsConfig.ThresholdGoalRatio
	if goal <= 0 {
		goal = 0.50
	}
	card := b.cfg.BudgetsConfig.ThresholdCardRatio
	if card <= 0 {
		card = 0.85
	}
	catRatio, err := valueobjects.NewThresholdRatio(category)
	if err != nil {
		return services.ThresholdConfig{}, fmt.Errorf("budgets: threshold category: %w", err)
	}
	goalRatio, err := valueobjects.NewThresholdRatio(goal)
	if err != nil {
		return services.ThresholdConfig{}, fmt.Errorf("budgets: threshold goal: %w", err)
	}
	cardRatio, err := valueobjects.NewThresholdRatio(card)
	if err != nil {
		return services.ThresholdConfig{}, fmt.Errorf("budgets: threshold card: %w", err)
	}
	return services.ThresholdConfig{Category: catRatio, Goal: goalRatio, Card: cardRatio}, nil
}

func (b *moduleBuilder) buildRouter(useCases moduleUseCases) *budgetsserver.BudgetsRouter {
	createBudgetHandler := handlers.NewCreateBudgetHandler(useCases.createBudget, b.o11y)
	activateBudgetHandler := handlers.NewActivateBudgetHandler(useCases.activateBudget, b.o11y)
	deleteBudgetHandler := handlers.NewDeleteBudgetHandler(useCases.deleteDraftBudget, b.o11y)
	createRecurrenceHandler := handlers.NewCreateRecurrenceHandler(useCases.createRecurrence, b.o11y)
	upsertExpenseHandler := handlers.NewUpsertExpenseHandler(useCases.upsertExpense, b.o11y)
	deleteExpenseHandler := handlers.NewDeleteExpenseHandler(useCases.deleteExpense, b.o11y)
	getMonthlySummaryHandler := handlers.NewGetMonthlySummaryHandler(useCases.getMonthlySummary, b.o11y)
	listAlertsHandler := handlers.NewListAlertsHandler(useCases.listAlerts, b.o11y)

	return budgetsserver.NewBudgetsRouter(
		createBudgetHandler,
		activateBudgetHandler,
		deleteBudgetHandler,
		createRecurrenceHandler,
		upsertExpenseHandler,
		deleteExpenseHandler,
		getMonthlySummaryHandler,
		listAlertsHandler,
		b.gatewayAuth,
	)
}

func (b *moduleBuilder) resolveLocation() (*time.Location, error) {
	location := valueobjects.SaoPauloLocation()
	if location != nil {
		return location, nil
	}

	loadedLocation, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, fmt.Errorf("budgets: carregar timezone: %w", err)
	}
	return loadedLocation, nil
}

func (b *moduleBuilder) pendingTTL() time.Duration {
	pendingTTL := b.cfg.BudgetsConfig.PendingTTL
	if pendingTTL == 0 {
		pendingTTL = time.Duration(b.cfg.BudgetsConfig.PendingTTLHours) * time.Hour
	}
	if pendingTTL == 0 {
		pendingTTL = 24 * time.Hour
	}
	return pendingTTL
}

func (b *moduleBuilder) retentionBatchSize() int {
	batchSize := b.cfg.BudgetsConfig.RetentionPurgeBatchSize
	if batchSize <= 0 {
		return 500
	}
	return batchSize
}
