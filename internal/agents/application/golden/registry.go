package golden

import (
	"errors"
	"fmt"
)

func AllCases() []Case {
	var cases []Case
	cases = append(cases, expenseIncomeCases()...)
	cases = append(cases, queryCases()...)
	cases = append(cases, cardCases()...)
	cases = append(cases, budgetCases()...)
	cases = append(cases, recurrenceCases()...)
	cases = append(cases, onboardingCases()...)
	cases = append(cases, pendingCases()...)
	cases = append(cases, confirmationCases()...)
	cases = append(cases, followUpCases()...)
	cases = append(cases, toolErrorCases()...)
	cases = append(cases, ambiguityCases()...)
	cases = append(cases, whatsAppFormatCases()...)
	cases = append(cases, noInternalTermsCases()...)
	cases = append(cases, journeyCases()...)
	return cases
}

func CasesByCategory(category Category) []Case {
	var out []Case
	for _, c := range AllCases() {
		if c.Category == category {
			out = append(out, c)
		}
	}
	return out
}

func ValidateAll(cases []Case) error {
	var errs []error
	for _, c := range cases {
		if err := c.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("caso %q: %w", c.Name, err))
		}
	}
	return errors.Join(errs...)
}
