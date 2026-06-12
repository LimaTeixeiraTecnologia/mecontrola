package consumers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	coalescerInternal "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/consumers/internal"
)

const shutdownTimeout = 10 * time.Second

type recomputeUseCase interface {
	Execute(ctx context.Context, in usecases.RecomputeMonthlySummaryInput) error
}

type refMonthsPayload struct {
	UserID            string   `json:"user_id"`
	RefMonth          string   `json:"ref_month"`
	RefMonthsAffected []string `json:"ref_months_affected"`
}

type MonthlySummaryRecomputeConsumer struct {
	recompute recomputeUseCase
	coalescer *coalescerInternal.Coalescer
	o11y      observability.Observability
}

func NewMonthlySummaryRecomputeConsumer(
	recompute recomputeUseCase,
	debounceWindow time.Duration,
	o11y observability.Observability,
) *MonthlySummaryRecomputeConsumer {
	_ = o11y.Metrics().HistogramWithBuckets(
		"transactions_monthly_summary_coalesce_factor",
		"Eventos colapsados por recompute de resumo mensal",
		"1",
		[]float64{1, 2, 5, 10, 20, 50},
	)
	return &MonthlySummaryRecomputeConsumer{
		recompute: recompute,
		coalescer: coalescerInternal.NewCoalescer(debounceWindow, shutdownTimeout),
		o11y:      o11y,
	}
}

func (c *MonthlySummaryRecomputeConsumer) Handle(ctx context.Context, event events.Event) error {
	_, span := c.o11y.Tracer().Start(ctx, "transactions.consumer.monthly_summary_recompute.handle")
	defer span.End()

	payload := event.GetPayload()
	env, ok := payload.(outbox.Envelope)
	if !ok {
		return fmt.Errorf("transactions.consumer.monthly_summary_recompute: unexpected payload type %T", payload)
	}

	var p refMonthsPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("transactions.consumer.monthly_summary_recompute: deserializar payload: %w", err)
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return fmt.Errorf("transactions.consumer.monthly_summary_recompute: user_id inválido: %w", err)
	}

	refMonths := p.RefMonthsAffected
	if len(refMonths) == 0 && p.RefMonth != "" {
		refMonths = []string{p.RefMonth}
	}

	for _, rm := range refMonths {
		refMonth, rmErr := valueobjects.NewRefMonth(rm)
		if rmErr != nil {
			continue
		}
		key := userID.String() + ":" + rm
		captured := refMonth
		capturedUserID := userID
		c.coalescer.Schedule(key, func(ctx context.Context) {
			if err := c.recompute.Execute(ctx, usecases.RecomputeMonthlySummaryInput{
				UserID:   capturedUserID,
				RefMonth: captured,
			}); err != nil {
				c.o11y.Logger().Error(ctx, "transactions.consumer.recompute_monthly_summary.failed",
					observability.String("user_id", capturedUserID.String()),
					observability.String("ref_month", captured.String()),
					observability.Error(err),
				)
			}
		})
	}

	return nil
}

func (c *MonthlySummaryRecomputeConsumer) Stop(ctx context.Context) {
	c.coalescer.Stop(ctx)
}
