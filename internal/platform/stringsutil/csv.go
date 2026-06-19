package stringsutil

import (
	"strings"
)

func ParseCSV(raw string) []string {
	if raw == "" {
		return nil
	}
	values := make([]string, 0)
	for item := range strings.SplitSeq(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}
