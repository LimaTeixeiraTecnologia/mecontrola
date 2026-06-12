package valueobjects

import "time"

type GraceWindow time.Duration

const DefaultGraceWindow GraceWindow = GraceWindow(3 * 24 * time.Hour)

func (g GraceWindow) Duration() time.Duration {
	return time.Duration(g)
}
