package agent

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
)

type whatsAppGateway interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type StubAgent struct {
	waGateway whatsAppGateway
	templates map[string]string
	o11y      observability.Observability
}

func NewStubAgent(
	waGateway whatsAppGateway,
	templates map[string]string,
	o11y observability.Observability,
) *StubAgent {
	return &StubAgent{
		waGateway: waGateway,
		templates: templates,
		o11y:      o11y,
	}
}

func (s *StubAgent) HandleMessage(ctx context.Context, msg payload.Message) error {
	p, _ := auth.FromContext(ctx)
	tmpl, ok := s.templates["agent_stub_received"]
	if !ok {
		return fmt.Errorf("agent.stub: template agent_stub_received not configured")
	}

	s.o11y.Logger().Info(ctx, "agent_stub_invoked",
		observability.String("user_id", p.UserID.String()),
		observability.String("wa_id_masked", payload.MaskMobile(msg.From)),
	)

	if err := s.waGateway.SendTextMessage(ctx, msg.From, tmpl); err != nil {
		s.o11y.Logger().Warn(ctx, "agent_stub_send_failed",
			observability.String("wa_id_masked", payload.MaskMobile(msg.From)),
			observability.Error(err),
		)
		return fmt.Errorf("agent.stub: send template: %w", err)
	}
	return nil
}
