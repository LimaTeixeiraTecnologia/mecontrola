package agent

import "errors"

var (
	ErrAgentNotFound   = errors.New("agent: agent not found")
	ErrContractNotMet  = errors.New("agent: structured output contract not satisfied")
	ErrEmptyAgentID    = errors.New("agent: agent_id is required")
	ErrEmptyResourceID = errors.New("agent: resource_id is required")
	ErrEmptyThreadID   = errors.New("agent: thread_id is required")
	ErrEmptyMessage    = errors.New("agent: message is required")
	ErrRunNotFound     = errors.New("agent: run not found")
	ErrMaxToolRounds   = errors.New("agent: max tool iterations exceeded")
)
