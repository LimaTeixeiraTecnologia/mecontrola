package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
)

type BudgetsRouter struct {
	gatewayAuth       func(http.Handler) http.Handler
	createBudget      *handlers.CreateBudgetHandler
	activateBudget    *handlers.ActivateBudgetHandler
	deleteBudget      *handlers.DeleteBudgetHandler
	createRecurrence  *handlers.CreateRecurrenceHandler
	upsertExpense     *handlers.UpsertExpenseHandler
	deleteExpense     *handlers.DeleteExpenseHandler
	getMonthlySummary *handlers.GetMonthlySummaryHandler
	listAlerts        *handlers.ListAlertsHandler
}

func NewBudgetsRouter(
	createBudget *handlers.CreateBudgetHandler,
	activateBudget *handlers.ActivateBudgetHandler,
	deleteBudget *handlers.DeleteBudgetHandler,
	createRecurrence *handlers.CreateRecurrenceHandler,
	upsertExpense *handlers.UpsertExpenseHandler,
	deleteExpense *handlers.DeleteExpenseHandler,
	getMonthlySummary *handlers.GetMonthlySummaryHandler,
	listAlerts *handlers.ListAlertsHandler,
	gatewayAuth func(http.Handler) http.Handler,
) *BudgetsRouter {
	return &BudgetsRouter{
		gatewayAuth:       gatewayAuth,
		createBudget:      createBudget,
		activateBudget:    activateBudget,
		deleteBudget:      deleteBudget,
		createRecurrence:  createRecurrence,
		upsertExpense:     upsertExpense,
		deleteExpense:     deleteExpense,
		getMonthlySummary: getMonthlySummary,
		listAlerts:        listAlerts,
	}
}

func (rt *BudgetsRouter) Register(r chi.Router) {
	r.Group(func(g chi.Router) {
		g.Use(rt.gatewayAuth)
		g.Use(middleware.InjectPrincipalFromHeader)
		g.Use(middleware.RequireUser)
		g.Route("/api/v1/budgets", func(b chi.Router) {
			b.Post("/", rt.createBudget.Handle)
			b.Post("/recurrence", rt.createRecurrence.Handle)
			b.Get("/alerts", rt.listAlerts.Handle)
			b.Post("/expenses", rt.upsertExpense.HandleCreate)
			b.Patch("/expenses/{id}", rt.upsertExpense.HandleUpdate)
			b.Delete("/expenses/{id}", rt.deleteExpense.Handle)
			b.Post("/{competence}/activate", rt.activateBudget.Handle)
			b.Delete("/{competence}", rt.deleteBudget.Handle)
			b.Get("/{competence}/summary", rt.getMonthlySummary.Handle)
		})
	})
}
