package input

import (
	"errors"
	"fmt"
)

type AudioDecisionInput struct {
	Model         string
	Text          string
	Language      string
	Truncated     bool
	Confidence    *float64
	MinConfidence float64
	STTError      error
}

func (i *AudioDecisionInput) Validate() error {
	var errs []error
	if i.Model == "" {
		errs = append(errs, fmt.Errorf("model: required"))
	}
	if i.MinConfidence < 0 || i.MinConfidence > 1 {
		errs = append(errs, fmt.Errorf("min_confidence: must be between 0 and 1"))
	}
	if i.Confidence != nil && (*i.Confidence < 0 || *i.Confidence > 1) {
		errs = append(errs, fmt.Errorf("confidence: must be between 0 and 1"))
	}
	return errors.Join(errs...)
}
