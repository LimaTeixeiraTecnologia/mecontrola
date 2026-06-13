package transactions

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	dtooutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	transconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/config"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/client"
	txserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
	jobhandlers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories"
	txrepo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/repositories/postgres"
)

type EventHandlerRegistration struct {
	EventType string
	Handler   events.Handler
}

type TransactionsModule struct {
	Router                          *txserver.TransactionsRouter
	MonthlySummaryRecomputeConsumer *consumers.MonthlySummaryRecomputeConsumer
	RecurringMaterializerJob        *jobhandlers.RecurringMaterializerJob
	MonthlySummaryReconcilerJob     *jobhandlers.MonthlySummaryReconcilerJob
	EventHandlers                   []EventHandlerRegistration
}

type transactionsModuleBuilder struct {
	cfg              *configs.Config
	o11y             observability.Observability
	mgr              manager.Manager
	cardModule       card.CardModule
	categoriesModule *categories.CategoriesModule
}

func NewTransactionsModule(
	cfg *configs.Config,
	o11y observability.Observability,
	mgr manager.Manager,
	cardModule card.CardModule,
	categoriesModule *categories.CategoriesModule,
) (TransactionsModule, error) {
	if !cfg.TransactionsConfig.Enabled {
		return TransactionsModule{}, nil
	}

	builder := &transactionsModuleBuilder{
		cfg:              cfg,
		o11y:             o11y,
		mgr:              mgr,
		cardModule:       cardModule,
		categoriesModule: categoriesModule,
	}
	return builder.build()
}

