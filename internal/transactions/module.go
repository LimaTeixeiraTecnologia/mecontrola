package transactions

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
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
	ListTransactionsUC              *usecases.ListTransactions
	SearchTransactionsUC            *usecases.SearchTransactions
	CreateTransactionUC             *usecases.CreateTransaction
	UpdateTransactionUC             *usecases.UpdateTransaction
	DeleteTransactionUC             *usecases.DeleteTransaction
	GetTransactionUC                *usecases.GetTransaction
	GetMonthlySummaryUC             *usecases.GetMonthlySummary
	ListMonthlyEntriesUC            *usecases.ListMonthlyEntries
	HasOpenInstallmentsUC           *usecases.HasOpenInstallments
	GetCardInvoiceUC                *usecases.GetCardInvoice
	CreateRecurringTemplateUC       *usecases.CreateRecurringTemplate
	UpdateRecurringTemplateUC       *usecases.UpdateRecurringTemplate
	DeleteRecurringTemplateUC       *usecases.DeleteRecurringTemplate
	ListRecurringTemplatesUC        *usecases.ListRecurringTemplates
	GetCardInvoiceHandler           *handlers.GetCardInvoiceHandler
}

type transactionsModuleBuilder struct {
	cfg              *configs.Config
	o11y             observability.Observability
	db               *sqlx.DB
	cardModule       card.CardModule
	categoriesModule *categories.CategoriesModule
	gatewayAuth      func(http.Handler) http.Handler
}

func NewTransactionsModule(
	cfg *configs.Config,
	o11y observability.Observability,
	db *sqlx.DB,
	cardModule card.CardModule,
	categoriesModule *categories.CategoriesModule,
	gatewayAuth func(http.Handler) http.Handler,
) (TransactionsModule, error) {
	if !cfg.TransactionsConfig.Enabled {
		return TransactionsModule{}, nil
	}

	builder := &transactionsModuleBuilder{
		cfg:              cfg,
		o11y:             o11y,
		db:               db,
		cardModule:       cardModule,
		categoriesModule: categoriesModule,
		gatewayAuth:      gatewayAuth,
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
	idemStorage := idempotency.NewPostgresStorage(b.db)

	categoriesCache, err := b.buildCategoriesCache()
	if err != nil {
		return TransactionsModule{}, err
	}

	cardLookup := client.NewCardLookupAdapter(b.cardModule.CardLookup, b.o11y)

	txPublisher := producers.NewTransactionEventPublisher(outboxFactory, b.cfg.OutboxConfig, b.o11y)

	txWorkflow := services.TransactionWorkflow{}
	recurringWorkflow := services.RecurringWorkflow{}

	db := b.db

	createTx := usecases.NewCreateTransaction(
		factory,
		uow.NewUnitOfWork(b.db),
		cardLookup,
		categoriesCache,
		txWorkflow,
		txPublisher,
		b.o11y,
	)
	updateTx := usecases.NewUpdateTransaction(
		factory,
		uow.NewUnitOfWork(b.db),
		categoriesCache,
		txWorkflow,
		txPublisher,
		b.o11y,
	)
	deleteTx := usecases.NewDeleteTransaction(
		factory,
		uow.NewUnitOfWork(b.db),
		txWorkflow,
		txPublisher,
		b.o11y,
	)
	getTx := usecases.NewGetTransaction(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	listTx := usecases.NewListTransactions(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)

	searchTx := usecases.NewSearchTransactions(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)

	getCI := usecases.NewGetCardInvoice(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	getCardInvoiceHandler := handlers.NewGetCardInvoiceHandler(getCI, b.o11y)

	createRT := usecases.NewCreateRecurringTemplate(
		factory,
		uow.NewUnitOfWork(b.db),
		categoriesCache,
		b.o11y,
	)
	updateRT := usecases.NewUpdateRecurringTemplate(
		factory,
		uow.NewUnitOfWork(b.db),
		categoriesCache,
		b.o11y,
	)
	deleteRT := usecases.NewDeleteRecurringTemplate(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	getRT := usecases.NewGetRecurringTemplate(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	listRT := usecases.NewListRecurringTemplates(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)

	recomputeMS := usecases.NewRecomputeMonthlySummary(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	lookbackHours := b.cfg.TransactionsConfig.MonthlySummaryReconcilerLookbackHours
	if lookbackHours == 0 {
		lookbackHours = 48
	}
	reconcileMS := usecases.NewReconcileMonthlySummary(
		factory.TransactionRepository(db),
		factory.CardInvoiceRepository(db),
		factory.MonthlySummaryRepository(db),
		lookbackHours,
		b.o11y,
	)
	getMS := usecases.NewGetMonthlySummary(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	listME := usecases.NewListMonthlyEntries(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)
	hasOpenInstallments := usecases.NewHasOpenInstallments(
		factory,
		uow.NewUnitOfWork(b.db),
		b.o11y,
	)

	materializeUC := usecases.NewMaterializeRecurringForDay(
		db,
		factory,
		uow.NewUnitOfWork(b.db),
		recurringWorkflow,
		createTx,
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
		b.gatewayAuth,
	)

	recomputeConsumer := consumers.NewMonthlySummaryRecomputeConsumer(recomputeMS, debounceWindow, b.o11y)

	recurringJob := jobhandlers.NewRecurringMaterializerJob(materializeUC, brazilLoc, b.cfg.TransactionsConfig)
	reconcilerJob := jobhandlers.NewMonthlySummaryReconcilerJob(reconcileMS, b.cfg.TransactionsConfig)

	return TransactionsModule{
		Router:                          router,
		MonthlySummaryRecomputeConsumer: recomputeConsumer,
		RecurringMaterializerJob:        recurringJob,
		MonthlySummaryReconcilerJob:     reconcilerJob,
		ListTransactionsUC:              listTx,
		SearchTransactionsUC:            searchTx,
		CreateTransactionUC:             createTx,
		UpdateTransactionUC:             updateTx,
		DeleteTransactionUC:             deleteTx,
		GetTransactionUC:                getTx,
		GetCardInvoiceUC:                getCI,
		UpdateRecurringTemplateUC:       updateRT,
		DeleteRecurringTemplateUC:       deleteRT,
		GetMonthlySummaryUC:             getMS,
		ListMonthlyEntriesUC:            listME,
		HasOpenInstallmentsUC:           hasOpenInstallments,
		CreateRecurringTemplateUC:       createRT,
		ListRecurringTemplatesUC:        listRT,
		GetCardInvoiceHandler:           getCardInvoiceHandler,
		EventHandlers: []EventHandlerRegistration{
			{EventType: "transactions.transaction.created.v1", Handler: recomputeConsumer},
			{EventType: "transactions.transaction.updated.v1", Handler: recomputeConsumer},
			{EventType: "transactions.transaction.deleted.v1", Handler: recomputeConsumer},
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
