package capability

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

var ErrCapabilityModeInvalid = errors.New("agent.application.capability: mode is invalid")

type CapabilityMode int

const (
	ModeRead CapabilityMode = iota + 1
	ModeWrite
)

type CapabilitySpec struct {
	ID                   string
	Description          string
	Kind                 intent.Kind
	WorkflowID           string
	ToolName             string
	Mode                 CapabilityMode
	RequiresConfirmation bool
	SupportsSuspend      bool
	SupportsResume       bool
	Channels             []string
	MetricsKey           string
}

func (m CapabilityMode) String() string {
	switch m {
	case ModeRead:
		return "read"
	case ModeWrite:
		return "write"
	default:
		return ""
	}
}

func (m CapabilityMode) IsValid() bool {
	return m == ModeRead || m == ModeWrite
}

func ParseCapabilityMode(raw string) (CapabilityMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModeRead.String():
		return ModeRead, nil
	case ModeWrite.String():
		return ModeWrite, nil
	default:
		return 0, fmt.Errorf("agent.application.capability: parse mode %q: %w", raw, ErrCapabilityModeInvalid)
	}
}
