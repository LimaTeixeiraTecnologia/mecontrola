package output

import "time"

type Invoice struct {
	ClosingDate time.Time
	DueDate     time.Time
}
