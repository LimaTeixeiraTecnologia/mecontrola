package input

import "errors"

type CreateCheckoutSessionInput struct {
	PlanID string
}

func (i *CreateCheckoutSessionInput) Validate() error {
	var errs []error
	if i.PlanID == "" {
		errs = append(errs, ErrPlanIDRequired)
	}
	return errors.Join(errs...)
}
