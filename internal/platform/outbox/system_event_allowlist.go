package outbox

var systemEventAllowlist = map[string]struct{}{
	"auth.failed":      {},
	"system.heartbeat": {},
}

func isSystemEvent(eventType string) bool {
	_, ok := systemEventAllowlist[eventType]
	return ok
}
