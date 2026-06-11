package kiwifypayload

import (
	"fmt"
	"strings"
	"time"
)

type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
		t.Time = parsed.UTC()
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, s); err == nil {
		t.Time = parsed.UTC()
		return nil
	}
	brt := time.FixedZone("BRT", -3*60*60)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, s, brt); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("kiwifypayload.Time: cannot parse %q", s)
}
