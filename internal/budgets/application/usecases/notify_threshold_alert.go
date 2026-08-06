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
	NotifyOutcomeSuppressed    = "suppressed"
)

var ErrNotifyChannelUnavailable = errors.New("budgets: canal nao disponivel para notificacao")

type NotifyThresholdAlertInput struct {
	UserID               uuid.UUID
	BudgetID             uuid.UUID
	Kind                 services.ThresholdAlertKind
	RootSlug             string
	PercentUsedBps       int32
	AmountRemainingCents int64
	PlannedCents         int64
	SpentCents           int64
	RefDay               time.Time
}

type NotifyThresholdAlertResult struct {
	Outcome string
	Channel string
	Reason  string
}

type NotifyThresholdAlert struct {
	sentRepo        appinterfaces.ThresholdAlertSentRepository
	channelResolver appinterfaces.UserChannelResolver
	channelGateway  notification.ChannelGateway
	templates       appinterfaces.AlertTemplateCatalog
	timezones       appinterfaces.UserTimezoneResolver
	consents        appinterfaces.MarketingConsentReader
	alertContext    appinterfaces.AlertContextRecorder
	quietStart      time.Duration
	quietEnd        time.Duration
	o11y            observability.Observability
	delivered       observability.Counter
	notified        observability.Counter
	suppressed      observability.Counter
	templateStatus  observability.Counter
}

func NewNotifyThresholdAlert(
	sentRepo appinterfaces.ThresholdAlertSentRepository,
	channelResolver appinterfaces.UserChannelResolver,
	channelGateway notification.ChannelGateway,
	templates appinterfaces.AlertTemplateCatalog,
	timezones appinterfaces.UserTimezoneResolver,
	consents appinterfaces.MarketingConsentReader,
	alertContext appinterfaces.AlertContextRecorder,
	quietStart time.Duration,
	quietEnd time.Duration,
	o11y observability.Observability,
) *NotifyThresholdAlert {
	return &NotifyThresholdAlert{
		sentRepo:        sentRepo,
		channelResolver: channelResolver,
		channelGateway:  channelGateway,
		templates:       templates,
		timezones:       timezones,
		consents:        consents,
		alertContext:    alertContext,
		quietStart:      quietStart,
		quietEnd:        quietEnd,
		o11y:            o11y,
		delivered: o11y.Metrics().Counter(
			"budgets_threshold_alert_delivered_total",
			"Total de notificacoes de threshold alert despachadas por kind, canal e outcome",
			"1",
		),
		notified: o11y.Metrics().Counter(
			"proactive_alerts_notified_total",
			"Total de alertas proativos notificados por kind, canal e outcome",
			"1",
		),
		suppressed: o11y.Metrics().Counter(
			"proactive_alerts_suppressed_total",
			"Total de alertas proativos suprimidos por kind e motivo",
			"1",
		),
		templateStatus: o11y.Metrics().Counter(
			"proactive_alerts_template_status",
			"Status de template Meta observado por kind no momento do envio",
			"1",
		),
	}
}

func (uc *NotifyThresholdAlert) Execute(ctx context.Context, in NotifyThresholdAlertInput) (NotifyThresholdAlertResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.notify_threshold_alert")
	defer span.End()

	kindLabel := in.Kind.String()

	if result, done, err := uc.validateNotificationEligibility(ctx, in, kindLabel); done || err != nil {
		return result, err
	}

	pref, result, done, err := uc.resolveChannel(ctx, in.UserID, kindLabel)
	if done || err != nil {
		return result, err
	}

	template, result, done := uc.resolveTemplate(ctx, in.Kind, kindLabel, pref.Channel)
	if done {
		return result, nil
	}

	if result, done, err := uc.validateConsent(ctx, in.UserID, template, kindLabel, pref.Channel); done || err != nil {
		return result, err
	}

	if result, done, err := uc.validateTimezone(ctx, in.UserID, kindLabel, pref.Channel); done || err != nil {
		return result, err
	}

	return uc.deliverNotification(ctx, in, kindLabel, pref, template)
}

