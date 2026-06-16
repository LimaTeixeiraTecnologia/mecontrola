package interfaces

import "context"

type OutreachChannelGateway interface {
	SendActivationTemplate(ctx context.Context, channel, externalID, templateName, token string) (messageID string, err error)
	SendText(ctx context.Context, channel, externalID, text string) error
}
