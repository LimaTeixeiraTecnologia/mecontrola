package valueobjects

import (
	"errors"
	"fmt"
	"regexp"
)

var ErrExternalTransactionIDInvalid = errors.New("budgets: external_transaction_id inválido: deve ser UUID v4 (lowercase) ou ULID canônico (uppercase Crockford base32)")

var (
	uuidV4Regex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	ulidRegex   = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
)

type ExternalTransactionID struct {
	value string
}

func NewExternalTransactionID(raw string) (ExternalTransactionID, error) {
	if uuidV4Regex.MatchString(raw) {
		return ExternalTransactionID{value: raw}, nil
	}
	if ulidRegex.MatchString(raw) {
		return ExternalTransactionID{value: raw}, nil
	}
	return ExternalTransactionID{}, fmt.Errorf("budgets: %q: %w", raw, ErrExternalTransactionIDInvalid)
}

func (e ExternalTransactionID) String() string {
	return e.value
}
