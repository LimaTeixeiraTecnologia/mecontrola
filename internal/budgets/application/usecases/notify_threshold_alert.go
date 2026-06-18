package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

const (
	NotifyOutcomeSent          = "sent"
	NotifyOutcomeAlreadySent   = "already_sent"
	NotifyOutcomeNoChannel     = "no_channel"
	NotifyOutcomeChannelFailed = "channel_failed"
	NotifyOutcomeResolverError = "resolver_error"
	NotifyOutcomeRecordMissing = "record_missing"
)

var ErrNotifyChannelUnavailable = errors.New("budgets: canal nao disponivel para notificacao")

type NotifyThresholdAlertInput struct {
	UserID               uuid.UUID
	BudgetID             uuid.UUID
	Kind                 services.ThresholdAlertKind
	RootSlug             string
	PercentUsedBps       int32
	AmountRemainingCents int64
	RefDay               time.Time
}

type NotifyThresholdAlertResult struct {
	Outcome string
	Channel string
}

type NotifyThresholdAlert struct {
	sentRepo        appinterfaces.ThresholdAlertSentRepository
	channelResolver appinterfaces.UserChannelResolver
	channelGateway  notification.ChannelGateway
	o11y            observability.Observability
	delivered       observability.Counter
}

func NewNotifyThresholdAlert(
	sentRepo appinterfaces.ThresholdAlertSentRepository,
	channelResolver appinterfaces.UserChannelResolver,
	channelGateway notification.ChannelGateway,
	o11y observability.Observability,
) *NotifyThresholdAlert {
	delivered := o11y.Metrics().Counter(
		"budgets_threshold_alert_delivered_total",
		"Total de notificacoes de threshold alert despachadas por kind, canal e outcome",
		"1",
	)
	return &NotifyThresholdAlert{
		sentRepo:        sentRepo,
		channelResolver: channelResolver,
		channelGateway:  channelGateway,
		o11y:            o11y,
		delivered:       delivered,
	}
}

func (uc *NotifyThresholdAlert) Execute(ctx context.Context, in NotifyThresholdAlertInput) (NotifyThresholdAlertResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.notify_threshold_alert")
	defer span.End()

	kindLabel := in.Kind.String()

	notified, err := uc.sentRepo.IsNotified(ctx, in.UserID, in.BudgetID, in.Kind, in.RefDay)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrAlertRecordMissing) {
			uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeRecordMissing)
			return NotifyThresholdAlertResult{Outcome: NotifyOutcomeRecordMissing}, nil
		}
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeRecordMissing)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeRecordMissing}, fmt.Errorf("budgets: notify threshold alert: check sent: %w", err)
	}
	if notified {
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeAlreadySent)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeAlreadySent}, nil
	}

	pref, ok, err := uc.channelResolver.ResolvePreferred(ctx, in.UserID)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeResolverError)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeResolverError}, fmt.Errorf("budgets: notify threshold alert: resolve channel: %w", err)
	}
	if !ok {
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeNoChannel)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeNoChannel}, nil
	}

	text, err := renderThresholdAlertText(in)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeChannelFailed)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("budgets: notify threshold alert: render: %w", err)
	}

	updated, err := uc.sentRepo.MarkNotified(ctx, in.UserID, in.BudgetID, in.Kind, in.RefDay, pref.Channel, time.Now().UTC())
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeChannelFailed)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("budgets: notify threshold alert: mark notified: %w", err)
	}
	if !updated {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeAlreadySent)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeAlreadySent, Channel: pref.Channel}, nil
	}

	if err := uc.channelGateway.SendText(ctx, pref.Channel, pref.ExternalID, text); err != nil {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeChannelFailed)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("budgets: notify threshold alert: send: %w: %w", ErrNotifyChannelUnavailable, err)
	}

	uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeSent)
	return NotifyThresholdAlertResult{Outcome: NotifyOutcomeSent, Channel: pref.Channel}, nil
}

func (uc *NotifyThresholdAlert) recordOutcome(ctx context.Context, kind, channel, outcome string) {
	uc.delivered.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
}

func renderThresholdAlertText(in NotifyThresholdAlertInput) (string, error) {
	percent := float64(in.PercentUsedBps) / 100.0
	remaining := formatBRL(in.AmountRemainingCents)
	switch in.Kind {
	case services.ThresholdAlertCategory:
		category := in.RootSlug
		if category == "" {
			category = "sua categoria"
		}
		return fmt.Sprintf("Atencao: voce ja utilizou %.1f%% de %s. Ainda restam %s.", percent, category, remaining), nil
	case services.ThresholdAlertGoal:
		return fmt.Sprintf("Boa! Voce ja acumulou %.1f%% da meta. Continue assim!", percent), nil
	case services.ThresholdAlertCardLimit:
		return fmt.Sprintf("Atencao: voce ja utilizou %.1f%% do limite do cartao. Restam apenas %s.", percent, remaining), nil
	default:
		return "", fmt.Errorf("budgets: kind desconhecido: %d", in.Kind)
	}
}

func formatBRL(cents int64) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}
	reais := cents / 100
	subunit := cents % 100
	prefix := "R$"
	if negative {
		prefix = "-R$"
	}
	return fmt.Sprintf("%s%d,%02d", prefix, reais, subunit)
}
