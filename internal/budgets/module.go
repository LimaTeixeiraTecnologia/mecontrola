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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/alerts"
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
	BudgetsRouter              *budgetsserver.BudgetsRouter
	AbandonedDraftReaper       *budgetsjobs.AbandonedDraftReaper
	PendingEventsReaper        *budgetsjobs.PendingEventsReaper
	RetentionPurge             *budgetsjobs.RetentionPurge
	ThresholdAlertsJob         *budgetsjobs.ThresholdAlertsJob
	ExpenseCommittedConsumer   *consumers.ExpenseCommittedConsumer
	ThresholdAlertNotifier     *consumers.ThresholdAlertNotifier
	TransactionCreatedConsumer *consumers.TransactionCreatedConsumer
	TransactionUpdatedConsumer *consumers.TransactionUpdatedConsumer
	TransactionDeletedConsumer *consumers.TransactionDeletedConsumer
	EventHandlers              []BudgetsEventHandlerRegistration
	CreateBudgetUC             *usecases.CreateBudget
	ActivateBudgetUC           *usecases.ActivateBudget
	CreateRecurrenceUC         *usecases.CreateRecurrence
	DeleteDraftBudgetUC        *usecases.DeleteDraftBudget
	DeleteExpenseUC            *usecases.DeleteExpense
	ListFutureBudgetsUC        *usecases.ListFutureBudgets
	ListAlertsUC               *usecases.ListAlerts
	GetMonthlySummaryUC        *usecases.GetMonthlySummary
	SyncFutureBudgetsUC        *usecases.SyncFutureBudgets
	UpsertExpenseUC            *usecases.UpsertExpense
	EditCategoryPercentageUC   *usecases.EditCategoryPercentage
	EditBudgetTotalUC          *usecases.EditBudgetTotal
	SuggestAllocationUC        *usecases.SuggestAllocation
}

