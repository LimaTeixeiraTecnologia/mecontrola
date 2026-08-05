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

type audioInboundProcessor interface {
	Execute(ctx context.Context, in usecases.AudioInboundInput) (usecases.AudioInboundResult, error)
	MarkDispatched(ctx context.Context, wamid string) error
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

type MessageDedupStore interface {
	InsertIfAbsent(ctx context.Context, consumer, messageID string) (bool, error)
	Delete(ctx context.Context, consumer, messageID string) error
}

const whatsAppInboundConsumerName = "whatsapp_inbound"

var errSendReply = errors.New("send reply")

type whatsAppInboundPayload struct {
	UserID        string `json:"user_id"`
	Peer          string `json:"peer"`
	Text          string `json:"text"`
	MessageID     string `json:"message_id"`
	MessageType   string `json:"message_type,omitempty"`
	AudioMediaID  string `json:"audio_media_id,omitempty"`
	AudioMimeType string `json:"audio_mime_type,omitempty"`
	AudioSHA256   string `json:"audio_sha256,omitempty"`
	AudioVoice    bool   `json:"audio_voice,omitempty"`
}

type inboundMessageKind string

const (
	inboundMessageKindText  inboundMessageKind = "text"
	inboundMessageKindAudio inboundMessageKind = "audio"
)

func (k inboundMessageKind) IsAudio() bool {
	return k == inboundMessageKindAudio
}

func parseInboundMessageKind(raw string) inboundMessageKind {
	if raw == string(inboundMessageKindAudio) {
		return inboundMessageKindAudio
	}
	return inboundMessageKindText
}

const defaultAudioDisabledReply = "entrada por áudio ainda não está disponível por aqui, pode digitar sua mensagem?"

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

func WithMessageDedup(store MessageDedupStore) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.dedup = store
	}
}

func WithAudioProcessor(p audioInboundProcessor) ConsumerOption {
	return func(c *WhatsAppInboundConsumer) {
		c.audioProcessor = p
	}
}

type WhatsAppInboundConsumer struct {
	handleInbound     handleInboundUseCase
	gateway           whatsAppTextSender
	o11y              observability.Observability
	resolveOnboarding onboardingResolver
	resumeDispatcher  resumeDispatcherResolver
	dedup             MessageDedupStore
	audioProcessor    audioInboundProcessor
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

	kind := parseInboundMessageKind(p.MessageType)

	if err := validateInboundPayload(p, kind); err != nil {
		c.decodeFails.Add(ctx, 1, observability.String("channel", "whatsapp"))
		return err
	}

	if c.dedup != nil && p.MessageID != "" {
		inserted, err := c.dedup.InsertIfAbsent(ctx, whatsAppInboundConsumerName, p.MessageID)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("agents.consumer.whatsapp_inbound: dedup: %w", err)
		}
		if !inserted {
			c.inboundTotal.Add(ctx, 1,
				observability.String("channel", "whatsapp"),
				observability.String("outcome", "deduplicated"),
			)
			c.o11y.Logger().Info(ctx, "agents.consumer.whatsapp_inbound: mensagem duplicada ignorada",
				observability.String("message_id", p.MessageID),
			)
			return nil
		}
	}

	if c.inboundTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.inboundTimeout)
		defer cancel()
	}

	if kind.IsAudio() {
		return c.handleAudioInbound(ctx, span, p)
	}

	if handled, err := c.tryResume(ctx, span, p); handled || err != nil {
		return c.compensateDedup(ctx, p.MessageID, err)
	}

	return c.compensateDedup(ctx, p.MessageID, c.handleAgentInbound(ctx, span, p))
}

func validateInboundPayload(p whatsAppInboundPayload, kind inboundMessageKind) error {
	if p.UserID == "" || p.Peer == "" {
		return fmt.Errorf("agents.consumer.whatsapp_inbound: payload incompleto: user_id=%q peer=%q text=%q", p.UserID, p.Peer, p.Text)
	}
	if kind.IsAudio() {
		if p.AudioMediaID == "" || p.AudioMimeType == "" || p.AudioSHA256 == "" {
			return fmt.Errorf("agents.consumer.whatsapp_inbound: payload de audio incompleto: media_id=%q mime_type=%q sha256=%q",
				p.AudioMediaID, p.AudioMimeType, p.AudioSHA256)
		}
		return nil
	}
	if p.Text == "" {
		return fmt.Errorf("agents.consumer.whatsapp_inbound: payload incompleto: user_id=%q peer=%q text=%q", p.UserID, p.Peer, p.Text)
	}
	return nil
}

func (c *WhatsAppInboundConsumer) handleAudioInbound(ctx context.Context, span observability.Span, p whatsAppInboundPayload) error {
	if c.audioProcessor == nil {
		return c.sendReply(ctx, p.Peer, defaultAudioDisabledReply, "audio_disabled")
	}

	result, err := c.audioProcessor.Execute(ctx, usecases.AudioInboundInput{
		WAMID:    p.MessageID,
		UserID:   p.UserID,
		Peer:     p.Peer,
		MediaID:  p.AudioMediaID,
		MimeType: p.AudioMimeType,
		SHA256:   p.AudioSHA256,
		Voice:    p.AudioVoice,
	})
	if err != nil {
		c.recordInboundTimeout(ctx, err)
		c.inboundTotal.Add(ctx, 1,
			observability.String("channel", "whatsapp"),
			observability.String("outcome", "audio_error"),
		)
		span.RecordError(err)
		return fmt.Errorf("agents.consumer.whatsapp_inbound: process audio: %w", err)
	}

	if !result.Outcome.AllowsHandleInbound() {
		return c.sendReply(ctx, p.Peer, result.ReplyText, "audio_"+result.Outcome.String())
	}

	textPayload := p
	textPayload.Text = result.CanonicalText

	dispatchErr := func() error {
		if handled, herr := c.tryResume(ctx, span, textPayload); handled || herr != nil {
			return herr
		}
		return c.handleAgentInbound(ctx, span, textPayload)
	}()

	dispatchErr = c.compensateDedup(ctx, p.MessageID, dispatchErr)
	if dispatchErr == nil {
		c.markAudioDispatched(ctx, p.MessageID)
	}
	return dispatchErr
}

func (c *WhatsAppInboundConsumer) markAudioDispatched(ctx context.Context, wamid string) {
	if c.audioProcessor == nil || wamid == "" {
		return
	}
	if err := c.audioProcessor.MarkDispatched(ctx, wamid); err != nil {
		c.o11y.Logger().Warn(ctx, "agents.consumer.whatsapp_inbound: marcar audio dispatched falhou",
			observability.String("outcome", "mark_dispatched_failed"),
			observability.Error(err),
		)
	}
}

func (c *WhatsAppInboundConsumer) compensateDedup(ctx context.Context, messageID string, processErr error) error {
	if processErr == nil || c.dedup == nil || messageID == "" || errors.Is(processErr, errSendReply) {
		return processErr
	}
	if err := c.dedup.Delete(ctx, whatsAppInboundConsumerName, messageID); err != nil {
		return errors.Join(processErr, fmt.Errorf("agents.consumer.whatsapp_inbound: compensar dedup: %w", err))
	}
	return processErr
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
		return c.sendReply(ctx, p.Peer, outcome.Content, "not_confirmed")
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
		return fmt.Errorf("agents.consumer.whatsapp_inbound: %w: %w", errSendReply, err)
	}

	c.inboundTotal.Add(ctx, 1,
		observability.String("channel", "whatsapp"),
		observability.String("outcome", deliveredOutcome),
	)
	return nil
}
