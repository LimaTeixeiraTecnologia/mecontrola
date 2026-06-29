package memory

import "errors"

var (
	ErrThreadNotFound        = errors.New("thread not found")
	ErrMessageNotFound       = errors.New("message not found")
	ErrWorkingMemoryNotFound = errors.New("working memory not found")
	ErrInvalidRole           = errors.New("invalid message role")
	ErrEmptyResourceID       = errors.New("resource_id is required")
	ErrEmptyThreadID         = errors.New("thread_id is required")
	ErrEmptyContent          = errors.New("content is required")
)
