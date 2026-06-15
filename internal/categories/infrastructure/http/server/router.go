package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
)

type CategoryRouter struct {
	listCategoriesHandler   *handlers.ListCategoriesHandler
	getCategoryHandler      *handlers.GetCategoryHandler
	listDictionaryHandler   *handlers.ListDictionaryHandler
	searchDictionaryHandler *handlers.SearchDictionaryHandler
	gatewayAuth             func(http.Handler) http.Handler
}

func NewCategoryRouter(
	listCategoriesHandler *handlers.ListCategoriesHandler,
	getCategoryHandler *handlers.GetCategoryHandler,
	listDictionaryHandler *handlers.ListDictionaryHandler,
	searchDictionaryHandler *handlers.SearchDictionaryHandler,
	gatewayAuth func(http.Handler) http.Handler,
) *CategoryRouter {
	return &CategoryRouter{
		listCategoriesHandler:   listCategoriesHandler,
		getCategoryHandler:      getCategoryHandler,
		listDictionaryHandler:   listDictionaryHandler,
		searchDictionaryHandler: searchDictionaryHandler,
		gatewayAuth:             gatewayAuth,
	}
}

func (rt *CategoryRouter) Register(r chi.Router) {
	r.Route("/api/v1", func(sub chi.Router) {
		sub.Use(rt.gatewayAuth)
		sub.Use(middleware.InjectPrincipalFromHeader)
		sub.With(middleware.RequireUser).Route("/categories", func(cat chi.Router) {
			cat.Get("/", rt.listCategoriesHandler.Handle)
			cat.Get("/{id}", rt.getCategoryHandler.Handle)
		})
		sub.With(middleware.RequireUser).Route("/category-dictionary", func(dict chi.Router) {
			dict.Get("/", rt.listDictionaryHandler.Handle)
			dict.Get("/search", rt.searchDictionaryHandler.Handle)
		})
	})
}
