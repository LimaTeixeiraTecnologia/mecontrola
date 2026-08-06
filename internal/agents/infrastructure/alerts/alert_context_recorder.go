package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/memory"
)

const (
	MetadataKeyLastProactiveAlert = "last_proactive_alert"
	metadataFieldKind             = "kind"
	metadataFieldSentAt           = "sent_at"
	metadataFieldTopic            = "follow_up_topic"
)

type AlertContextRecorder struct {
	threads    memory.ThreadGateway
	messages   memory.MessageStore
	workingMem memory.WorkingMemory
	ttl        time.Duration
	o11y       observability.Observability
}

func NewAlertContextRecorder(
	threads memory.ThreadGateway,
	messages memory.MessageStore,
	workingMem memory.WorkingMemory,
	ttl time.Duration,
	o11y observability.Observability,
) *AlertContextRecorder {
	return &AlertContextRecorder{
		threads:    threads,
		messages:   messages,
		workingMem: workingMem,
		ttl:        ttl,
		o11y:       o11y,
	}
}

func (r *AlertContextRecorder) PurgeExpired(ctx context.Context, resourceID string, now time.Time) error {
	reader, ok := r.workingMem.(memory.WorkingMemoryMetadataReader)
	if !ok || resourceID == "" {
		return nil
	}

	metadata, err := reader.GetMetadata(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("agents.alerts.alert_context_recorder: ler contexto: %w", err)
	}

	sentAt, ok := parseAlertContextSentAt(metadata)
	if !ok {
		return nil
	}
	if IsAlertContextValid(sentAt, now, r.ttl) {
		return nil
	}

	if err := r.workingMem.UpsertMetadata(ctx, resourceID, map[string]any{MetadataKeyLastProactiveAlert: nil}); err != nil {
		return fmt.Errorf("agents.alerts.alert_context_recorder: expirar contexto: %w", err)
	}
	return nil
}

func IsAlertContextValid(sentAt, now time.Time, ttl time.Duration) bool {
	if sentAt.IsZero() || ttl <= 0 {
		return false
	}
	if now.Before(sentAt) {
		return false
	}
	return now.Sub(sentAt) <= ttl
}

func parseAlertContextSentAt(metadata map[string]any) (time.Time, bool) {
	raw, ok := metadata[MetadataKeyLastProactiveAlert]
	if !ok || raw == nil {
		return time.Time{}, false
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return time.Time{}, false
	}
	value, ok := entry[metadataFieldSentAt].(string)
	if !ok || value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func (r *AlertContextRecorder) Record(ctx context.Context, resourceID, threadID, kind, followUpTopic string, sentAt time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "agents.alerts.alert_context_recorder.record")
	defer span.End()

	if resourceID == "" || threadID == "" || kind == "" {
		return fmt.Errorf("agents.alerts.alert_context_recorder: identificadores obrigatorios ausentes")
	}

	thread, err := r.threads.GetOrCreate(ctx, resourceID, threadID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.alerts.alert_context_recorder: resolver thread: %w", err)
	}

	message := memory.Message{
		ID:               uuid.New(),
		PlatformThreadID: thread.ID,
		ResourceID:       resourceID,
		Role:             memory.RoleAssistant,
		Content:          renderAlertContextMessage(kind, followUpTopic),
		CreatedAt:        sentAt,
	}
	if err := r.messages.Append(ctx, thread.ID, message); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.alerts.alert_context_recorder: registrar mensagem: %w", err)
	}

	metadata := map[string]any{
		MetadataKeyLastProactiveAlert: map[string]any{
			metadataFieldKind:   kind,
			metadataFieldSentAt: sentAt.UTC().Format(time.RFC3339),
			metadataFieldTopic:  followUpTopic,
		},
	}
	if err := r.workingMem.UpsertMetadata(ctx, resourceID, metadata); err != nil {
		span.RecordError(err)
		return fmt.Errorf("agents.alerts.alert_context_recorder: registrar contexto: %w", err)
	}

	return nil
}

func renderAlertContextMessage(kind, followUpTopic string) string {
	if followUpTopic == "" {
		return fmt.Sprintf("Enviei um alerta proativo (%s) para voce.", kind)
	}
	return fmt.Sprintf("Enviei um alerta proativo (%s) para voce. Se responder que sim, o proximo passo e %s.", kind, followUpTopic)
}
