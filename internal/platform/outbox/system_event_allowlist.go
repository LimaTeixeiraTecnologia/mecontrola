package outbox

var systemEventAllowlist = map[string]struct{}{
	"auth.failed":      {},
	"system.heartbeat": {},
}

func isSystemEvent(eventType string) bool {
	_, ok := systemEventAllowlist[eventType]
	return ok
}

var noUserEventAllowlist = map[string]struct{}{
	"billing.subscription.activated": {},
}

func isNoUserEvent(eventType string) bool {
	_, ok := noUserEventAllowlist[eventType]
	return ok
}
