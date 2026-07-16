package workflows

import (
	"errors"
	"fmt"
	"time"
)

type TreatmentNameEditStatus int

const (
	TreatmentNameEditActive TreatmentNameEditStatus = iota + 1
	TreatmentNameEditCompleted
	TreatmentNameEditCancelled
	TreatmentNameEditExpired
)

func (s TreatmentNameEditStatus) String() string {
	switch s {
	case TreatmentNameEditActive:
		return "active"
	case TreatmentNameEditCompleted:
		return "completed"
	case TreatmentNameEditCancelled:
		return "cancelled"
	case TreatmentNameEditExpired:
		return "expired"
	default:
		return "unknown"
	}
}

func (s TreatmentNameEditStatus) IsValid() bool {
	return s >= TreatmentNameEditActive && s <= TreatmentNameEditExpired
}

var errInvalidTreatmentNameEditStatus = errors.New("workflows: treatment name edit status inválido")

func ParseTreatmentNameEditStatus(s string) (TreatmentNameEditStatus, error) {
	switch s {
	case "active":
		return TreatmentNameEditActive, nil
	case "completed":
		return TreatmentNameEditCompleted, nil
	case "cancelled":
		return TreatmentNameEditCancelled, nil
	case "expired":
		return TreatmentNameEditExpired, nil
	default:
		return 0, fmt.Errorf("%w: %q", errInvalidTreatmentNameEditStatus, s)
	}
}

type TreatmentNameEditState struct {
	Status        TreatmentNameEditStatus `json:"status"`
	ResourceID    string                  `json:"resourceId"`
	ProvidedName  string                  `json:"providedName"`
	PreviousName  string                  `json:"previousName"`
	NewName       string                  `json:"newName"`
	RepromptCount int                     `json:"repromptCount"`
	MessageID     string                  `json:"messageId"`
	SuspendedAt   time.Time               `json:"suspendedAt"`
	ResumeText    string                  `json:"resumeText"`
	ResponseText  string                  `json:"responseText"`
	Expired       bool                    `json:"expired"`
}
