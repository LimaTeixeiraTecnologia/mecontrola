package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/observability"
)

// AnonymizeWebhookEventsUseCase executa o job diário de anonimização de payloads PII (RF-49, ADR-013).
// Para cada row elegível: aplica o redactor → persiste payload redacted + anonymized_at.
type AnonymizeWebhookEventsUseCase struct {
	webhookRepo interfaces.WebhookEventRepository
	redactor    interfaces.PIIRedactor
	o11y        observability.Observability
	metrics     *observability.UsecaseMetrics
}

func NewAnonymizeWebhookEventsUseCase(
	webhookRepo interfaces.WebhookEventRepository,
	redactor interfaces.PIIRedactor,
	o11y observability.Observability,
	metrics *observability.UsecaseMetrics,
) *AnonymizeWebhookEventsUseCase {
	return &AnonymizeWebhookEventsUseCase{
		webhookRepo: webhookRepo,
		redactor:    redactor,
		o11y:        o11y,
		metrics:     metrics,
	}
}

func (u *AnonymizeWebhookEventsUseCase) Execute(ctx context.Context, in input.AnonymizeInput) (output.AnonymizationReport, error) {
	return observability.Observe(ctx, u.o11y, u.metrics, "billing", "anonymize_webhook_events", func(ctx context.Context) (output.AnonymizationReport, error) {
		return u.execute(ctx, in)
	})
}

func (u *AnonymizeWebhookEventsUseCase) execute(ctx context.Context, in input.AnonymizeInput) (output.AnonymizationReport, error) {
	rows, err := u.webhookRepo.ListPendingAnonymization(ctx, in.OlderThan, in.BatchSize)
	if err != nil {
		return output.AnonymizationReport{}, fmt.Errorf("anonimizar webhooks: listar pendentes: %w", err)
	}

	var report output.AnonymizationReport
	now := time.Now().UTC()

	for _, row := range rows {
		redacted, err := u.redactor.Strip(row.Payload())
		if err != nil {
			report.Errors++
			continue
		}
		if err := u.webhookRepo.Anonymize(ctx, row.ID(), redacted, now); err != nil {
			report.Errors++
			continue
		}
		report.Processed++
	}
	return report, nil
}
