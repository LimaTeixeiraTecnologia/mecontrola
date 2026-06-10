package input

import "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"

type AlertQuery struct {
	Competence *valueobjects.Competence
	RootSlug   *valueobjects.RootSlug
	Threshold  *valueobjects.Threshold
	Cursor     string
	Limit      int
}
