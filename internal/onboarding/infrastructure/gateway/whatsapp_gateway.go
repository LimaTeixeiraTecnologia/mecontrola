package gateway

import (
	"context"
	"errors"
	"fmt"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type WhatsAppGateway struct {
	client *meta.Client
}

func NewWhatsAppGateway(client *meta.Client) *WhatsAppGateway {
	return &WhatsAppGateway{client: client}
}

func (g *WhatsAppGateway) SendActivationTemplate(ctx context.Context, toE164, templateName, token string) (string, error) {
	wamid, err := g.SendTemplateMessage(ctx, toE164, notification.TemplateMessage{
		Channel:      notification.ChannelWhatsApp,
		ExternalID:   toE164,
		TemplateName: templateName,
		LanguageCode: "pt_BR",
		Components: []notification.TemplateComponent{
			{
				Type: notification.TemplateComponentBody,
				Parameters: []notification.TemplateParameter{
					{Type: notification.TemplateParameterText, Text: "ATIVAR " + token},
				},
			},
		},
	})
	if err != nil {
		return "", g.classifyError(err, "enviar template de ativação")
	}
	return wamid, nil
}

func (g *WhatsAppGateway) SendTemplateMessage(ctx context.Context, toE164 string, message notification.TemplateMessage) (string, error) {
	components := make([]any, 0, len(message.Components))
	for _, component := range message.Components {
		parameters := make([]any, 0, len(component.Parameters))
		for _, parameter := range component.Parameters {
			parameters = append(parameters, map[string]any{
				"type": string(parameter.Type),
				"text": parameter.Text,
			})
		}
		entry := map[string]any{
			"type":       string(component.Type),
			"parameters": parameters,
		}
		if component.Type == notification.TemplateComponentButton {
			entry["sub_type"] = string(component.SubType)
			entry["index"] = component.Index
		}
		components = append(components, entry)
	}
	wamid, err := g.client.SendTemplate(ctx, toE164, message.TemplateName, message.LanguageCode, components)
	if err != nil {
		return "", g.classifyError(err, "enviar template")
	}
	return wamid, nil
}

func (g *WhatsAppGateway) SendTextMessage(ctx context.Context, toE164, text string) error {
	_, err := g.client.SendText(ctx, toE164, text)
	if err != nil {
		return g.classifyError(err, "enviar mensagem de texto")
	}
	return nil
}

func (g *WhatsAppGateway) classifyError(err error, op string) error {
	switch {
	case errors.Is(err, meta.ErrMetaBadRequest) || errors.Is(err, meta.ErrMetaAuth):
		return fmt.Errorf("onboarding/gateway: %s: %w: %w", op, application.ErrWhatsAppClientError, err)
	case errors.Is(err, meta.ErrMetaServer):
		return fmt.Errorf("onboarding/gateway: %s: %w: %w", op, application.ErrWhatsAppServerError, err)
	default:
		return fmt.Errorf("onboarding/gateway: %s: %w", op, err)
	}
}
