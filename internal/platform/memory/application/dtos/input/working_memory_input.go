package input

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type GetWorkingMemoryInput struct {
	ResourceID string
}

func (i *GetWorkingMemoryInput) Validate() error {
	var errs []error
	if i.ResourceID == "" {
		errs = append(errs, memory.ErrEmptyResourceID)
	}
	return errors.Join(errs...)
}

type UpsertWorkingMemoryInput struct {
	ResourceID string
	Content    string
}

func (i *UpsertWorkingMemoryInput) Validate() error {
	var errs []error
	if i.ResourceID == "" {
		errs = append(errs, memory.ErrEmptyResourceID)
	}
	if i.Content == "" {
		errs = append(errs, memory.ErrEmptyContent)
	}
	return errors.Join(errs...)
}
