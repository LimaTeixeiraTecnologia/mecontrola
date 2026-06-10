package valueobjects

import (
	"errors"
	"fmt"
)

var ErrRootSlugUnknown = errors.New("budgets: root slug desconhecido")

type RootSlug uint8

const (
	RootSlugCustoFixo RootSlug = iota + 1
	RootSlugConhecimento
	RootSlugPrazeres
	RootSlugMetas
	RootSlugLiberdadeFinanceira
)

const (
	slugCustoFixo           = "expense.custo_fixo"
	slugConhecimento        = "expense.conhecimento"
	slugPrazeres            = "expense.prazeres"
	slugMetas               = "expense.metas"
	slugLiberdadeFinanceira = "expense.liberdade_financeira"
)

var canonicalOrder = [5]RootSlug{
	RootSlugCustoFixo,
	RootSlugConhecimento,
	RootSlugPrazeres,
	RootSlugMetas,
	RootSlugLiberdadeFinanceira,
}

func CanonicalOrder() [5]RootSlug {
	return canonicalOrder
}

func ParseRootSlug(s string) (RootSlug, error) {
	switch s {
	case slugCustoFixo:
		return RootSlugCustoFixo, nil
	case slugConhecimento:
		return RootSlugConhecimento, nil
	case slugPrazeres:
		return RootSlugPrazeres, nil
	case slugMetas:
		return RootSlugMetas, nil
	case slugLiberdadeFinanceira:
		return RootSlugLiberdadeFinanceira, nil
	default:
		return 0, fmt.Errorf("budgets: %q: %w", s, ErrRootSlugUnknown)
	}
}

func (r RootSlug) String() string {
	switch r {
	case RootSlugCustoFixo:
		return slugCustoFixo
	case RootSlugConhecimento:
		return slugConhecimento
	case RootSlugPrazeres:
		return slugPrazeres
	case RootSlugMetas:
		return slugMetas
	case RootSlugLiberdadeFinanceira:
		return slugLiberdadeFinanceira
	default:
		return ""
	}
}
