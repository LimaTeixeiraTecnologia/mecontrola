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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	budgetsconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
	budgetsserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	budgetsjobs "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories/postgres"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

const eventTypeExternalExpense = "external.expense.v1"

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

func NewBudgetsModule(
	cfg *configs.Config,
	o11y observability.Observability,
	mgr manager.Manager,
	categoriesModule *categories.CategoriesModule,
) (*BudgetsModule, error) {
	outboxFactory := outbox.NewRepositoryFactory(o11y)
	idGen := id.NewUUIDGenerator()

	budgetRepo := postgres.NewBudgetRepository(o11y)
	expenseRepo := postgres.NewExpenseRepository(o11y)
	alertRepo := postgres.NewAlertRepository(o11y)
	pendingRepo := postgres.NewPendingEventRepository(o11y)
	thresholdStateRepo := postgres.NewThresholdStateRepository(o11y)

	catReader := postgres.NewCategoriesReaderAdapter(
		categoriesModule.ResolveBySlug,
		categoriesModule.ValidateSubcategory,
		categoriesModule.VersionReader,
		o11y,
	)

	catCache := budgetsconfig.NewCategoriesCache(catReader)
	if err := catCache.Boot(context.Background()); err != nil {
		return nil, fmt.Errorf("budgets: resolver raízes editoriais no boot: %w", err)
	}

	publisher := producers.NewExpenseCommittedPublisher(outboxFactory, cfg.OutboxConfig, idGen)

	budgetUoW := uow.New[entities.Budget](mgr, uow.WithObservability(o11y))
	expenseUoW := uow.New[entities.Expense](mgr, uow.WithObservability(o11y))
	voidUoW := uow.NewVoid(mgr, uow.WithObservability(o11y))
	listAlertsUoW := uow.New[dtooutput.ListAlertsOutput](mgr, uow.WithObservability(o11y))
	monthlySummaryUoW := uow.New[dtooutput.MonthlySummaryOutput](mgr, uow.WithObservability(o11y))

	loc := valueobjects.SaoPauloLocation()
	if loc == nil {
		loaded, locErr := time.LoadLocation("America/Sao_Paulo")
		if locErr != nil {
			return nil, fmt.Errorf("budgets: carregar timezone: %w", locErr)
		}
		loc = loaded
	}

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(budgetRepo)

	upsertExpense := usecases.NewUpsertExpense(
		expenseRepo,
		budgetRepo,
		catCache,
		publisher,
		autoDraft,
		expenseUoW,
		o11y,
		loc,
	)

	deleteExpense := usecases.NewDeleteExpense(expenseRepo, publisher, voidUoW, o11y, loc)

	pendingTTL := cfg.BudgetsConfig.PendingTTL
	if pendingTTL == 0 {
		pendingTTL = time.Duration(cfg.BudgetsConfig.PendingTTLHours) * time.Hour
	}
	if pendingTTL == 0 {
		pendingTTL = 24 * time.Hour
	}

	applyPending := usecases.NewApplyPendingEvent(expenseRepo, upsertExpense, deleteExpense, pendingTTL, o11y)

	batchSize := cfg.BudgetsConfig.RetentionPurgeBatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	signalAbandonedDrafts := usecases.NewSignalAbandonedDrafts(budgetRepo, voidUoW, loc, o11y)
	purgeRetention := usecases.NewPurgeRetention(expenseRepo, alertRepo, pendingRepo, voidUoW, batchSize, o11y)
	runPendingEventsReaper := usecases.NewRunPendingEventsReaper(pendingRepo, applyPending, voidUoW, o11y)

	createBudget := usecases.NewCreateBudget(budgetRepo, budgetUoW, o11y)
	activateBudget := usecases.NewActivateBudget(budgetRepo, budgetUoW, o11y)
	deleteDraft := usecases.NewDeleteDraftBudget(budgetRepo, voidUoW, o11y)
	createRecurrence := usecases.NewCreateRecurrence(budgetRepo, voidUoW, o11y)
	getMonthlySummary := usecases.NewGetMonthlySummary(budgetRepo, expenseRepo, monthlySummaryUoW, o11y)
	listAlerts := usecases.NewListAlerts(alertRepo, listAlertsUoW, o11y)
	evaluateAlert := usecases.NewEvaluateAlert(expenseRepo, budgetRepo, thresholdStateRepo, alertRepo, voidUoW, o11y)

	createBudgetHandler := handlers.NewCreateBudgetHandler(createBudget, o11y)
	activateBudgetHandler := handlers.NewActivateBudgetHandler(activateBudget, o11y)
	deleteBudgetHandler := handlers.NewDeleteBudgetHandler(deleteDraft, o11y)
	createRecurrenceHandler := handlers.NewCreateRecurrenceHandler(createRecurrence, o11y)
	upsertExpenseHandler := handlers.NewUpsertExpenseHandler(upsertExpense, o11y)
	deleteExpenseHandler := handlers.NewDeleteExpenseHandler(deleteExpense, o11y)
	getMonthlySummaryHandler := handlers.NewGetMonthlySummaryHandler(getMonthlySummary, o11y)
	listAlertsHandler := handlers.NewListAlertsHandler(listAlerts, o11y)

	budgetsRouter := budgetsserver.NewBudgetsRouter(
		createBudgetHandler,
		activateBudgetHandler,
		deleteBudgetHandler,
		createRecurrenceHandler,
		upsertExpenseHandler,
		deleteExpenseHandler,
		getMonthlySummaryHandler,
		listAlertsHandler,
	)

	ingestExternalExpense := usecases.NewIngestExternalExpense(pendingRepo, upsertExpense, deleteExpense, voidUoW, o11y)

	expenseCommittedConsumer := consumers.NewExpenseCommittedConsumer(evaluateAlert, o11y)
	externalExpenseConsumer := consumers.NewExternalExpenseConsumer(ingestExternalExpense, o11y)

	abandonedDraftReaper := budgetsjobs.NewAbandonedDraftReaper(signalAbandonedDrafts, cfg.BudgetsConfig)
	pendingEventsReaper := budgetsjobs.NewPendingEventsReaper(runPendingEventsReaper, cfg.BudgetsConfig)
	retentionPurge := budgetsjobs.NewRetentionPurge(purgeRetention, cfg.BudgetsConfig)

	return &BudgetsModule{
		BudgetsRouter:            budgetsRouter,
		AbandonedDraftReaper:     abandonedDraftReaper,
		PendingEventsReaper:      pendingEventsReaper,
		RetentionPurge:           retentionPurge,
		ExpenseCommittedConsumer: expenseCommittedConsumer,
		ExternalExpenseConsumer:  externalExpenseConsumer,
		EventHandlers: []BudgetsEventHandlerRegistration{
			{EventType: "budgets.expense.committed.v1", Handler: expenseCommittedConsumer},
			{EventType: eventTypeExternalExpense, Handler: externalExpenseConsumer},
		},
	}, nil
}
