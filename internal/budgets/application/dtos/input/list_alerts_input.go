package input

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"

type ListAlertsInput struct {
	UserID     string
	Competence *valueobjects.Competence
	RootSlug   *valueobjects.RootSlug
	Threshold  *valueobjects.Threshold
	Cursor     string
	Limit      int
}
