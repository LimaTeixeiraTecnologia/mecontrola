package signature

import "net/http"

func Compose(secretCurrent, secretNext string, onInvalid func()) func(http.Handler) http.Handler {
	return ComposeWithStatus(secretCurrent, secretNext, onInvalid, nil)
}

func ComposeWithStatus(secretCurrent, secretNext string, onInvalid func(), onStatus func(status string)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return RawBody(HMACWithMetrics(secretCurrent, secretNext, onInvalid, onStatus)(next))
	}
}
