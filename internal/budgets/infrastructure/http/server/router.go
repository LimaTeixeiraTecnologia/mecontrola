package server

import (
	"github.com/go-chi/chi/v5"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/http/server/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/http/server/middleware"
)

type BudgetsRouter struct {
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
) *BudgetsRouter {
	return &BudgetsRouter{
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
	r.Route("/api/v1", func(sub chi.Router) {
		sub.With(middleware.RequireUser).Route("/budgets", func(b chi.Router) {
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
