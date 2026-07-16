package input

import (
	"errors"
	"strings"
)

var ErrInputEditCandidateAmountOrTermRequired = errors.New("amount_or_term: obrigatório informar valor ou termo")

type SearchEditCandidates struct {
	AmountCents int64  `json:"amount_cents"`
	Term        string `json:"term"`
	RefMonth    string `json:"ref_month"`
	Limit       int    `json:"limit"`
}

func (i *SearchEditCandidates) Validate() error {
	var errs []error
	if i.AmountCents <= 0 && strings.TrimSpace(i.Term) == "" {
		errs = append(errs, ErrInputEditCandidateAmountOrTermRequired)
	}
	return errors.Join(errs...)
}
