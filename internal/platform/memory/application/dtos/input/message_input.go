package input

import (
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type AppendMessageInput struct {
	PlatformThreadID uuid.UUID
	ResourceID       string
	Role             string
	Content          string
	Parts            []byte
}

func (i *AppendMessageInput) Validate() error {
	var errs []error
	if i.PlatformThreadID == uuid.Nil {
		errs = append(errs, errors.New("platform_thread_id is required"))
	}
	if i.ResourceID == "" {
		errs = append(errs, memory.ErrEmptyResourceID)
	}
	if i.Content == "" {
		errs = append(errs, memory.ErrEmptyContent)
	}
	if _, err := memory.ParseMessageRole(i.Role); err != nil {
		errs = append(errs, memory.ErrInvalidRole)
	}
	return errors.Join(errs...)
}

type RecentMessagesInput struct {
	PlatformThreadID uuid.UUID
	Limit            int
}

func (i *RecentMessagesInput) Validate() error {
	var errs []error
	if i.PlatformThreadID == uuid.Nil {
		errs = append(errs, errors.New("platform_thread_id is required"))
	}
	if i.Limit <= 0 {
		errs = append(errs, errors.New("limit must be greater than zero"))
	}
	return errors.Join(errs...)
}
