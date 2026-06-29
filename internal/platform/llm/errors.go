package llm

import (
	"errors"
	"net/http"
)

var ErrEmptyChoices = errors.New("llm.openrouter: response has no choices")
var ErrProviderUpstream = errors.New("llm.openrouter: upstream error")
var ErrContractViolation = errors.New("llm: structured output contract violated")

func classifyStatus(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return "unauthorized"
	case code == http.StatusPaymentRequired:
		return "no_credit"
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code == http.StatusRequestTimeout:
		return "timeout"
	case code >= 500:
		return "upstream_5xx"
	default:
		return "client_4xx"
	}
}

func truncatePreview(b []byte) string {
	const maxLen = 256
	if len(b) > maxLen {
		return string(b[:maxLen]) + "..."
	}
	return string(b)
}
