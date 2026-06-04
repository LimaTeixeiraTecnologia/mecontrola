package valueobjects

import (
	"regexp"
	"strings"
)

type WebhookEventID struct{ value string }

var uuidV4Pattern = regexp.MustCompile(
	`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`,
)

func NewWebhookEventID(v string) (WebhookEventID, error) {
	if !uuidV4Pattern.MatchString(strings.TrimSpace(v)) {
		return WebhookEventID{}, ErrInvalidWebhookEventID
	}
	return WebhookEventID{value: strings.ToLower(strings.TrimSpace(v))}, nil
}

func (w WebhookEventID) String() string { return w.value }
func (w WebhookEventID) IsZero() bool   { return w.value == "" }
