package onboarding

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

var slugToCategoryKind = map[string]valueobjects.CategoryKind{
	"fixed_cost":        valueobjects.CategoryKindFixedCost,
	"knowledge":         valueobjects.CategoryKindKnowledge,
	"pleasures":         valueobjects.CategoryKindPleasures,
	"goals":             valueobjects.CategoryKindGoals,
	"financial_freedom": valueobjects.CategoryKindFinancialFreedom,
}

var categoryKindStringToSlug = map[string]string{
	"fixed_cost":        "fixed_cost",
	"knowledge":         "knowledge",
	"pleasures":         "pleasures",
	"goals":             "goals",
	"financial_freedom": "financial_freedom",
}
