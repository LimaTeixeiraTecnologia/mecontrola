package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

// receivedEventPayload é o pointer mínimo publicado no outbox (ADR-001).
// Contém apenas referência ao webhook_event_id para que o processor busque o payload bruto.
type receivedEventPayload struct {
	WebhookEventID string `json:"webhook_event_id"`
	Provider       string `json:"provider"`
}

// IngestKiwifyWebhookUseCase orquestra o recebimento de um webhook Kiwify:
// verifica assinatura → dedup via InsertIfNew → publica no outbox (ADR-001).
type IngestKiwifyWebhookUseCase struct {
	provider    interfaces.BillingProvider
	webhookRepo interfaces.WebhookEventRepository
	publisher   outbox.Publisher
	txRunner    interfaces.TxRunner[output.IngestWebhookResult]
	idGenerator interfaces.IDGenerator
	o11y        observability.Observability
	metrics     *observability.UsecaseMetrics
}

func NewIngestKiwifyWebhookUseCase(
	provider interfaces.BillingProvider,
	webhookRepo interfaces.WebhookEventRepository,
	publisher outbox.Publisher,
	txRunner interfaces.TxRunner[output.IngestWebhookResult],
	idGenerator interfaces.IDGenerator,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *IngestKiwifyWebhookUseCase {
	return &IngestKiwifyWebhookUseCase{
		provider:    provider,
		webhookRepo: webhookRepo,
		publisher:   publisher,
		txRunner:    txRunner,
		idGenerator: idGenerator,
		o11y:        o11y,
		metrics:     metrics,
	}
}

func (u *IngestKiwifyWebhookUseCase) Execute(ctx context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "billing", "ingest_kiwify_webhook", func(ctx context.Context) (output.IngestWebhookResult, error) {
		return u.execute(ctx, in)
	})
}

func (u *IngestKiwifyWebhookUseCase) execute(ctx context.Context, in input.IngestWebhookInput) (output.IngestWebhookResult, error) {
	if err := u.provider.VerifySignature(in.RawBody, in.Headers); err != nil {
		return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: %w", err)
	}
	externalID, err := valueobjects.NewExternalEventIDCascade(in.RawBody)
	if err != nil {
		return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: %w", err)
	}
	return u.txRunner.Do(ctx, func(txCtx context.Context, tx database.DBTX) (output.IngestWebhookResult, error) {
		now := time.Now().UTC()
		webhookID, err := valueobjects.NewWebhookEventID(u.idGenerator.NewID())
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: gerar webhook event id: %w", err)
		}
		webhookEvent, err := entities.NewWebhookEvent(entities.NewWebhookEventParams{
			ID:              webhookID,
			Provider:        "kiwify",
			ExternalEventID: externalID,
			EventType:       extractEventType(in.RawBody),
			Signature:       lookupHeader(in.Headers, in.SignatureHeaderName),
			HeadersJSON:     in.HeadersJSON(),
			Payload:         in.RawBody,
			ReceivedAt:      now,
		})
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: criar webhook event: %w", err)
		}
		inserted, err := u.webhookRepo.InsertIfNew(txCtx, webhookEvent)
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: inserir webhook_event: %w", err)
		}
		if !inserted {
			return output.IngestWebhookResult{Duplicate: true}, nil
		}
		payload, err := encodeReceivedPayload(webhookEvent.ID().String(), webhookEvent.Provider())
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: codificar payload outbox: %w", err)
		}
		eventID, err := events.NewEventID(u.idGenerator.NewID())
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: gerar event id: %w", err)
		}
		eventName, err := events.NewEventName("billing.kiwify.received")
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: criar event name: %w", err)
		}
		evt, err := outbox.NewEvent(outbox.NewEventParams{
			ID:            eventID,
			EventType:     eventName,
			AggregateType: "webhook_event",
			AggregateID:   webhookEvent.ID().String(),
			Payload:       payload,
			OccurredAt:    now,
		})
		if err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: criar outbox event: %w", err)
		}
		if err := u.publisher.Publish(txCtx, tx, evt); err != nil {
			return output.IngestWebhookResult{}, fmt.Errorf("ingest kiwify: publicar outbox: %w", err)
		}
		return output.IngestWebhookResult{Duplicate: false, WebhookEventID: webhookEvent.ID()}, nil
	})
}

func encodeReceivedPayload(webhookEventID, provider string) (json.RawMessage, error) {
	return json.Marshal(receivedEventPayload{
		WebhookEventID: webhookEventID,
		Provider:       provider,
	})
}

func decodeReceivedPayload(raw json.RawMessage) (receivedEventPayload, error) {
	var p receivedEventPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return receivedEventPayload{}, fmt.Errorf("decodificar payload outbox: %w", err)
	}
	if p.WebhookEventID == "" {
		return receivedEventPayload{}, fmt.Errorf("payload outbox: webhook_event_id obrigatório")
	}
	return p, nil
}

func extractEventType(raw []byte) string {
	var probe struct {
		EventType string `json:"webhook_event_type"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.EventType
}

func lookupHeader(headers map[string]string, name string) string {
	if strings.TrimSpace(name) == "" {
		name = "X-Kiwify-Webhook-Token"
	}
	for k, v := range headers {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}
