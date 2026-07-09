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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
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

type pendingEntryContinuerResolver interface {
	Continue(ctx context.Context, userID, peer, message, messageID string) (workflows.PendingEntryResult, error)
}

type destructiveConfirmResolver interface {
	Continue(ctx context.Context, userID, message string) (bool, string, error)
}

type cardCreateResolver interface {
	Continue(ctx context.Context, resourceID, peer, message, messageID string) (bool, string, error)
}

type budgetCreationResolver interface {
	Continue(ctx context.Context, resourceID, text, messageID string) (bool, string, error)
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

func WithPendingEntryContinuer(r pendingEntryContinuerResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.continuePendingEntry = r
	}
}

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

func WithCardCreateResolver(r cardCreateResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.continueCardCreate = r
	}
}

func WithBudgetCreationResolver(r budgetCreationResolver) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.continueBudgetCreation = r
	}
}

const defaultInboundTimeout = 60 * time.Second

func WithInboundTimeout(d time.Duration) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.inboundTimeout = d
	}
}

type WhatsAppInboundConsumer struct {
	handleInbound          handleInboundUseCase
	gateway                whatsAppTextSender
	o11y                   observability.Observability
	continuePendingEntry   pendingEntryContinuerResolver
	resolveOnboarding      onboardingResolver
	continueDestructive    destructiveConfirmResolver
	continueCardCreate     cardCreateResolver
	continueBudgetCreation budgetCreationResolver
	inboundTimeout         time.Duration
	inboundTotal           observability.Counter
	decodeFails            observability.Counter
	timeoutTotal           observability.Counter
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

	if handled, err := c.tryResumeChain(ctx, span, p); handled || err != nil {
		return err
	}

	return c.handleAgentInbound(ctx, span, p)
}

func (c *WhatsAppInboundConsumer) tryResumeChain(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	resumers := []func(context.Context, observability.Span, whatsAppInboundPayload) (bool, error){
		c.tryContinuePendingEntry,
		c.tryContinueDestructive,
		c.tryContinueCardCreate,
		c.tryContinueBudgetCreation,
		c.tryResolveOnboarding,
	}
	for _, resume := range resumers {
		if handled, err := resume(ctx, span, p); handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) tryContinuePendingEntry(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.continuePendingEntry == nil {
		return false, nil
	}
	result, err := c.continuePendingEntry.Continue(ctx, p.UserID, p.Peer, p.Text, p.MessageID)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "pending_entry_error"),
		)
		span.RecordError(err)
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: pendencia de lancamento: %w", err)
	}
	if result.Handled {
		return true, c.sendReply(ctx, p.Peer, result.Message, "success")
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) tryContinueDestructive(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.continueDestructive == nil {
		return false, nil
	}
	handled, reply, err := c.continueDestructive.Continue(ctx, p.UserID, p.Text)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "destructive_confirm_error"),
		)
		span.RecordError(err)
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: confirmacao destrutiva: %w", err)
	}
	if handled {
		return true, c.sendReply(ctx, p.Peer, reply, "success")
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) tryContinueCardCreate(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.continueCardCreate == nil {
		return false, nil
	}
	handled, reply, err := c.continueCardCreate.Continue(ctx, p.UserID, p.Peer, p.Text, p.MessageID)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "card_create_error"),
		)
		span.RecordError(err)
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: confirmacao de cadastro de cartao: %w", err)
	}
	if handled {
		return true, c.sendReply(ctx, p.Peer, reply, "success")
	}
	return false, nil
}

func (c *WhatsAppInboundConsumer) tryContinueBudgetCreation(ctx context.Context, span observability.Span, p whatsAppInboundPayload) (bool, error) {
	if c.continueBudgetCreation == nil {
		return false, nil
	}
	handled, reply, err := c.continueBudgetCreation.Continue(ctx, p.UserID, p.Text, p.MessageID)
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "budget_creation_error"),
		)
		span.RecordError(err)
		return false, fmt.Errorf("agents.consumer.whatsapp_inbound: criacao de orcamento: %w", err)
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
