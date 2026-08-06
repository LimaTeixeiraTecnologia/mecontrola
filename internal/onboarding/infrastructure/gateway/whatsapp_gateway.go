package gateway

import (
	"context"
	"errors"
	"fmt"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/http/client/meta"
)

type WhatsAppGateway struct {
	client *meta.Client
}

func NewWhatsAppGateway(client *meta.Client) *WhatsAppGateway {
	return &WhatsAppGateway{client: client}
}

func (g *WhatsAppGateway) SendActivationTemplate(ctx context.Context, toE164, templateName, token string) (string, error) {
	components := []any{
		map[string]any{
			"type": "body",
			"parameters": []any{
				map[string]any{
					"type": "text",
					"text": "ATIVAR " + token,
				},
			},
		},
	}
	wamid, err := g.client.SendTemplate(ctx, toE164, templateName, "pt_BR", components)
	if err != nil {
		return "", g.classifyError(err, "enviar template de ativação")
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
