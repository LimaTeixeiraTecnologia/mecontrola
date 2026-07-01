package consumers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/formatting"
)

const mecontrolaAgentID = "mecontrola-agent"

type handleInboundUseCase interface {
	Execute(ctx context.Context, in input.InboundInput) (agent.Outcome, error)
}

type onboardingResolver interface {
	Execute(ctx context.Context, userID, message string) (usecases.OnboardingResult, error)
}

type destructiveConfirmResolver interface {
	Continue(ctx context.Context, userID, message string) (bool, string, error)
}

type whatsAppTextSender interface {
	SendTextMessage(ctx context.Context, toE164, text string) error
}

type whatsAppInboundPayload struct {
	UserID    string `json:"user_id"`
	Peer      string `json:"peer"`
	Text      string `json:"text"`
	MessageID string `json:"message_id"`
}

type ConsumerOption func(*WhatsAppInboundConsumer)

func WithOnboardingResolver(r onboardingResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.resolveOnboarding = r
	}
}

func WithDestructiveConfirmResolver(r destructiveConfirmResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.continueDestructive = r
	}
}

type WhatsAppInboundConsumer struct {
	handleInbound       handleInboundUseCase
	gateway             whatsAppTextSender
	o11y                observability.Observability
	resolveOnboarding   onboardingResolver
	continueDestructive destructiveConfirmResolver
	inboundTotal        observability.Counter
	decodeFails         observability.Counter
}

func NewWhatsAppInboundConsumer(
	handleInbound handleInboundUseCase,
	gateway whatsAppTextSender,
	o11y observability.Observability,
	opts ...ConsumerOption,
) *WhatsAppInboundConsumer {
	inboundTotal := o11y.Metrics().Counter(
		"agents_whatsapp_inbound_total",
		"Total de mensagens inbound processadas pelo consumer de WhatsApp",
		"1",
	)
	decodeFails := o11y.Metrics().Counter(
		"agents_whatsapp_inbound_decode_failed_total",
		"Total de falhas de decode do consumer de WhatsApp inbound",
		"1",
	)
	c := &WhatsAppInboundConsumer{
		handleInbound: handleInbound,
		gateway:       gateway,
		o11y:          o11y,
		inboundTotal:  inboundTotal,
		decodeFails:   decodeFails,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *WhatsAppInboundConsumer) Handle(ctx context.Context, event events.Event) error {
	ctx, span := c.o11y.Tracer().Start(ctx, "agents.consumer.whatsapp_inbound.handle")
	defer span.End()

	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agents.consumer.whatsapp_inbound: unexpected payload type %T", rawPayload)
	}

	var p whatsAppInboundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1, observability.String("channel", "whatsapp"))
		return fmt.Errorf("agents.consumer.whatsapp_inbound: deserializar payload: %w", err)
	}

	if p.UserID == "" || p.Peer == "" || p.Text == "" {
		c.decodeFails.Add(ctx, 1, observability.String("channel", "whatsapp"))
		return fmt.Errorf("agents.consumer.whatsapp_inbound: payload incompleto: user_id=%q peer=%q text=%q", p.UserID, p.Peer, p.Text)
	}

	if c.continueDestructive != nil {
		handled, reply, err := c.continueDestructive.Continue(ctx, p.UserID, p.Text)
		if err != nil {
			c.inboundTotal.Add(ctx, 1,
				observability.String("channel", "whatsapp"),
				observability.String("outcome", "destructive_confirm_error"),
			)
			span.RecordError(err)
			return fmt.Errorf("agents.consumer.whatsapp_inbound: confirmacao destrutiva: %w", err)
		}
		if handled {
			return c.sendReply(ctx, p.Peer, reply)
		}
	}

	if c.resolveOnboarding != nil {
		result, err := c.resolveOnboarding.Execute(ctx, p.UserID, p.Text)
		if err != nil {
			c.inboundTotal.Add(ctx, 1,
				observability.String("channel", "whatsapp"),
				observability.String("outcome", "onboarding_error"),
			)
			span.RecordError(err)
			return fmt.Errorf("agents.consumer.whatsapp_inbound: onboarding: %w", err)
		}
		if result.Handled {
			return c.sendReply(ctx, p.Peer, result.Message)
		}
	}

	outcome, err := c.handleInbound.Execute(ctx, input.InboundInput{
		ResourceID: p.UserID,
		ThreadID:   p.Peer,
		AgentID:    mecontrolaAgentID,
		Message:    p.Text,
		MessageID:  p.MessageID,
	})
	if err != nil {
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "error"),
		)
		span.RecordError(err)
		return fmt.Errorf("agents.consumer.whatsapp_inbound: handle inbound: %w", err)
	}

	return c.sendReply(ctx, p.Peer, outcome.Content)
}

func (c *WhatsAppInboundConsumer) sendReply(ctx context.Context, peer, content string) error {
	if content == "" {
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "no_reply"),
		)
		return nil
	}

	content = formatting.NormalizeOutboundText(content)

	if err := c.gateway.SendTextMessage(ctx, peer, content); err != nil {
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "send_error"),
		)
		return fmt.Errorf("agents.consumer.whatsapp_inbound: send reply: %w", err)
	}

	c.inboundTotal.Add(ctx, 1,
		observability.String("channel", "whatsapp"),
		observability.String("outcome", "success"),
	)
	return nil
}