type moduleBuilder struct {
	cfg                     *configs.Config
	o11y                    observability.Observability
	db                      *sqlx.DB
	categoriesModule        *categories.CategoriesModule
	publisher               *producers.ExpenseCommittedPublisher
	thresholdAlertPublisher *producers.ThresholdAlertPublisher
	gatewayAuth             func(http.Handler) http.Handler
	channelGateway          notification.ChannelGateway
	channelResolver         appinterfaces.UserChannelResolver
	alertContext            appinterfaces.AlertContextRecorder
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
	listFutureBudgets       *usecases.ListFutureBudgets
	getMonthlySummary       *usecases.GetMonthlySummary
	syncFutureBudgets       *usecases.SyncFutureBudgets
	editCategoryPercentage  *usecases.EditCategoryPercentage
	editBudgetTotal         *usecases.EditBudgetTotal
	listAlerts              *usecases.ListAlerts
	evaluateAlert           *usecases.EvaluateAlert
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
	alertContext appinterfaces.AlertContextRecorder,
) (*BudgetsModule, error) {
	outboxFactory := outbox.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()
	builder := moduleBuilder{
		cfg:                     cfg,
		o11y:                    o11y,
		db:                      db,
		categoriesModule:        categoriesModule,
		publisher:               producers.NewExpenseCommittedPublisher(outboxFactory, cfg.OutboxConfig, idGen, o11y),
		thresholdAlertPublisher: producers.NewThresholdAlertPublisher(outboxFactory, cfg.OutboxConfig, idGen, o11y),
		gatewayAuth:             gatewayAuth,
		channelGateway:          channelGateway,
		channelResolver:         channelResolver,
		alertContext:            alertContext,
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
	transactionCreatedConsumer := consumers.NewTransactionCreatedConsumer(useCases.upsertExpense, b.o11y)
	transactionUpdatedConsumer := consumers.NewTransactionUpdatedConsumer(useCases.upsertExpense, b.o11y)
	transactionDeletedConsumer := consumers.NewTransactionDeletedConsumer(useCases.deleteExpense, b.o11y)

	mode := strings.ToLower(strings.TrimSpace(b.cfg.BudgetsConfig.ThresholdAlertsMode))
	if mode == "" {
		mode = configs.ThresholdAlertsModeLegacy
	}
	legacyEnabled := mode == configs.ThresholdAlertsModeLegacy || mode == configs.ThresholdAlertsModeBoth
	jobEnabled := mode == configs.ThresholdAlertsModeJob || mode == configs.ThresholdAlertsModeBoth

	eventHandlers := []BudgetsEventHandlerRegistration{
		{EventType: "transactions.transaction.created.v1", Handler: transactionCreatedConsumer},
		{EventType: "transactions.transaction.updated.v1", Handler: transactionUpdatedConsumer},
		{EventType: "transactions.transaction.deleted.v1", Handler: transactionDeletedConsumer},
	}
	if legacyEnabled {
		eventHandlers = append([]BudgetsEventHandlerRegistration{
			{EventType: "budgets.expense.committed.v1", Handler: expenseCommittedConsumer},
		}, eventHandlers...)
	}

	var thresholdAlertNotifier *consumers.ThresholdAlertNotifier
	if b.channelGateway != nil && b.channelResolver != nil {
		quietStart, quietEnd, quietErr := b.quietHours()
		if quietErr != nil {
			return nil, quietErr
		}
		notifierLocation, locErr := b.resolveLocation()
		if locErr != nil {
			return nil, locErr
		}
		notifyAlertUC := usecases.NewNotifyThresholdAlert(
			repositories.factory.ThresholdAlertSentRepository(b.db),
			b.channelResolver,
			b.channelGateway,
			alerts.NewTemplateCatalog(b.templateCatalogEntries()),
			alerts.NewFallbackTimezoneResolver(notifierLocation),
			alerts.NewDenyAllMarketingConsent(),
			b.alertContext,
			quietStart,
			quietEnd,
			b.o11y,
		)
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
		BudgetsRouter:              b.buildRouter(useCases),
		AbandonedDraftReaper:       budgetsjobs.NewAbandonedDraftReaper(useCases.signalAbandonedDrafts, b.cfg.BudgetsConfig),
		PendingEventsReaper:        budgetsjobs.NewPendingEventsReaper(useCases.runPendingEventsReaper, b.cfg.BudgetsConfig),
		RetentionPurge:             budgetsjobs.NewRetentionPurge(useCases.purgeRetention, b.cfg.BudgetsConfig),
		ThresholdAlertsJob:         thresholdAlertsJob,
		ExpenseCommittedConsumer:   expenseCommittedConsumer,
		ThresholdAlertNotifier:     thresholdAlertNotifier,
		TransactionCreatedConsumer: transactionCreatedConsumer,
		TransactionUpdatedConsumer: transactionUpdatedConsumer,
		TransactionDeletedConsumer: transactionDeletedConsumer,
		EventHandlers:              eventHandlers,
		CreateBudgetUC:             useCases.createBudget,
		ActivateBudgetUC:           useCases.activateBudget,
		CreateRecurrenceUC:         useCases.createRecurrence,
		DeleteDraftBudgetUC:        useCases.deleteDraftBudget,
		DeleteExpenseUC:            useCases.deleteExpense,
		ListFutureBudgetsUC:        useCases.listFutureBudgets,
		ListAlertsUC:               useCases.listAlerts,
		GetMonthlySummaryUC:        useCases.getMonthlySummary,
		SyncFutureBudgetsUC:        useCases.syncFutureBudgets,
		UpsertExpenseUC:            useCases.upsertExpense,
		EditCategoryPercentageUC:   useCases.editCategoryPercentage,
		EditBudgetTotalUC:          useCases.editBudgetTotal,
		SuggestAllocationUC:        usecases.NewSuggestAllocation(),
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
	editBudgetTotalUoW := uow.NewUnitOfWork(b.db)
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
		activateBudget:         usecases.NewActivateBudget(repositories.factory, budgetUoW, b.o11y),
		deleteDraftBudget:      usecases.NewDeleteDraftBudget(repositories.factory, voidUoW, b.o11y),
		createRecurrence:       usecases.NewCreateRecurrence(repositories.factory, voidUoW, b.o11y),
		upsertExpense:          upsertExpense,
		deleteExpense:          deleteExpense,
		listFutureBudgets:      usecases.NewListFutureBudgets(repositories.factory, monthlySummaryUoW, b.o11y),
		getMonthlySummary:      usecases.NewGetMonthlySummary(repositories.factory, monthlySummaryUoW, b.o11y),
		syncFutureBudgets:      usecases.NewSyncFutureBudgets(repositories.factory, voidUoW, b.o11y),
		editCategoryPercentage: usecases.NewEditCategoryPercentage(repositories.factory, editCategoryUoW, b.o11y),
		editBudgetTotal:        usecases.NewEditBudgetTotal(repositories.factory, editBudgetTotalUoW, b.o11y),
		listAlerts:             usecases.NewListAlerts(repositories.factory, listAlertsUoW, b.o11y),
		evaluateAlert:          usecases.NewEvaluateAlert(repositories.factory, voidUoW, b.o11y),
		evaluateThresholdAlerts: usecases.NewEvaluateThresholdAlerts(
			repositories.factory,
			b.thresholdAlertPublisher,
			thresholdAlertsUoW,
			thresholdConfig,
			location,
			b.cfg.BudgetsConfig.ThresholdAlertsScanLimit,
			b.cfg.BudgetsConfig.ThresholdAlertsDryRun,
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
	catRatio, err := valueobjects.NewThresholdRatio(category)
	if err != nil {
		return services.ThresholdConfig{}, fmt.Errorf("budgets: threshold category: %w", err)
	}
	goalRatio, err := valueobjects.NewThresholdRatio(goal)
	if err != nil {
		return services.ThresholdConfig{}, fmt.Errorf("budgets: threshold goal: %w", err)
	}
	return services.ThresholdConfig{Category: catRatio, Goal: goalRatio}, nil
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
	if fallback := b.cfg.BudgetsConfig.ThresholdAlertsTimezoneFallback; fallback != "" {
		location, err := time.LoadLocation(fallback)
		if err != nil {
			return nil, fmt.Errorf("budgets: carregar timezone fallback: %w", err)
		}
		return location, nil
	}

	location := valueobjects.SaoPauloLocation()
	if location != nil {
		return location, nil
	}
	return nil, fmt.Errorf("budgets: timezone fallback não configurado")
}

func (b *moduleBuilder) quietHours() (time.Duration, time.Duration, error) {
	start, err := parseClockDuration(b.cfg.BudgetsConfig.ThresholdAlertsQuietHoursStart)
	if err != nil {
		return 0, 0, fmt.Errorf("budgets: quiet hours start invalido: %w", err)
	}
	end, err := parseClockDuration(b.cfg.BudgetsConfig.ThresholdAlertsQuietHoursEnd)
	if err != nil {
		return 0, 0, fmt.Errorf("budgets: quiet hours end invalido: %w", err)
	}
	return start, end, nil
}

func parseClockDuration(raw string) (time.Duration, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute, nil
}

func (b *moduleBuilder) templateCatalogEntries() []alerts.CatalogEntry {
	approved := splitKindSet(b.cfg.BudgetsConfig.ThresholdTemplatesApprovedKinds)
	marketing := splitKindSet(b.cfg.BudgetsConfig.ThresholdTemplatesMarketingKinds)
	language := b.cfg.BudgetsConfig.ThresholdAlertsLanguageCode

	definitions := []struct {
		kind services.ThresholdAlertKind
		name string
	}{
		{services.ThresholdAlertCategory80, b.cfg.BudgetsConfig.ThresholdTemplateCategory80},
		{services.ThresholdAlertCategory100, b.cfg.BudgetsConfig.ThresholdTemplateCategory100},
		{services.ThresholdAlertBudgetMissingMonthStart, b.cfg.BudgetsConfig.ThresholdTemplateBudgetMissing},
		{services.ThresholdAlertBudgetNotReviewedDay3, b.cfg.BudgetsConfig.ThresholdTemplateBudgetDay3},
	}

	entries := make([]alerts.CatalogEntry, 0, len(definitions))
	for _, definition := range definitions {
		label := definition.kind.String()
		category := services.TemplateCategoryUtility
		if _, ok := marketing[label]; ok {
			category = services.TemplateCategoryMarketing
		}
		_, isApproved := approved[label]
		entries = append(entries, alerts.CatalogEntry{
			Kind:         definition.kind,
			Name:         definition.name,
			LanguageCode: language,
			Category:     category,
			Approved:     isApproved,
		})
	}
	return entries
}

func splitKindSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		kind := strings.TrimSpace(part)
		if kind == "" {
			continue
		}
		out[kind] = struct{}{}
	}
	return out
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
