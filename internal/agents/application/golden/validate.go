package golden

import (
	"errors"
	"fmt"
)

func validateCase(c Case) error {
	var errs []error
	if c.Name == "" {
		errs = append(errs, errors.New("name: obrigatório"))
	}
	if !c.Category.IsValid() {
		errs = append(errs, fmt.Errorf("category: valor inválido %q", c.Category))
	}
	if c.Input == "" {
		errs = append(errs, errors.New("input: obrigatório"))
	}
	if err := validateToolExpectation(c); err != nil {
		errs = append(errs, err)
	}
	if c.ResponseProperty == nil {
		errs = append(errs, errors.New("responseProperty: obrigatório"))
	}
	if c.ResponseDescribe == "" {
		errs = append(errs, errors.New("responseDescribe: obrigatório para diagnóstico do gate"))
	}
	return errors.Join(errs...)
}

func validateToolExpectation(c Case) error {
	var errs []error
	if c.ExpectedTool == "" && len(c.ExpectedTools) == 0 && len(c.ExpectedAnyOfTools) == 0 && !c.NoToolExpected {
		errs = append(errs, errors.New("expectedTool: obrigatório informar expectedTool, expectedTools, expectedAnyOfTools ou noToolExpected=true"))
	}
	if c.ExpectedTool != "" && len(c.ExpectedTools) > 0 {
		errs = append(errs, errors.New("expectedTool: não pode coexistir com expectedTools"))
	}
	if c.ExpectedTool != "" && len(c.ExpectedAnyOfTools) > 0 {
		errs = append(errs, errors.New("expectedTool: não pode coexistir com expectedAnyOfTools"))
	}
	if len(c.ExpectedTools) > 0 && len(c.ExpectedAnyOfTools) > 0 {
		errs = append(errs, errors.New("expectedTools: não pode coexistir com expectedAnyOfTools"))
	}
	return errors.Join(errs...)
}
