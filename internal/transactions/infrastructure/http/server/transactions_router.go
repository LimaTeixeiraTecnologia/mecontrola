package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/http/server/handlers"
)

type TransactionsRouter struct {
	createTransaction       *handlers.CreateTransactionHandler
	updateTransaction       *handlers.UpdateTransactionHandler
	deleteTransaction       *handlers.DeleteTransactionHandler
	getTransaction          *handlers.GetTransactionHandler
	listTransactions        *handlers.ListTransactionsHandler
	createCardPurchase      *handlers.CreateCardPurchaseHandler
	updateCardPurchase      *handlers.UpdateCardPurchaseHandler
	deleteCardPurchase      *handlers.DeleteCardPurchaseHandler
	getCardPurchase         *handlers.GetCardPurchaseHandler
	listCardPurchases       *handlers.ListCardPurchasesHandler
	createRecurringTemplate *handlers.CreateRecurringTemplateHandler
	updateRecurringTemplate *handlers.UpdateRecurringTemplateHandler
	deleteRecurringTemplate *handlers.DeleteRecurringTemplateHandler
	getRecurringTemplate    *handlers.GetRecurringTemplateHandler
	listRecurringTemplates  *handlers.ListRecurringTemplatesHandler
	getMonthlySummary       *handlers.GetMonthlySummaryHandler
	listMonthlyEntries      *handlers.ListMonthlyEntriesHandler
	idemStorage             idempotency.Storage
	idemTTL                 time.Duration
	o11y                    observability.Observability
	gatewayAuth             func(http.Handler) http.Handler
}

func NewTransactionsRouter(
	createTx *handlers.CreateTransactionHandler,
	updateTx *handlers.UpdateTransactionHandler,
	deleteTx *handlers.DeleteTransactionHandler,
	getTx *handlers.GetTransactionHandler,
	listTx *handlers.ListTransactionsHandler,
	createCP *handlers.CreateCardPurchaseHandler,
	updateCP *handlers.UpdateCardPurchaseHandler,
	deleteCP *handlers.DeleteCardPurchaseHandler,
	getCP *handlers.GetCardPurchaseHandler,
	listCP *handlers.ListCardPurchasesHandler,
	createRT *handlers.CreateRecurringTemplateHandler,
	updateRT *handlers.UpdateRecurringTemplateHandler,
	deleteRT *handlers.DeleteRecurringTemplateHandler,
	getRT *handlers.GetRecurringTemplateHandler,
	listRT *handlers.ListRecurringTemplatesHandler,
	getMS *handlers.GetMonthlySummaryHandler,
	listME *handlers.ListMonthlyEntriesHandler,
	idemStorage idempotency.Storage,
	idemTTL time.Duration,
	o11y observability.Observability,
	gatewayAuth func(http.Handler) http.Handler,
) *TransactionsRouter {
	return &TransactionsRouter{
		createTransaction:       createTx,
		updateTransaction:       updateTx,
		deleteTransaction:       deleteTx,
		getTransaction:          getTx,
		listTransactions:        listTx,
		createCardPurchase:      createCP,
		updateCardPurchase:      updateCP,
		deleteCardPurchase:      deleteCP,
		getCardPurchase:         getCP,
		listCardPurchases:       listCP,
		createRecurringTemplate: createRT,
		updateRecurringTemplate: updateRT,
		deleteRecurringTemplate: deleteRT,
		getRecurringTemplate:    getRT,
		listRecurringTemplates:  listRT,
		getMonthlySummary:       getMS,
		listMonthlyEntries:      listME,
		idemStorage:             idemStorage,
		idemTTL:                 idemTTL,
		o11y:                    o11y,
		gatewayAuth:             gatewayAuth,
	}
}

func (rt *TransactionsRouter) Register(r chi.Router) {
	idem := idempotency.Middleware("transactions", rt.idemStorage, rt.idemTTL, rt.o11y)

	r.Group(func(g chi.Router) {
		g.Use(rt.gatewayAuth)
		g.Use(middleware.InjectPrincipalFromHeaderWithO11y(rt.o11y))
		g.Use(middleware.RequireUser)

		g.Route("/api/v1/transactions", func(sub chi.Router) {
			sub.With(idem).Post("/", rt.createTransaction.Handle)
			sub.Get("/", rt.listTransactions.Handle)
			sub.Get("/{id}", rt.getTransaction.Handle)
			sub.With(idem).Patch("/{id}", rt.updateTransaction.Handle)
			sub.With(idem).Delete("/{id}", rt.deleteTransaction.Handle)
		})

		g.Route("/api/v1/card-purchases", func(sub chi.Router) {
			sub.With(idem).Post("/", rt.createCardPurchase.Handle)
			sub.Get("/", rt.listCardPurchases.Handle)
			sub.Get("/{id}", rt.getCardPurchase.Handle)
			sub.With(idem).Patch("/{id}", rt.updateCardPurchase.Handle)
			sub.With(idem).Delete("/{id}", rt.deleteCardPurchase.Handle)
		})

		g.Route("/api/v1/recurring-templates", func(sub chi.Router) {
			sub.With(idem).Post("/", rt.createRecurringTemplate.Handle)
			sub.Get("/", rt.listRecurringTemplates.Handle)
			sub.Get("/{id}", rt.getRecurringTemplate.Handle)
			sub.With(idem).Patch("/{id}", rt.updateRecurringTemplate.Handle)
			sub.With(idem).Delete("/{id}", rt.deleteRecurringTemplate.Handle)
		})

		g.Route("/api/v1/months", func(sub chi.Router) {
			sub.Get("/{ref_month}", rt.getMonthlySummary.Handle)
			sub.Get("/{ref_month}/entries", rt.listMonthlyEntries.Handle)
		})
	})
}
