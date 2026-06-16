package adapters

import (
	"context"
	"fmt"
	"strconv"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
)

type TelegramSender struct {
	gateway *outbound.Gateway
}

func NewTelegramSender(gateway *outbound.Gateway) *TelegramSender {
	return &TelegramSender{gateway: gateway}
}

func (s *TelegramSender) SendText(ctx context.Context, externalID, text string) error {
	chatID, err := strconv.ParseInt(externalID, 10, 64)
	if err != nil {
		return fmt.Errorf("notification.telegram: external_id inválido %q: %w", externalID, err)
	}
	if s.gateway == nil {
		return fmt.Errorf("notification.telegram: gateway nao configurado")
	}
	if err := s.gateway.SendTextMessage(ctx, chatID, text); err != nil {
		return fmt.Errorf("notification.telegram: send: %w", err)
	}
	return nil
}

func (s *TelegramSender) AsChannelSenders() notification.ChannelSenders {
	return notification.ChannelSenders{
		Text: s.SendText,
	}
}
