package tools

import (
	"fmt"
	"time"
)

func currentCompetence(loc *time.Location) string {
	now := time.Now().UTC().In(loc)
	return fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
}
