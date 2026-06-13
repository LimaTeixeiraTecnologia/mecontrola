package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
)

type CardRouter struct {
	createHandler     *handlers.CreateCardHandler
	listHandler       *handlers.ListCardsHandler
	getHandler        *handlers.GetCardHandler
	updateHandler     *handlers.UpdateCardHandler
	deleteHandler     *handlers.DeleteCardHandler
	invoiceForHandler *handlers.InvoiceForHandler
	idemStorage       idempotency.Storage
	o11y              observability.Observability
	gatewayAuth       func(http.Handler) http.Handler
	userRateLimit     func(http.Handler) http.Handler
}

func NewCardRouter(
	create *handlers.CreateCardHandler,
	list *handlers.ListCardsHandler,
	get *handlers.GetCardHandler,
	update *handlers.UpdateCardHandler,
	delete *handlers.DeleteCardHandler,
	invoiceFor *handlers.InvoiceForHandler,
	idemStorage idempotency.Storage,
	o11y observability.Observability,
	gatewayAuth func(http.Handler) http.Handler,
	userRateLimit func(http.Handler) http.Handler,
) *CardRouter {
	return &CardRouter{
		createHandler:     create,
		listHandler:       list,
		getHandler:        get,
		updateHandler:     update,
		deleteHandler:     delete,
		invoiceForHandler: invoiceFor,
		idemStorage:       idemStorage,
		o11y:              o11y,
		gatewayAuth:       gatewayAuth,
		userRateLimit:     userRateLimit,
	}
}

func (rt *CardRouter) Register(r chi.Router) {
	idemMiddleware := idempotency.Middleware("card", rt.idemStorage, 24*time.Hour, rt.o11y)

	r.Route("/api/v1/cards", func(sub chi.Router) {
		sub.Use(rt.gatewayAuth)
		sub.Use(middleware.InjectPrincipalFromHeaderWithO11y(rt.o11y))
		sub.Use(middleware.RequireUserWithO11y(rt.o11y))
		sub.Use(rt.userRateLimit)

		sub.With(idemMiddleware).Post("/", rt.createHandler.Handle)
		sub.Get("/", rt.listHandler.Handle)

		sub.Route("/{id}", func(idSub chi.Router) {
			idSub.Get("/", rt.getHandler.Handle)
			idSub.With(idemMiddleware).Put("/", rt.updateHandler.Handle)
			idSub.With(idemMiddleware).Delete("/", rt.deleteHandler.Handle)
			idSub.Get("/invoices", rt.invoiceForHandler.Handle)
		})
	})
}
