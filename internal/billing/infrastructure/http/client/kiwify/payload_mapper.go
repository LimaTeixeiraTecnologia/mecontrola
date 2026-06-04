package kiwify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

var (
	// ErrPayloadDecode é retornado quando o JSON do payload Kiwify é inválido.
	ErrPayloadDecode = errors.New("kiwify payload: json inválido")
	// ErrUnknownKiwifyEventType é retornado para tipos de evento não mapeados.
	ErrUnknownKiwifyEventType = errors.New("kiwify payload: event_type desconhecido")
)

const periodDivergenceTolerance = 14 * 24 * time.Hour

var tracer = otel.Tracer("billing/kiwify")

// periodDivergenceCounter é uma função injetável para emissão de métrica de divergência.
// Padrão via OTel meter — substituível em testes por contador simples.
type periodDivergenceCounter interface {
	Add(planCode string, sign string)
}

// noopDivergenceCounter ignora divergências (padrão quando nenhum counter é fornecido).
type noopDivergenceCounter struct{}

func (noopDivergenceCounter) Add(_, _ string) {}

// PayloadMapper mapeia payloads Kiwify para CanonicalEvent (RF-29, RF-30, ADR-011).
type PayloadMapper struct {
	registry          *BillingPlansRegistry
	divergenceCounter periodDivergenceCounter
}

// NewPayloadMapper cria um PayloadMapper com registry de planos e counter de divergência OTel.
func NewPayloadMapper(registry *BillingPlansRegistry, counter periodDivergenceCounter) *PayloadMapper {
	if counter == nil {
		counter = noopDivergenceCounter{}
	}
	return &PayloadMapper{
		registry:          registry,
		divergenceCounter: counter,
	}
}

// Parse mapeia o payload bruto Kiwify para CanonicalEvent.
// Aplica cascata de tracking (RF-30) e sanity check de period_end com tolerância ±14 dias (ADR-011).
// Nunca entra em pânico — FuzzPayloadMapperParse valida isso.
func (m *PayloadMapper) Parse(raw []byte) (services.CanonicalEvent, error) {
	return m.ParseWithContext(context.Background(), raw)
}

// ParseWithContext mapeia o payload bruto Kiwify para CanonicalEvent propagando contexto OTel.
func (m *PayloadMapper) ParseWithContext(ctx context.Context, raw []byte) (services.CanonicalEvent, error) {
	_, span := tracer.Start(ctx, "kiwify.payload.parse")
	defer span.End()

	var payload kiwifyPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return services.CanonicalEvent{}, ErrPayloadDecode
	}
	eventType, err := mapEventType(payload.WebhookEventType)
	if err != nil {
		return services.CanonicalEvent{}, err
	}
	whatsapp, err := identityvo.NewWhatsAppNumber(payload.Customer.Mobile)
	if err != nil {
		return services.CanonicalEvent{}, fmt.Errorf("kiwify payload: normalizar mobile: %w", err)
	}
	plan, err := m.registry.ParsePlanCodeFromKiwifyProductID(payload.Product.ID)
	if err != nil {
		return services.CanonicalEvent{}, fmt.Errorf("kiwify payload: resolver plan code: %w", err)
	}
	m.checkPeriodDivergence(ctx, span, plan, payload)
	return services.CanonicalEvent{
		Type:                   eventType,
		ExternalEventID:        payload.ID,
		ExternalSubscriptionID: payload.Subscription.ID,
		PlanCode:               plan,
		OccurredAt:             payload.UpdatedAt,
		PeriodStart:            payload.Subscription.CurrentPeriodStart,
		PeriodEnd:              payload.Subscription.CurrentPeriodEnd,
		SignupToken:            extractSignupTokenCascade(payload.Tracking),
		Customer: services.CanonicalCustomer{
			WhatsApp: whatsapp,
			Email:    payload.Customer.Email,
		},
		RefundAmountCents: payload.Refund.AmountCents,
	}, nil
}

// checkPeriodDivergence valida se period_end diverge do esperado local com tolerância ±14 dias.
// Divergência emite métrica billing_period_divergence_total mas NÃO bloqueia o processamento (ADR-011).
func (m *PayloadMapper) checkPeriodDivergence(ctx context.Context, span interface{ SetAttributes(...attribute.KeyValue) }, plan valueobjects.PlanCode, payload kiwifyPayload) {
	period, err := valueobjects.NewBillingPeriodFor(plan)
	if err != nil {
		return
	}
	expectedEnd := payload.Subscription.CurrentPeriodStart.Add(period.Length())
	actual := payload.Subscription.CurrentPeriodEnd
	if actual.IsZero() || payload.Subscription.CurrentPeriodStart.IsZero() {
		return
	}
	diff := actual.Sub(expectedEnd)
	if diff < 0 {
		diff = -diff
	}
	if diff > periodDivergenceTolerance {
		sign := "ahead"
		if actual.Before(expectedEnd) {
			sign = "behind"
		}
		span.SetAttributes(
			attribute.String("billing.period_divergence.plan", plan.String()),
			attribute.String("billing.period_divergence.sign", sign),
		)
		m.divergenceCounter.Add(plan.String(), sign)
		_ = ctx
	}
}

func extractSignupTokenCascade(tracking kiwifyTracking) string {
	candidates := []string{tracking.Src, tracking.UTMContent, tracking.S1, tracking.S2, tracking.S3}
	for _, candidate := range candidates {
		if v := strings.TrimSpace(candidate); v != "" {
			return v
		}
	}
	return ""
}

func mapEventType(s string) (valueobjects.CanonicalEventType, error) {
	switch s {
	case "compra_aprovada":
		return valueobjects.CanonicalEventPurchaseApproved, nil
	case "subscription_renewed":
		return valueobjects.CanonicalEventRenewed, nil
	case "subscription_late":
		return valueobjects.CanonicalEventLate, nil
	case "subscription_canceled":
		return valueobjects.CanonicalEventCanceled, nil
	case "compra_reembolsada":
		return valueobjects.CanonicalEventRefunded, nil
	case "chargeback":
		return valueobjects.CanonicalEventChargeback, nil
	default:
		return valueobjects.CanonicalEventUnknown, fmt.Errorf("%w: %w: %q", interfaces.ErrUnknownProviderEventType, ErrUnknownKiwifyEventType, s)
	}
}

type kiwifyPayload struct {
	ID               string         `json:"id"`
	WebhookEventType string         `json:"webhook_event_type"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Customer         kiwifyCustomer `json:"customer"`
	Product          kiwifyProduct  `json:"product"`
	Subscription     kiwifySub      `json:"subscription"`
	Refund           kiwifyRefund   `json:"refund"`
	Tracking         kiwifyTracking `json:"tracking"`
}

type kiwifyCustomer struct {
	Mobile string `json:"mobile"`
	Email  string `json:"email"`
}

type kiwifyProduct struct {
	ID string `json:"id"`
}

type kiwifySub struct {
	ID                 string    `json:"id"`
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
}

type kiwifyRefund struct {
	AmountCents int64 `json:"amount_cents"`
}

type kiwifyTracking struct {
	Src        string `json:"src"`
	UTMContent string `json:"utm_content"`
	S1         string `json:"s1"`
	S2         string `json:"s2"`
	S3         string `json:"s3"`
}
