package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	gotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

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
	Execute(ctx context.Context, userID, peer, message string) (usecases.OnboardingResult, error)
}

type resumeDispatcherResolver interface {
	Continue(ctx context.Context, resourceID, threadID, message, messageID string) (bool, string, error)
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

func WithResumeDispatcher(r resumeDispatcherResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.resumeDispatcher = r
	}
}

const defaultInboundTimeout = 60 * time.Second

func WithInboundTimeout(d time.Duration) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.inboundTimeout = d
	}
}

type WhatsAppInboundConsumer struct {
	handleInbound     handleInboundUseCase
	gateway           whatsAppTextSender
	o11y              observability.Observability
	resolveOnboarding onboardingResolver
	resumeDispatcher  resumeDispatcherResolver
	inboundTimeout    time.Duration
	inboundTotal      observability.Counter
	decodeFails       observability.Counter
	timeoutTotal      observability.Counter
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
	timeoutTotal := o11y.Metrics().Counter(
		"agents_whatsapp_inbound_timeout_total",
		"Total de timeouts de LLM/tool no consumer de WhatsApp inbound",
		"1",
	)
	c := &WhatsAppInboundConsumer{
		handleInbound: handleInbound,
		gateway:       gateway,
		o11y:          o11y,
		inboundTotal:  inboundTotal,
		decodeFails:   decodeFails,
		timeoutTotal:  timeoutTotal,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.inboundTimeout <= 0 {
		c.inboundTimeout = defaultInboundTimeout
	}
	return c
}

func (c *WhatsAppInboundConsumer) Handle(ctx context.Context, event events.Event) error {
	rawPayload := event.GetPayload()
	env, ok := rawPayload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("agents.consumer.whatsapp_inbound: unexpected payload type %T", rawPayload)
	}

	ctx = gotel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(env.Metadata))

	ctx, span := c.o11y.Tracer().Start(ctx, "agents.consumer.whatsapp_inbound.handle")
	defer span.End()

	var p whatsAppInboundPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		c.decodeFails.Add(ctx, 1, observability.String("channel", "whatsapp"))
		return fmt.Errorf("agents.consumer.whatsapp_inbound: deserializar payload: %w", err)
	}

	if p.UserID == "" || p.Peer == "" || p.Text == "" || p.MessageID == "" {
		c.decodeFails.Add(ctx, 1, observability.String("channel", "whatsapp"))
		return fmt.Errorf("agents.consumer.whatsapp_inbound: payload incompleto: user_id=%q peer=%q text=%q message_id=%q", p.UserID, p.Peer, p.Text, p.MessageID)
	}

	if c.inboundTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.inboundTimeout)
		defer cancel()
	}

	if handled, err := c.tryResume(ctx, span, p); handled || err != nil {
		return err
	}

	return c.handleAgentInbound(ctx, span, p)
}

func (c *WhatsAppInboundConsumer) tryResume(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if handled, err := c.tryDispatchResume(ctx, span, p); handled || err != nil {
		return handled, err
	}
	return c.tryResolveOnboarding(ctx, span, p)
}

func (c *WhatsAppInboundConsumer) tryDispatchResume(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.resumeDispatcher == nil {
		return false, nil
	}
	handled, reply, err := c.resumeDispatcher.Continue(ctx, p.UserID, p.Peer, p.Text, p.MessageID)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "resume_error"),
		)
		span.RecordError(err)
		if handled && reply != "" && !errors.Is(err, context.DeadlineExceeded) {
			return true, c.sendReply(ctx, p.Peer, reply, "resume_error")
		}
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: resume: %w", err)
	}
	if handled {
		return true, c.sendReply(ctx, p.Peer, reply, "success")
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) tryResolveOnboarding(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.resolveOnboarding == nil {
		return false, nil
	}
	result, err := c.resolveOnboarding.Execute(ctx, p.UserID, p.Peer, p.Text)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "onboarding_error"),
		)
		span.RecordError(err)
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: onboarding: %w", err)
	}
	if result.Handled {
		return true, c.sendReply(ctx, p.Peer, result.Message, "success")
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) recordInboundTimeout(ctx context.Context, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		c.timeoutTotal.Add(ctx, 1, observability.String("channel", "whatsapp"))
	}
}

func (c *WhatsAppInboundConsumer) handleAgentInbound(ctx context.Context, span observability.Span, p whatsAppInboundPayload) error {
	outcome, err := c.handleInbound.Execute(ctx, input.InboundInput{
		ResourceID: p.UserID,
		ThreadID:   p.Peer,
		AgentID:    mecontrolaAgentID,
		Message:    p.Text,
		MessageID:  p.MessageID,
	})
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "error"),
		)
		span.RecordError(err)
		return fmt.Errorf("agents.consumer.whatsapp_inbound: handle inbound: %w", err)
	}
	if !outcome.Succeeded() {
		return c.sendReply(ctx, p.Peer, fallbackReply, "not_confirmed")
	}
	return c.sendReply(ctx, p.Peer, outcome.Content, "success")
}

const fallbackReply = "não consegui concluir agora, pode repetir?"

func (c *WhatsAppInboundConsumer) sendReply(ctx context.Context, peer, content, deliveredOutcome string) error {
	content = formatting.NormalizeOutboundText(content)
	if strings.TrimSpace(content) == "" {
		deliveredOutcome = "no_reply"
		content = fallbackReply
	}

	if err := c.gateway.SendTextMessage(ctx, peer, content); err != nil {
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "send_error"),
		)
		return fmt.Errorf("agents.consumer.whatsapp_inbound: send reply: %w", err)
	}

	c.inboundTotal.Add(ctx, 1,
		observability.String("channel", "whatsapp"),
		observability.String("outcome", deliveredOutcome),
	)
	return nil
}
