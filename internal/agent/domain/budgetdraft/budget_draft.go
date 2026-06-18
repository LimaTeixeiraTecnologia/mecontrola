package budgetdraft

import (
	"errors"
	"fmt"
	"maps"
	"sort"
)

const (
	PendingActionKind = "budget_config"

	SlugCustoFixo           = "expense.custo_fixo"
	SlugConhecimento        = "expense.conhecimento"
	SlugPrazeres            = "expense.prazeres"
	SlugMetas               = "expense.metas"
	SlugLiberdadeFinanceira = "expense.liberdade_financeira"

	totalBasisPoints = 10000
	minBasisPoints   = 1
)

var (
	ErrSlugNotAllowed   = errors.New("agent.budgetdraft: root slug fora da allowlist")
	ErrBasisPointsRange = errors.New("agent.budgetdraft: basis points deve estar entre 1 e 10000")
	ErrTotalNegative    = errors.New("agent.budgetdraft: total em centavos não pode ser negativo")
)

var allowedSlugs = map[string]struct{}{
	SlugCustoFixo:           {},
	SlugConhecimento:        {},
	SlugPrazeres:            {},
	SlugMetas:               {},
	SlugLiberdadeFinanceira: {},
}

type Draft struct {
	totalCents  int64
	allocations map[string]int
	competence  string
}

func New(competence string) Draft {
	return Draft{
		totalCents:  0,
		allocations: make(map[string]int),
		competence:  competence,
	}
}

func Restore(totalCents int64, allocations map[string]int, competence string) (Draft, error) {
	if totalCents < 0 {
		return Draft{}, ErrTotalNegative
	}
	clean := make(map[string]int, len(allocations))
	for slug, bp := range allocations {
		if _, ok := allowedSlugs[slug]; !ok {
			return Draft{}, fmt.Errorf("%w: %q", ErrSlugNotAllowed, slug)
		}
		if bp < minBasisPoints || bp > totalBasisPoints {
			return Draft{}, fmt.Errorf("%w: %q=%d", ErrBasisPointsRange, slug, bp)
		}
		clean[slug] = bp
	}
	return Draft{totalCents: totalCents, allocations: clean, competence: competence}, nil
}

func (d Draft) TotalCents() int64 {
	return d.totalCents
}

func (d Draft) Competence() string {
	return d.competence
}

func (d Draft) Allocations() map[string]int {
	return maps.Clone(d.allocations)
}

func (d Draft) SumBasisPoints() int {
	sum := 0
	for _, bp := range d.allocations {
		sum += bp
	}
	return sum
}

func (d Draft) RemainingBasisPoints() int {
	return totalBasisPoints - d.SumBasisPoints()
}

func (d Draft) IsComplete() bool {
	return d.totalCents > 0 && d.SumBasisPoints() == totalBasisPoints
}

func (d Draft) MissingSlugs() []string {
	missing := make([]string, 0, len(allowedSlugs))
	for slug := range allowedSlugs {
		if _, ok := d.allocations[slug]; !ok {
			missing = append(missing, slug)
		}
	}
	sort.Strings(missing)
	return missing
}

type Change struct {
	TotalCents  int64
	Allocations map[string]int
}

func (d Draft) Merge(change Change) (Draft, error) {
	next := Draft{
		totalCents:  d.totalCents,
		allocations: maps.Clone(d.allocations),
		competence:  d.competence,
	}
	if next.allocations == nil {
		next.allocations = make(map[string]int)
	}
	if change.TotalCents < 0 {
		return Draft{}, ErrTotalNegative
	}
	if change.TotalCents > 0 {
		next.totalCents = change.TotalCents
	}
	for slug, bp := range change.Allocations {
		if _, ok := allowedSlugs[slug]; !ok {
			return Draft{}, fmt.Errorf("%w: %q", ErrSlugNotAllowed, slug)
		}
		if bp < minBasisPoints || bp > totalBasisPoints {
			return Draft{}, fmt.Errorf("%w: %q=%d", ErrBasisPointsRange, slug, bp)
		}
		next.allocations[slug] = bp
	}
	return next, nil
}

func IsAllowedSlug(slug string) bool {
	_, ok := allowedSlugs[slug]
	return ok
}
