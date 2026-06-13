package budgets

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type BudgetsEventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type BudgetsModule struct {
	BudgetsRouter            *budgetsserver.BudgetsRouter
	AbandonedDraftReaper     *budgetsjobs.AbandonedDraftReaper
	PendingEventsReaper      *budgetsjobs.PendingEventsReaper
	RetentionPurge           *budgetsjobs.RetentionPurge
	ExpenseCommittedConsumer *consumers.ExpenseCommittedConsumer
	ExternalExpenseConsumer  *consumers.ExternalExpenseConsumer
	EventHandlers            []BudgetsEventHandlerRegistration
}

type moduleBuilder struct {
	cfg              *configs.Config
	o11y             observability.Observability
	mgr              manager.Manager
	categoriesModule *categories.CategoriesModule
	publisher        *producers.ExpenseCommittedPublisher
}

type moduleRepositories struct {
	factory appinterfaces.RepositoryFactory
}

type moduleUseCases struct {
	signalAbandonedDrafts  *usecases.SignalAbandonedDrafts
	runPendingEventsReaper *usecases.RunPendingEventsReaper
	purgeRetention         *usecases.PurgeRetention
	createBudget           *usecases.CreateBudget
	activateBudget         *usecases.ActivateBudget
	deleteDraftBudget      *usecases.DeleteDraftBudget
	createRecurrence       *usecases.CreateRecurrence
	upsertExpense          *usecases.UpsertExpense
	deleteExpense          *usecases.DeleteExpense
	getMonthlySummary      *usecases.GetMonthlySummary
	listAlerts             *usecases.ListAlerts
	evaluateAlert          *usecases.EvaluateAlert
	ingestExternalExpense  *usecases.IngestExternalExpense
}

func NewBudgetsModule(
	cfg *configs.Config,
	o11y observability.Observability,
	mgr manager.Manager,
	categoriesModule *categories.CategoriesModule,
) (*BudgetsModule, error) {
	builder := moduleBuilder{
		cfg:              cfg,
		o11y:             o11y,
		mgr:              mgr,
		categoriesModule: categoriesModule,
		publisher:        producers.NewExpenseCommittedPublisher(outbox.NewRepositoryFactory(o11y), cfg.OutboxConfig, id.NewUUIDGenerator(), o11y),
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

	return &BudgetsModule{
		BudgetsRouter:            b.buildRouter(useCases),
		AbandonedDraftReaper:     budgetsjobs.NewAbandonedDraftReaper(useCases.signalAbandonedDrafts, b.cfg.BudgetsConfig),
		PendingEventsReaper:      budgetsjobs.NewPendingEventsReaper(useCases.runPendingEventsReaper, b.cfg.BudgetsConfig),
		RetentionPurge:           budgetsjobs.NewRetentionPurge(useCases.purgeRetention, b.cfg.BudgetsConfig),
		ExpenseCommittedConsumer: expenseCommittedConsumer,
		ExternalExpenseConsumer:  externalExpenseConsumer,
		EventHandlers: []BudgetsEventHandlerRegistration{
			{EventType: "budgets.expense.committed.v1", Handler: expenseCommittedConsumer},
			{EventType: "external.expense.v1", Handler: externalExpenseConsumer},
		},
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

	budgetUoW := uow.New[entities.Budget](b.mgr, uow.WithObservability(b.o11y))
	expenseUoW := uow.New[entities.Expense](b.mgr, uow.WithObservability(b.o11y))
	voidUoW := uow.NewVoid(b.mgr, uow.WithObservability(b.o11y))
	listAlertsUoW := uow.New[dtooutput.ListAlertsOutput](b.mgr, uow.WithObservability(b.o11y))
	monthlySummaryUoW := uow.New[dtooutput.MonthlySummaryOutput](b.mgr, uow.WithObservability(b.o11y))

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
		getMonthlySummary:      usecases.NewGetMonthlySummary(repositories.factory, monthlySummaryUoW, b.o11y),
		listAlerts:             usecases.NewListAlerts(repositories.factory, listAlertsUoW, b.o11y),
		evaluateAlert:          usecases.NewEvaluateAlert(repositories.factory, voidUoW, b.o11y),
		ingestExternalExpense:  usecases.NewIngestExternalExpense(repositories.factory, upsertExpense, deleteExpense, voidUoW, b.o11y),
	}, nil
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
