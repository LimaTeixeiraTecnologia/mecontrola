package output

type FutureBudgetOutput struct {
	Competence string `json:"competence"`
	State      string `json:"state"`
}

type ListFutureBudgetsOutput struct {
	Budgets []FutureBudgetOutput `json:"budgets"`
}

type SyncFutureBudgetsOutput struct {
	UpdatedCompetences       []string `json:"updated_competences"`
	SkippedActiveCompetences []string `json:"skipped_active_competences"`
}
