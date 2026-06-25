package bootstrap

import (
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	notificationadapters "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification/adapters"
)

func BuildChannelGateway(cfg *configs.Config, o11y observability.Observability, whatsappBridge notificationadapters.WhatsAppGatewayBridge) (notification.ChannelGateway, error) {
	senders := map[string]notification.ChannelSenders{}
	if whatsappBridge != nil {
		senders[notification.ChannelWhatsApp] = notificationadapters.NewWhatsAppSender(whatsappBridge).AsChannelSenders()
	}
	return notification.NewMultiChannelGateway(senders), nil
}
