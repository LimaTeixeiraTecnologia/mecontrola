package config

import (
	"errors"
	"slices"
)

var ErrProducerNotAllowed = errors.New("budgets: producer_source não autorizado")

var AllowedProducerSources = []string{
	"kiwify",
}

func IsAllowedProducerSource(source string) bool {
	return slices.Contains(AllowedProducerSources, source)
}
