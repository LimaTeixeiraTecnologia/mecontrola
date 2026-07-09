package valueobjects

import (
	"errors"
	"fmt"
	"time"
)

var ErrCompetenceInvalid = errors.New("budgets: competence inválida")

var saoPauloLocation *time.Location

func SetSaoPauloLocation(loc *time.Location) {
	saoPauloLocation = loc
}

func SaoPauloLocation() *time.Location {
	return saoPauloLocation
}

type Competence struct {
	value string
}

func NewCompetence(raw string) (Competence, error) {
	if len(raw) != 7 || raw[4] != '-' {
		return Competence{}, fmt.Errorf("budgets: %q: %w", raw, ErrCompetenceInvalid)
	}
	for i, ch := range raw {
		if i == 4 {
			continue
		}
		if ch < '0' || ch > '9' {
			return Competence{}, fmt.Errorf("budgets: %q: %w", raw, ErrCompetenceInvalid)
		}
	}
	year := raw[0:4]
	month := raw[5:7]
	if month < "01" || month > "12" {
		return Competence{}, fmt.Errorf("budgets: %q: %w", raw, ErrCompetenceInvalid)
	}
	_ = year
	return Competence{value: raw}, nil
}

func CompetenceFromTime(t time.Time, loc *time.Location) Competence {
	br := t.In(loc)
	return Competence{value: fmt.Sprintf("%04d-%02d", br.Year(), int(br.Month()))}
}

func (c Competence) String() string {
	return c.value
}

func (c Competence) Before(other Competence) bool {
	return c.value < other.value
}

func (c Competence) Equal(other Competence) bool {
	return c.value == other.value
}

func (c Competence) IsZero() bool {
	return c.value == ""
}

func (c Competence) Next() Competence {
	t, _ := time.Parse("2006-01", c.value)
	next := t.AddDate(0, 1, 0)
	return Competence{value: fmt.Sprintf("%04d-%02d", next.Year(), int(next.Month()))}
}

func (c Competence) Prev() Competence {
	t, _ := time.Parse("2006-01", c.value)
	prev := t.AddDate(0, -1, 0)
	return Competence{value: fmt.Sprintf("%04d-%02d", prev.Year(), int(prev.Month()))}
}

var monthNamesPtBR = [13]string{
	"",
	"janeiro",
	"fevereiro",
	"março",
	"abril",
	"maio",
	"junho",
	"julho",
	"agosto",
	"setembro",
	"outubro",
	"novembro",
	"dezembro",
}

func FormatCompetencePtBR(c Competence) string {
	if c.IsZero() {
		return ""
	}
	t, err := time.Parse("2006-01", c.value)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s de %d", monthNamesPtBR[int(t.Month())], t.Year())
}
