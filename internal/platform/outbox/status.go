package outbox

type Status int

const (
	StatusPending Status = iota + 1
	StatusProcessing
	StatusPublished
	StatusFailed
)
