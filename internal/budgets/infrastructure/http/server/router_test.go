package server_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	budgetserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server"
	categoryserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server"
)

func passthroughGateway(next http.Handler) http.Handler { return next }

func TestBudgetsRouterRegisterDoesNotConflictWithCategoriesRouter(t *testing.T) {
	router := chi.NewRouter()

	categoriesRouter := categoryserver.NewCategoryRouter(nil, nil, nil, nil, passthroughGateway)
	budgetsRouter := budgetserver.NewBudgetsRouter(nil, nil, nil, nil, nil, nil, nil, nil, passthroughGateway)

	require.NotPanics(t, func() {
		categoriesRouter.Register(router)
		budgetsRouter.Register(router)
	})
}