func (uc *NotifyThresholdAlert) validateNotificationEligibility(ctx context.Context, in NotifyThresholdAlertInput, kindLabel string) (NotifyThresholdAlertResult, bool, error) {
	if !services.IsReleaseOneEmittable(in.Kind) {
		return uc.suppress(ctx, kindLabel, "unknown", services.SuppressedKindBlocked), true, nil
	}

	notified, err := uc.sentRepo.IsNotified(ctx, in.UserID, in.BudgetID, in.Kind, in.RefDay)
	if err != nil {
		if errors.Is(err, appinterfaces.ErrAlertRecordMissing) {
			uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeRecordMissing)
			return NotifyThresholdAlertResult{Outcome: NotifyOutcomeRecordMissing}, true, nil
		}
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeRecordMissing)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeRecordMissing}, true, fmt.Errorf("budgets: notify threshold alert: check sent: %w", err)
	}
	if notified {
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeAlreadySent)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeAlreadySent}, true, nil
	}

	return NotifyThresholdAlertResult{}, false, nil
}

func (uc *NotifyThresholdAlert) resolveChannel(ctx context.Context, userID uuid.UUID, kindLabel string) (appinterfaces.UserChannelPreference, NotifyThresholdAlertResult, bool, error) {
	pref, ok, err := uc.channelResolver.ResolvePreferred(ctx, userID)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, "unknown", NotifyOutcomeResolverError)
		return appinterfaces.UserChannelPreference{}, NotifyThresholdAlertResult{Outcome: NotifyOutcomeResolverError}, true, fmt.Errorf("budgets: notify threshold alert: resolve channel: %w", err)
	}
	if !ok {
		return appinterfaces.UserChannelPreference{}, uc.suppress(ctx, kindLabel, "unknown", services.SuppressedNoChannel), true, nil
	}

	return pref, NotifyThresholdAlertResult{}, false, nil
}

func (uc *NotifyThresholdAlert) resolveTemplate(ctx context.Context, kind services.ThresholdAlertKind, kindLabel string, channel string) (appinterfaces.AlertTemplate, NotifyThresholdAlertResult, bool) {
	template, found := uc.templates.Lookup(kind)
	if !found {
		uc.templateStatus.Add(ctx, 1,
			observability.String("kind", kindLabel),
			observability.String("status", services.TemplateStatusUnknown.String()),
		)
		return appinterfaces.AlertTemplate{}, uc.suppress(ctx, kindLabel, channel, services.SuppressedTemplateUnapproved), true
	}

	uc.templateStatus.Add(ctx, 1,
		observability.String("kind", kindLabel),
		observability.String("status", template.Status.String()),
	)
	if !services.AllowsRealSend(template.Status) {
		return appinterfaces.AlertTemplate{}, uc.suppress(ctx, kindLabel, channel, services.SuppressedTemplateUnapproved), true
	}

	return template, NotifyThresholdAlertResult{}, false
}

func (uc *NotifyThresholdAlert) validateConsent(ctx context.Context, userID uuid.UUID, template appinterfaces.AlertTemplate, kindLabel string, channel string) (NotifyThresholdAlertResult, bool, error) {
	if !services.RequiresExplicitOptIn(template.Category) {
		return NotifyThresholdAlertResult{}, false, nil
	}

	consented, err := uc.consents.HasConsent(ctx, userID)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, channel, NotifyOutcomeResolverError)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeResolverError, Channel: channel}, true, fmt.Errorf("budgets: notify threshold alert: consentimento: %w", err)
	}
	if !consented {
		return uc.suppress(ctx, kindLabel, channel, services.SuppressedOptInMissing), true, nil
	}

	return NotifyThresholdAlertResult{}, false, nil
}

func (uc *NotifyThresholdAlert) validateTimezone(ctx context.Context, userID uuid.UUID, kindLabel string, channel string) (NotifyThresholdAlertResult, bool, error) {
	location, err := uc.timezones.Resolve(ctx, userID)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, channel, NotifyOutcomeResolverError)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeResolverError, Channel: channel}, true, fmt.Errorf("budgets: notify threshold alert: timezone: %w", err)
	}
	if uc.isQuietHours(time.Now().UTC(), location) {
		return uc.suppress(ctx, kindLabel, channel, services.SuppressedQuietHours), true, nil
	}

	return NotifyThresholdAlertResult{}, false, nil
}

