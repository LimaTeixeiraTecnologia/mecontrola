package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type WhatsAppGateway interface {
	SendActivationTemplate(ctx context.Context, toE164, templateName, token string) (wamid string, err error)
	SendTemplateMessage(ctx context.Context, toE164 string, message notification.TemplateMessage) (wamid string, err error)
	SendTextMessage(ctx context.Context, toE164, text string) error
}