func (b *transactionsModuleBuilder) build() (TransactionsModule, error) { //nolint:revive // wiring de módulo; cada statement é injeção de dependência sem lógica
	brazilLoc, err := b.resolveBrazilLocation()
	if err != nil {
		return TransactionsModule{}, err
	}

	factory := repositories.NewRepositoryFactory(b.o11y)
	outboxFactory := outbox.NewRepositoryFactory(b.o11y)
	idGen := id.NewUUIDGenerator()
	idemStorage := idempotency.NewPostgresStorage(b.mgr)

	categoriesCache, err := b.buildCategoriesCache()
	if err != nil {
		return TransactionsModule{}, err
	}

	cardLookup := client.NewCardLookupAdapter(b.cardModule.CardLookup, b.o11y)

	txPublisher := producers.NewTransactionEventPublisher(outboxFactory, b.cfg.OutboxConfig, b.o11y)
	cpPublisher := producers.NewCardPurchaseEventPublisher(outboxFactory, b.cfg.OutboxConfig, b.o11y)
	rtPublisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, b.cfg.OutboxConfig, b.o11y)

	txWorkflow := services.TransactionWorkflow{}
	cpWorkflow := services.NewCardPurchaseWorkflow()
	recurringWorkflow := services.RecurringWorkflow{}

	db := b.mgr.DBTX(context.Background())

	createTx := usecases.NewCreateTransaction(
		factory,
		uow.New[entities.Transaction](b.mgr, uow.WithObservability(b.o11y)),
		categoriesCache,
		txWorkflow,
		txPublisher,
		b.o11y,
	)
	updateTx := usecases.NewUpdateTransaction(
		factory,
		uow.New[entities.Transaction](b.mgr, uow.WithObservability(b.o11y)),
		categoriesCache,
		txWorkflow,
		txPublisher,
		b.o11y,
	)
	deleteTx := usecases.NewDeleteTransaction(
		factory,
		uow.NewVoid(b.mgr, uow.WithObservability(b.o11y)),
		txPublisher,
		b.o11y,
	)
	getTx := usecases.NewGetTransaction(
		factory,
		uow.New[dtooutput.Transaction](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	listTx := usecases.NewListTransactions(
		factory,
		uow.New[usecases.TransactionPage](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)

	createCP := usecases.NewCreateCardPurchase(
		factory,
		cardLookup,
		categoriesCache,
		&cpWorkflow,
		cpPublisher,
		uow.New[entities.CardPurchase](b.mgr, uow.WithObservability(b.o11y)),
		idGen,
		b.o11y,
	)
	updateCP := usecases.NewUpdateCardPurchase(
		factory,
		categoriesCache,
		&cpWorkflow,
		cpPublisher,
		uow.New[entities.CardPurchase](b.mgr, uow.WithObservability(b.o11y)),
		idGen,
		b.o11y,
	)
	deleteCP := usecases.NewDeleteCardPurchase(
		factory,
		&cpWorkflow,
		cpPublisher,
		uow.New[entities.CardPurchase](b.mgr, uow.WithObservability(b.o11y)),
		idGen,
		b.o11y,
	)
	getCP := usecases.NewGetCardPurchase(
		factory,
		uow.New[dtooutput.CardPurchase](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	listCP := usecases.NewListCardPurchases(
		factory,
		uow.New[usecases.ListCardPurchasesOutput](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	getCI := usecases.NewGetCardInvoice(
		factory,
		uow.New[dtooutput.CardInvoice](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)

	createRT := usecases.NewCreateRecurringTemplate(
		factory,
		uow.New[entities.RecurringTemplate](b.mgr, uow.WithObservability(b.o11y)),
		categoriesCache,
		rtPublisher,
		b.o11y,
	)
	updateRT := usecases.NewUpdateRecurringTemplate(
		factory,
		uow.New[entities.RecurringTemplate](b.mgr, uow.WithObservability(b.o11y)),
		categoriesCache,
		rtPublisher,
		b.o11y,
	)
	deleteRT := usecases.NewDeleteRecurringTemplate(
		factory,
		uow.NewVoid(b.mgr, uow.WithObservability(b.o11y)),
		rtPublisher,
		b.o11y,
	)
	getRT := usecases.NewGetRecurringTemplate(
		factory,
		uow.New[dtooutput.RecurringTemplate](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	listRT := usecases.NewListRecurringTemplates(
		factory,
		uow.New[usecases.RecurringTemplatePage](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)

	recomputeMS := usecases.NewRecomputeMonthlySummary(
		factory,
		uow.New[struct{}](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	lookbackHours := b.cfg.TransactionsConfig.MonthlySummaryReconcilerLookbackHours
	if lookbackHours == 0 {
		lookbackHours = 48
	}
	reconcileMS := usecases.NewReconcileMonthlySummary(db, factory, lookbackHours, b.o11y)
	getMS := usecases.NewGetMonthlySummary(
		factory,
		uow.New[dtooutput.MonthlySummary](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)
	listME := usecases.NewListMonthlyEntries(
		factory,
		uow.New[dtooutput.MonthlyEntriesPage](b.mgr, uow.WithObservability(b.o11y)),
		b.o11y,
	)

	materializeUC := usecases.NewMaterializeRecurringForDay(
		db,
		factory,
		uow.New[struct{}](b.mgr, uow.WithObservability(b.o11y)),
		recurringWorkflow,
		createTx,
		createCP,
		brazilLoc,
		b.o11y,
	)

	idemTTL := b.cfg.TransactionsConfig.IdempotencyTTL
	if idemTTL == 0 {
		idemTTL = 24 * time.Hour
	}

	debounceWindow := b.cfg.TransactionsConfig.MonthlySummaryDebounceWindow
	if debounceWindow == 0 {
		debounceWindow = 1500 * time.Millisecond
	}

	router := txserver.NewTransactionsRouter(
		handlers.NewCreateTransactionHandler(createTx, b.o11y),
		handlers.NewUpdateTransactionHandler(updateTx, b.o11y),
		handlers.NewDeleteTransactionHandler(deleteTx, b.o11y),
		handlers.NewGetTransactionHandler(getTx, b.o11y),
		handlers.NewListTransactionsHandler(listTx, b.o11y),
		handlers.NewCreateCardPurchaseHandler(createCP, b.o11y),
		handlers.NewUpdateCardPurchaseHandler(updateCP, b.o11y),
		handlers.NewDeleteCardPurchaseHandler(deleteCP, b.o11y),
		handlers.NewGetCardPurchaseHandler(getCP, b.o11y),
		handlers.NewListCardPurchasesHandler(listCP, b.o11y),
		handlers.NewGetCardInvoiceHandler(getCI, b.o11y),
		handlers.NewCreateRecurringTemplateHandler(createRT, b.o11y),
		handlers.NewUpdateRecurringTemplateHandler(updateRT, b.o11y),
		handlers.NewDeleteRecurringTemplateHandler(deleteRT, b.o11y),
		handlers.NewGetRecurringTemplateHandler(getRT, b.o11y),
		handlers.NewListRecurringTemplatesHandler(listRT, b.o11y),
		handlers.NewGetMonthlySummaryHandler(getMS, b.o11y),
		handlers.NewListMonthlyEntriesHandler(listME, b.o11y),
		idemStorage,
		idemTTL,
		b.o11y,
	)

	recomputeConsumer := consumers.NewMonthlySummaryRecomputeConsumer(recomputeMS, debounceWindow, b.o11y)

	recurringJob := jobhandlers.NewRecurringMaterializerJob(materializeUC, brazilLoc, b.cfg.TransactionsConfig)
	reconcilerJob := jobhandlers.NewMonthlySummaryReconcilerJob(reconcileMS, b.cfg.TransactionsConfig)

	return TransactionsModule{
		Router:                          router,
		MonthlySummaryRecomputeConsumer: recomputeConsumer,
		RecurringMaterializerJob:        recurringJob,
		MonthlySummaryReconcilerJob:     reconcilerJob,
		EventHandlers: []EventHandlerRegistration{
			{EventType: "transactions.transaction.created.v1", Handler: recomputeConsumer},
			{EventType: "transactions.transaction.updated.v1", Handler: recomputeConsumer},
			{EventType: "transactions.transaction.deleted.v1", Handler: recomputeConsumer},
			{EventType: "transactions.card_purchase.created.v1", Handler: recomputeConsumer},
			{EventType: "transactions.card_purchase.updated.v1", Handler: recomputeConsumer},
			{EventType: "transactions.card_purchase.deleted.v1", Handler: recomputeConsumer},
		},
	}, nil
}

func (b *transactionsModuleBuilder) buildCategoriesCache() (*transconfig.CategoriesCache, error) {
	categoriesReader := txrepo.NewCategoriesReaderAdapter(
		b.categoriesModule.ResolveBySlug,
		b.categoriesModule.ValidateSubcategory,
		b.categoriesModule.VersionReader,
		b.o11y,
	)
	cache := transconfig.NewCategoriesCache(categoriesReader)
	if err := cache.Boot(context.Background()); err != nil {
		return nil, fmt.Errorf("transactions: resolver raízes editoriais no boot: %w", err)
	}
	return cache, nil
}

func (b *transactionsModuleBuilder) resolveBrazilLocation() (*time.Location, error) {
	tz := b.cfg.TransactionsConfig.BrazilTimezone
	if tz == "" {
		tz = "America/Sao_Paulo"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("transactions: carregar timezone %s: %w", tz, err)
	}
	return loc, nil
}
