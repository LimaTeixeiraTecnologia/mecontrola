package postgres

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

func TestBuildTokenSearchQuery(t *testing.T) {
	query, args := buildTokenSearchQuery(interfaces.DictionaryTokenSearchQuery{
		Kind:   valueobjects.KindExpense,
		Tokens: []string{"netflix", "mercado"},
		Limit:  50,
	})

	assert.Equal(t, []any{"expense", "netflix", "mercado", 50}, args)
	assert.Contains(t, query, "lower(mecontrola.immutable_unaccent($2))")
	assert.Contains(t, query, "lower(mecontrola.immutable_unaccent($3))")
	assert.Contains(t, query, "LIMIT $4")
	assert.Contains(t, query, "AND deprecated_at IS NULL")
}

func TestBuildTokenSearchQueryIncludeDeprecated(t *testing.T) {
	query, _ := buildTokenSearchQuery(interfaces.DictionaryTokenSearchQuery{
		Kind:              valueobjects.KindIncome,
		Tokens:            []string{"salario"},
		Limit:             10,
		IncludeDeprecated: true,
	})
	assert.NotContains(t, query, "deprecated_at IS NULL")
}

func TestBuildFuzzySearchQuery(t *testing.T) {
	query, args := buildFuzzySearchQuery(interfaces.DictionaryFuzzySearchQuery{
		Kind:          valueobjects.KindExpense,
		Tokens:        []string{"netflyx", "mercadoo"},
		MinSimilarity: 0.4,
		Limit:         25,
	})

	assert.Equal(t, []any{"expense", "netflyx", "mercadoo", 0.4, 25}, args)
	assert.Equal(t, 2, strings.Count(query, "GREATEST("))
	assert.Contains(t, query, ">= $4")
	assert.Contains(t, query, "LIMIT $5")
	assert.Contains(t, query, "ORDER BY")
}
