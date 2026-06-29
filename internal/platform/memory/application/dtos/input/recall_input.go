package input

import (
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

type RecallInput struct {
	ResourceID string
	Query      string
	Embedding  []float32
	K          int
}

func (i *RecallInput) Validate() error {
	var errs []error
	if i.ResourceID == "" {
		errs = append(errs, memory.ErrEmptyResourceID)
	}
	if i.Query == "" {
		errs = append(errs, errors.New("query is required"))
	}
	if len(i.Embedding) == 0 {
		errs = append(errs, errors.New("embedding is required"))
	}
	if i.K <= 0 {
		errs = append(errs, errors.New("k must be greater than zero"))
	}
	return errors.Join(errs...)
}
