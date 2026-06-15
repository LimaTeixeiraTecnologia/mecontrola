package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var ErrIntentModuleEmpty = errors.New("agent.llm: intent module is empty")

var ErrIntentModuleUnknown = errors.New("agent.llm: intent module is not allowed")

type IntentModule struct {
	value string
}

const (
	moduleCategories   = "categories"
	moduleCards        = "cards"
	moduleBudgets      = "budgets"
	moduleTransactions = "transactions"
)

func NewIntentModule(raw string) (IntentModule, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return IntentModule{}, ErrIntentModuleEmpty
	}
	switch trimmed {
	case moduleCategories, moduleCards, moduleBudgets, moduleTransactions:
		return IntentModule{value: trimmed}, nil
	default:
		return IntentModule{}, fmt.Errorf("agent.llm: %q: %w", raw, ErrIntentModuleUnknown)
	}
}

func IntentModuleCategories() IntentModule   { return IntentModule{value: moduleCategories} }
func IntentModuleCards() IntentModule        { return IntentModule{value: moduleCards} }
func IntentModuleBudgets() IntentModule      { return IntentModule{value: moduleBudgets} }
func IntentModuleTransactions() IntentModule { return IntentModule{value: moduleTransactions} }

func (m IntentModule) String() string { return m.value }
func (m IntentModule) IsZero() bool   { return m.value == "" }

func (m IntentModule) Equal(o IntentModule) bool { return m.value == o.value }
