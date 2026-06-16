package notification

import (
	"context"
	"errors"
	"fmt"
	"maps"
)

const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)

var (
	ErrUnknownChannel      = errors.New("notification: canal desconhecido")
	ErrEmptyExternal       = errors.New("notification: external_id vazio")
	ErrEmptyText           = errors.New("notification: texto vazio")
	ErrTemplateUnsupported = errors.New("notification: template nao suportado pelo canal")
	ErrEmptyTemplateName   = errors.New("notification: template_name vazio")
)

type ChannelGateway interface {
	SendText(ctx context.Context, channel, externalID, text string) error
	SendActivationTemplate(ctx context.Context, channel, externalID, templateName, token string) (messageID string, err error)
}

type SendFunc func(ctx context.Context, externalID, text string) error

type TemplateSendFunc func(ctx context.Context, externalID, templateName, token string) (string, error)

type ChannelSenders struct {
	Text     SendFunc
	Template TemplateSendFunc
}

type MultiChannelGateway struct {
	senders map[string]ChannelSenders
}

func NewMultiChannelGateway(senders map[string]ChannelSenders) *MultiChannelGateway {
	dst := make(map[string]ChannelSenders, len(senders))
	maps.Copy(dst, senders)
	return &MultiChannelGateway{senders: dst}
}

func (g *MultiChannelGateway) SendText(ctx context.Context, channel, externalID, text string) error {
	if externalID == "" {
		return ErrEmptyExternal
	}
	if text == "" {
		return ErrEmptyText
	}
	set, ok := g.senders[channel]
	if !ok || set.Text == nil {
		return fmt.Errorf("%w: %q", ErrUnknownChannel, channel)
	}
	if err := set.Text(ctx, externalID, text); err != nil {
		return fmt.Errorf("notification: send text %s: %w", channel, err)
	}
	return nil
}

func (g *MultiChannelGateway) SendActivationTemplate(ctx context.Context, channel, externalID, templateName, token string) (string, error) {
	if externalID == "" {
		return "", ErrEmptyExternal
	}
	if templateName == "" {
		return "", ErrEmptyTemplateName
	}
	set, ok := g.senders[channel]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownChannel, channel)
	}
	if set.Template == nil {
		return "", fmt.Errorf("%w: %q", ErrTemplateUnsupported, channel)
	}
	messageID, err := set.Template(ctx, externalID, templateName, token)
	if err != nil {
		return "", fmt.Errorf("notification: send template %s: %w", channel, err)
	}
	return messageID, nil
}
