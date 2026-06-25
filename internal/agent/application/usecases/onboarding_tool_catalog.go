package usecases

const (
	ToolSaveOnboardingObjective    = "save_onboarding_objective"
	ToolSaveOnboardingIncome       = "save_onboarding_income"
	ToolSaveOnboardingCard         = "save_onboarding_card"
	ToolSaveOnboardingBudgetSplits = "save_onboarding_budget_splits"
	ToolCompleteOnboardingSession  = "complete_onboarding_session"
)

var onboardingBudgetSlugs = []string{
	"expense.custo_fixo",
	"expense.conhecimento",
	"expense.prazeres",
	"expense.metas",
	"expense.liberdade_financeira",
}
