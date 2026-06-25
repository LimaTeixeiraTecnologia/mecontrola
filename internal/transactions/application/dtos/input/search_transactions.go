package input

import (
	"errors"
	"strings"
)

var ErrInputSearchQueryRequired = errors.New("query: obrigatório")

type SearchTransactions struct {
	Query    string `json:"query"`
	RefMonth string `json:"ref_month"`
	Limit    int    `json:"limit"`
}

func (i *SearchTransactions) Validate() error {
	var errs []error
	if strings.TrimSpace(i.Query) == "" {
		errs = append(errs, ErrInputSearchQueryRequired)
	}
	return errors.Join(errs...)
}