func (uc *NotifyThresholdAlert) deliverNotification(ctx context.Context, in NotifyThresholdAlertInput, kindLabel string, pref appinterfaces.UserChannelPreference, template appinterfaces.AlertTemplate) (NotifyThresholdAlertResult, error) {
	message, err := buildTemplateMessage(pref.Channel, pref.ExternalID, template, in)
	if err != nil {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeChannelFailed)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("budgets: notify threshold alert: montar template: %w", err)
	}

	if _, err := uc.channelGateway.SendTemplate(ctx, message); err != nil {
		uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeChannelFailed)
		return NotifyThresholdAlertResult{Outcome: NotifyOutcomeChannelFailed, Channel: pref.Channel}, fmt.Errorf("budgets: notify threshold alert: send: %w: %w", ErrNotifyChannelUnavailable, err)
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

	uc.recordAlertContext(ctx, in, pref.ExternalID)
	uc.recordOutcome(ctx, kindLabel, pref.Channel, NotifyOutcomeSent)

	return NotifyThresholdAlertResult{Outcome: NotifyOutcomeSent, Channel: pref.Channel}, nil
}

func (uc *NotifyThresholdAlert) recordAlertContext(ctx context.Context, in NotifyThresholdAlertInput, externalID string) {
	if uc.alertContext == nil {
		return
	}
	err := uc.alertContext.Record(
		ctx,
		in.UserID.String(),
		externalID,
		in.Kind.String(),
		services.AlertFollowUpTopic(in.Kind),
		time.Now().UTC(),
	)
	if err != nil {
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.notify_threshold_alert.alert_context_failed",
			observability.String("kind", in.Kind.String()),
			observability.Error(err),
		)
	}
}

func (uc *NotifyThresholdAlert) isQuietHours(now time.Time, loc *time.Location) bool {
	if loc == nil {
		loc = time.UTC
	}
	if uc.quietStart == uc.quietEnd {
		return false
	}
	local := now.In(loc)
	current := time.Duration(local.Hour())*time.Hour + time.Duration(local.Minute())*time.Minute
	if uc.quietStart < uc.quietEnd {
		return current >= uc.quietStart && current < uc.quietEnd
	}
	return current >= uc.quietStart || current < uc.quietEnd
}

func (uc *NotifyThresholdAlert) suppress(ctx context.Context, kind, channel string, reason services.SuppressionReason) NotifyThresholdAlertResult {
	uc.suppressed.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("reason", reason.String()),
	)
	uc.recordOutcome(ctx, kind, channel, NotifyOutcomeSuppressed)
	uc.o11y.Logger().Info(ctx, "budgets.usecase.notify_threshold_alert.suppressed",
		observability.String("kind", kind),
		observability.String("reason", reason.String()),
	)
	return NotifyThresholdAlertResult{Outcome: NotifyOutcomeSuppressed, Channel: channel, Reason: reason.String()}
}

func (uc *NotifyThresholdAlert) recordOutcome(ctx context.Context, kind, channel, outcome string) {
	uc.delivered.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
	uc.notified.Add(ctx, 1,
		observability.String("kind", kind),
		observability.String("channel", channel),
		observability.String("outcome", outcome),
	)
}

func buildTemplateMessage(channel, externalID string, template appinterfaces.AlertTemplate, in NotifyThresholdAlertInput) (notification.TemplateMessage, error) {
	message := notification.TemplateMessage{
		Channel:      channel,
		ExternalID:   externalID,
		TemplateName: template.Name,
		LanguageCode: template.LanguageCode,
	}

	parameters, err := templateParameters(in)
	if err != nil {
		return notification.TemplateMessage{}, err
	}
	if len(parameters) > 0 {
		message.Components = []notification.TemplateComponent{
			{Type: notification.TemplateComponentBody, Parameters: parameters},
		}
	}

	if err := message.Validate(); err != nil {
		return notification.TemplateMessage{}, err
	}
	return message, nil
}

func templateParameters(in NotifyThresholdAlertInput) ([]notification.TemplateParameter, error) {
	switch in.Kind {
	case services.ThresholdAlertCategory80, services.ThresholdAlertCategory100:
		category := in.RootSlug
		if category == "" {
			category = "sua categoria"
		}
		return []notification.TemplateParameter{
			{Type: notification.TemplateParameterText, Text: category},
			{Type: notification.TemplateParameterText, Text: formatBRL(in.PlannedCents)},
			{Type: notification.TemplateParameterText, Text: formatBRL(in.SpentCents)},
			{Type: notification.TemplateParameterText, Text: formatBRL(absCents(in.AmountRemainingCents))},
		}, nil
	case services.ThresholdAlertBudgetMissingMonthStart, services.ThresholdAlertBudgetNotReviewedDay3:
		return nil, nil
	default:
		return nil, fmt.Errorf("budgets: kind sem contrato de template: %q", in.Kind.String())
	}
}

func absCents(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
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
