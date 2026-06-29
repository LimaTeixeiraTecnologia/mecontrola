package input

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type GetOrCreateThreadInput struct {
	ResourceID string
	ThreadID   string
}

func (i *GetOrCreateThreadInput) Validate() error {
	var errs []error
	if i.ResourceID == "" {
		errs = append(errs, memory.ErrEmptyResourceID)
	}
	if i.ThreadID == "" {
		errs = append(errs, memory.ErrEmptyThreadID)
	}
	return errors.Join(errs...)
}
