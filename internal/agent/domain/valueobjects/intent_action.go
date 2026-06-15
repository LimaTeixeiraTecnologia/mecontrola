package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var ErrIntentActionEmpty = errors.New("agent.llm: intent action is empty")

var ErrIntentActionUnknown = errors.New("agent.llm: intent action is not allowed")

type IntentAction struct {
	value string
}

const (
	actionList   = "list"
	actionGet    = "get"
	actionCreate = "create"
	actionUpdate = "update"
	actionDelete = "delete"
)

func NewIntentAction(raw string) (IntentAction, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return IntentAction{}, ErrIntentActionEmpty
	}
	switch trimmed {
	case actionList, actionGet, actionCreate, actionUpdate, actionDelete:
		return IntentAction{value: trimmed}, nil
	default:
		return IntentAction{}, fmt.Errorf("agent.llm: %q: %w", raw, ErrIntentActionUnknown)
	}
}

func IntentActionList() IntentAction   { return IntentAction{value: actionList} }
func IntentActionGet() IntentAction    { return IntentAction{value: actionGet} }
func IntentActionCreate() IntentAction { return IntentAction{value: actionCreate} }
func IntentActionUpdate() IntentAction { return IntentAction{value: actionUpdate} }
func IntentActionDelete() IntentAction { return IntentAction{value: actionDelete} }

func (a IntentAction) String() string { return a.value }
func (a IntentAction) IsZero() bool   { return a.value == "" }

func (a IntentAction) Equal(o IntentAction) bool { return a.value == o.value }

func (a IntentAction) IsMutation() bool {
	return a.value == actionCreate || a.value == actionUpdate || a.value == actionDelete
}
