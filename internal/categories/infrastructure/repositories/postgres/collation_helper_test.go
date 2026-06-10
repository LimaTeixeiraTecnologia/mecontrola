//go:build integration

package postgres_test

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func ptBRCollator() *collate.Collator {
	return collate.New(language.BrazilianPortuguese, collate.IgnoreCase)
}
