package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

const (
	NotifyInvoiceDueOutcomeSent          = "sent"
	NotifyInvoiceDueOutcomeAlreadySent   = "already_sent"
	NotifyInvoiceDueOutcomeNoChannel     = "no_channel"
	NotifyInvoiceDueOutcomeChannelFailed = "channel_failed"
	NotifyInvoiceDueOutcomeResolverError = "resolver_error"
	NotifyInvoiceDueOutcomeRecordMissing = "record_missing"
)

var ErrInvoiceDueChannelUnavailable = errors.New("card: canal nao disponivel para notificacao")

type NotifyInvoiceDueInput struct {
	UserID       uuid.UUID
	CardID       uuid.UUID
	CardNickname string
	DueDate      time.Time
	DaysUntil    int
}

type NotifyInvoiceDueResult struct {
	Outcome string
	Channel string
}

type NotifyInvoiceDue struct {
	repo            interfaces.InvoiceDueAlertSentRepository
	channelResolver interfaces.UserChannelResolver
	channelGateway  notification.ChannelGateway
	location        *time.Location
	o11y            observability.Observability
	delivered       observability.Counter
}

func NewNotifyInvoiceDue(
	repo interfaces.InvoiceDueAlertSentRepository,
	channelResolver interfaces.UserChannelResolver,
	channelGateway notification.ChannelGateway,
	location *time.Location,
	o11y observability.Observability,
) *NotifyInvoiceDue {
	delivered := o11y.Metrics().Counter(
		"card_invoice_due_alert_delivered_total",
		"Total de notificacoes de vencimento de fatura despachadas por canal e outcome",
		"1",
	)
	return &NotifyInvoiceDue{
		repo:            repo,
		channelResolver: channelResolver,
		channelGateway:  channelGateway,
		location:        location,
		o11y:            o11y,
		delivered:       delivered,
	}
}

func (uc *NotifyInvoiceDue) Execute(ctx context.Context, in NotifyInvoiceDueInput) (NotifyInvoiceDueResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "card.usecase.notify_invoice_due")
	defer span.End()

	notified, err := uc.repo.IsNotified(ctx, in.UserID, in.CardID, in.DueDate)
	if err != nil {
		if errors.Is(err, interfaces.ErrInvoiceDueAlertRecordMissing) {
			uc.recordOutcome(ctx, "unknown", NotifyInvoiceDueOutcomeRecordMissing)
			return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeRecordMissing}, nil
		}
		uc.recordOutcome(ctx, "unknown", NotifyInvoiceDueOutcomeRecordMissing)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeRecordMissing}, fmt.Errorf("card: notify invoice due: check sent: %w", err)
	}
	if notified {
		uc.recordOutcome(ctx, "unknown", NotifyInvoiceDueOutcomeAlreadySent)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeAlreadySent}, nil
	}

	pref, ok, err := uc.channelResolver.ResolvePreferred(ctx, in.UserID)
	if err != nil {
		uc.recordOutcome(ctx, "unknown", NotifyInvoiceDueOutcomeResolverError)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeResolverError}, fmt.Errorf("card: notify invoice due: resolve channel: %w", err)
	}
	if !ok {
		uc.recordOutcome(ctx, "unknown", NotifyInvoiceDueOutcomeNoChannel)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeNoChannel}, nil
	}

	text := uc.renderText(in)

	updated, err := uc.repo.MarkNotified(ctx, in.UserID, in.CardID, in.DueDate, pref.Channel, time.Now().UTC())
	if err != nil {
		uc.recordOutcome(ctx, pref.Channel, NotifyInvoiceDueOutcomeChannelFailed)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("card: notify invoice due: mark notified: %w", err)
	}
	if !updated {
		uc.recordOutcome(ctx, pref.Channel, NotifyInvoiceDueOutcomeAlreadySent)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeAlreadySent, Channel: pref.Channel}, nil
	}

	if err := uc.channelGateway.SendText(ctx, pref.Channel, pref.ExternalID, text); err != nil {
		uc.recordOutcome(ctx, pref.Channel, NotifyInvoiceDueOutcomeChannelFailed)
		return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("card: notify invoice due: send: %w: %w", ErrInvoiceDueChannelUnavailable, err)
	}

	uc.recordOutcome(ctx, pref.Channel, NotifyInvoiceDueOutcomeSent)
	return NotifyInvoiceDueResult{Outcome: NotifyInvoiceDueOutcomeSent, Channel: pref.Channel}, nil
}

func (uc *NotifyInvoiceDue) renderText(in NotifyInvoiceDueInput) string {
	loc := uc.location
	if loc == nil {
		loc = time.UTC
	}
	due := in.DueDate.In(loc).Format("02/01")
	name := in.CardNickname
	if name == "" {
		name = "seu cartao"
	}
	switch {
	case in.DaysUntil <= 0:
		return fmt.Sprintf("Sua fatura do cartao %s vence hoje (%s).", name, due)
	case in.DaysUntil == 1:
		return fmt.Sprintf("Sua fatura do cartao %s vence amanha (%s).", name, due)
	default:
		return fmt.Sprintf("Sua fatura do cartao %s vence em %d dias (%s).", name, in.DaysUntil, due)
	}
}

func (uc *NotifyInvoiceDue) recordOutcome(ctx context.Context, channel, outcome string) {
	uc.delivered.Add(ctx, 1,
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
}
