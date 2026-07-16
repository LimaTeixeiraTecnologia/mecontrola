package workflows

import (
	"errors"
	"fmt"
	"time"
)

type GoalEditStatus int

const (
	GoalEditActive GoalEditStatus = iota + 1
	GoalEditCompleted
	GoalEditCancelled
	GoalEditExpired
)

func (s GoalEditStatus) String() string {
	switch s {
	case GoalEditActive:
		return "active"
	case GoalEditCompleted:
		return "completed"
	case GoalEditCancelled:
		return "cancelled"
	case GoalEditExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (s GoalEditStatus) IsValid() bool {
	return s >= GoalEditActive && s <= GoalEditExpired
}

var errInvalidGoalEditStatus = errors.New("workflows: goal edit status inválido")

func ParseGoalEditStatus(s string) (GoalEditStatus, error) {
	switch s {
	case "active":
		return GoalEditActive, nil
	case "completed":
		return GoalEditCompleted, nil
	case "cancelled":
		return GoalEditCancelled, nil
	case "expired":
		return GoalEditExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidGoalEditStatus, s)
	}
}

type GoalEditAwaitingSlot int

const (
	GoalEditAwaitingGoal GoalEditAwaitingSlot = iota + 1
	GoalEditAwaitingConfirm
)

func (a GoalEditAwaitingSlot) String() string {
	switch a {
	case GoalEditAwaitingGoal:
		return "goal"
	case GoalEditAwaitingConfirm:
		return "confirm"
	default:
		return "unknown"
	}
}

func (a GoalEditAwaitingSlot) IsValid() bool {
	return a >= GoalEditAwaitingGoal && a <= GoalEditAwaitingConfirm
}

var errInvalidGoalEditAwaitingSlot = errors.New("workflows: goal edit awaiting slot inválido")

func ParseGoalEditAwaitingSlot(s string) (GoalEditAwaitingSlot, error) {
	switch s {
	case "goal":
		return GoalEditAwaitingGoal, nil
	case "confirm":
		return GoalEditAwaitingConfirm, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidGoalEditAwaitingSlot, s)
	}
}

type GoalEditState struct {
	Status            GoalEditStatus       `json:"status"`
	Awaiting          GoalEditAwaitingSlot `json:"awaiting"`
	ResourceID        string               `json:"resourceId"`
	PreviousGoal      string               `json:"previousGoal"`
	NewGoal           string               `json:"newGoal"`
	RepromptCount     int                  `json:"repromptCount"`
	MessageID         string               `json:"messageId"`
	IncomingMessageID string               `json:"incomingMessageId"`
	SuspendedAt       time.Time            `json:"suspendedAt"`
	ResumeText        string               `json:"resumeText"`
	ResponseText      string               `json:"responseText"`
	Expired           bool                 `json:"expired"`
}
