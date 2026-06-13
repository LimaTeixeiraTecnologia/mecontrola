package outbox

var systemEventAllowlist = map[string]struct{}{}

func isSystemEvent(eventType string) bool {
	_, ok := systemEventAllowlist[eventType]
	return ok
}
