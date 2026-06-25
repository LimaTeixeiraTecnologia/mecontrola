package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var ErrExternalIDEmpty = errors.New("identity: external_id is empty")

var ErrExternalIDInvalid = errors.New("identity: external_id is invalid for channel")

var ErrExternalIDChannelRequired = errors.New("identity: channel is required to validate external_id")

type ExternalID struct {
	channel Channel
	value   string
}

func NewExternalID(channel Channel, raw string) (ExternalID, error) {
	if channel.IsZero() {
		return ExternalID{}, ErrExternalIDChannelRequired
	}
	cleaned, err := normalizeExternalID(channel, raw)
	if err != nil {
		return ExternalID{}, err
	}
	return ExternalID{channel: channel, value: cleaned}, nil
}

func (e ExternalID) String() string { return e.value }

func (e ExternalID) Channel() Channel { return e.channel }

func (e ExternalID) IsZero() bool { return e.value == "" }

func (e ExternalID) Equal(o ExternalID) bool {
	return e.channel.Equal(o.channel) && e.value == o.value
}

func (e ExternalID) Masked() string {
	if e.channel.IsWhatsApp() {
		wa, err := NewWhatsAppNumber(e.value)
		if err == nil {
			return wa.Masked()
		}
	}
	if len(e.value) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(e.value)-4) + e.value[len(e.value)-4:]
}

func normalizeExternalID(channel Channel, raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", ErrExternalIDEmpty
	}
	switch {
	case channel.IsWhatsApp():
		wa, err := NewWhatsAppNumber(trimmed)
		if err != nil {
			return "", fmt.Errorf("identity: external_id whatsapp: %w", err)
		}
		return wa.String(), nil
	default:
		return "", fmt.Errorf("identity: %q: %w", channel.String(), ErrChannelUnknown)
	}
}
