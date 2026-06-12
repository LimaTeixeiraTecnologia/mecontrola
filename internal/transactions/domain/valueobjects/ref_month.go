package valueobjects

import (
	"errors"
	"fmt"
	"time"
)

var ErrRefMonthInvalid = errors.New("transactions: ref_month inválido (esperado YYYY-MM em America/Sao_Paulo)")

type RefMonth struct {
	value string
}

func NewRefMonth(raw string) (RefMonth, error) {
	if len(raw) != 7 || raw[4] != '-' {
		return RefMonth{}, fmt.Errorf("transactions: %q: %w", raw, ErrRefMonthInvalid)
	}
	for i, ch := range raw {
		if i == 4 {
			continue
		}
		if ch < '0' || ch > '9' {
			return RefMonth{}, fmt.Errorf("transactions: %q: %w", raw, ErrRefMonthInvalid)
		}
	}
	month := raw[5:7]
	if month < "01" || month > "12" {
		return RefMonth{}, fmt.Errorf("transactions: %q: %w", raw, ErrRefMonthInvalid)
	}
	return RefMonth{value: raw}, nil
}

func RefMonthFromTime(t time.Time, loc *time.Location) RefMonth {
	br := t.In(loc)
	return RefMonth{value: fmt.Sprintf("%04d-%02d", br.Year(), int(br.Month()))}
}

func (r RefMonth) String() string {
	return r.value
}

func (r RefMonth) Equal(other RefMonth) bool {
	return r.value == other.value
}

func (r RefMonth) Before(other RefMonth) bool {
	return r.value < other.value
}

func (r RefMonth) IsZero() bool {
	return r.value == ""
}

func (r RefMonth) Next() RefMonth {
	t, _ := time.Parse("2006-01", r.value)
	next := t.AddDate(0, 1, 0)
	return RefMonth{value: fmt.Sprintf("%04d-%02d", next.Year(), int(next.Month()))}
}
