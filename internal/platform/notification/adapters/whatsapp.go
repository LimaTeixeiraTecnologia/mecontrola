package adapters

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type WhatsAppTextSender interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type WhatsAppTemplateSender interface {
	SendActivationTemplate(ctx context.Context, toE164, templateName, token string) (string, error)
}

type WhatsAppGatewayBridge interface {
	WhatsAppTextSender
	WhatsAppTemplateSender
}

type WhatsAppSender struct {
	bridge WhatsAppGatewayBridge
}

func NewWhatsAppSender(bridge WhatsAppGatewayBridge) *WhatsAppSender {
	return &WhatsAppSender{bridge: bridge}
}

func (s *WhatsAppSender) SendText(ctx context.Context, externalID, text string) error {
	if err := s.bridge.SendTextMessage(ctx, externalID, text); err != nil {
		return fmt.Errorf("notification.whatsapp: send text: %w", err)
	}
	return nil
}

func (s *WhatsAppSender) SendTemplate(ctx context.Context, externalID, templateName, token string) (string, error) {
	messageID, err := s.bridge.SendActivationTemplate(ctx, externalID, templateName, token)
	if err != nil {
		return "", fmt.Errorf("notification.whatsapp: send template: %w", err)
	}
	return messageID, nil
}

func (s *WhatsAppSender) AsChannelSenders() notification.ChannelSenders {
	return notification.ChannelSenders{
		Text:     s.SendText,
		Template: s.SendTemplate,
	}
}
