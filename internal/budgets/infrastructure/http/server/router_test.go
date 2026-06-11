package server_test

import (
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	budgetserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server"
	categoryserver "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/infrastructure/http/server"
)

func TestBudgetsRouterRegisterDoesNotConflictWithCategoriesRouter(t *testing.T) {
	router := chi.NewRouter()

	categoriesRouter := categoryserver.NewCategoryRouter(nil, nil, nil, nil)
	budgetsRouter := budgetserver.NewBudgetsRouter(nil, nil, nil, nil, nil, nil, nil, nil)

	require.NotPanics(t, func() {
		categoriesRouter.Register(router)
		budgetsRouter.Register(router)
	})
}
