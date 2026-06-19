package bootstrap

import (
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	notificationadapters "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification/adapters"
	tgoutbound "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/outbound"
)

func BuildChannelGateway(cfg *configs.Config, o11y observability.Observability, whatsappBridge notificationadapters.WhatsAppGatewayBridge) (notification.ChannelGateway, error) {
	senders := map[string]notification.ChannelSenders{}
	if whatsappBridge != nil {
		senders[notification.ChannelWhatsApp] = notificationadapters.NewWhatsAppSender(whatsappBridge).AsChannelSenders()
	}
	if cfg.TelegramConfig.Enabled {
		gateway, err := tgoutbound.NewSharedGateway(o11y, tgoutbound.FactoryConfig{
			APIBaseURL: cfg.TelegramConfig.APIBaseURL,
			BotToken:   cfg.TelegramConfig.BotToken,
			Timeout:    cfg.TelegramConfig.OutboundTimeout,
		})
		if err != nil {
			return nil, fmt.Errorf("bootstrap: build telegram sender: %w", err)
		}
		senders[notification.ChannelTelegram] = notificationadapters.NewTelegramSender(gateway).AsChannelSenders()
	}
	return notification.NewMultiChannelGateway(senders), nil
}
