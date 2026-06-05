package sqlnull

import "time"

func Str(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func Time(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
