package input

import (
	"errors"
	"fmt"
)

type InboundInput struct {
	ResourceID string
	ThreadID   string
	AgentID    string
	Message    string
	MessageID  string
}

func (i *InboundInput) Validate() error {
	var errs []error
	if i.ResourceID == "" {
		errs = append(errs, fmt.Errorf("resource_id: required"))
	}
	if i.ThreadID == "" {
		errs = append(errs, fmt.Errorf("thread_id: required"))
	}
	if i.AgentID == "" {
		errs = append(errs, fmt.Errorf("agent_id: required"))
	}
	if i.Message == "" {
		errs = append(errs, fmt.Errorf("message: required"))
	}
	return errors.Join(errs...)
}
