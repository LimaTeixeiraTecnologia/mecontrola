//go:build tools

// Package tools declara dependencias de tooling Go instaladas via go install.
// Nao faz parte do binario de producao (build tag "tools").
// Referencia: ADR-014 (tool pinning).
package tools

import (
	_ "github.com/stretchr/testify"
	_ "github.com/vektra/mockery/v2"
)
