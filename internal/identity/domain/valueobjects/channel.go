package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

var ErrChannelEmpty = errors.New("identity: channel is empty")

var ErrChannelUnknown = errors.New("identity: channel is not supported")

type Channel struct {
	value string
}

const channelWhatsApp = "whatsapp"

func NewChannel(raw string) (Channel, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return Channel{}, ErrChannelEmpty
	}
	switch trimmed {
	case channelWhatsApp:
		return Channel{value: trimmed}, nil
	default:
		return Channel{}, fmt.Errorf("identity: %q: %w", raw, ErrChannelUnknown)
	}
}

func ChannelWhatsApp() Channel { return Channel{value: channelWhatsApp} }

func (c Channel) String() string { return c.value }

func (c Channel) IsZero() bool { return c.value == "" }

func (c Channel) Equal(o Channel) bool { return c.value == o.value }

func (c Channel) IsWhatsApp() bool { return c.value == channelWhatsApp }
