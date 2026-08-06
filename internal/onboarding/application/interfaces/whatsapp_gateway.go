package interfaces

import "context"

type WhatsAppGateway interface {
	SendActivationTemplate(ctx context.Context, toE164, templateName, token string) (wamid string, err error)
	SendTextMessage(ctx context.Context, toE164, text string) error
}
