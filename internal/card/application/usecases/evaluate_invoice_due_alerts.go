package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/services"
)

type EvaluateInvoiceDueAlerts struct {
	factory    interfaces.RepositoryFactory
	publisher  interfaces.InvoiceDuePublisher
	uow        uow.UnitOfWork[struct{}]
	decider    services.InvoiceDueAlertsDecider
	location   *time.Location
	windowDays int
	scanLimit  int
	o11y       observability.Observability
	dispatched observability.Counter
}

func NewEvaluateInvoiceDueAlerts(
	factory interfaces.RepositoryFactory,
	publisher interfaces.InvoiceDuePublisher,
	u uow.UnitOfWork[struct{}],
	location *time.Location,
	windowDays int,
	scanLimit int,
	o11y observability.Observability,
) *EvaluateInvoiceDueAlerts {
	dispatched := o11y.Metrics().Counter(
		"card_invoice_due_alerts_dispatched_total",
		"Total de alertas proativos de vencimento de fatura disparados",
		"1",
	)
	if windowDays <= 0 {
		windowDays = 3
	}
	if scanLimit <= 0 {
		scanLimit = 500
	}
	return &EvaluateInvoiceDueAlerts{
		factory:    factory,
		publisher:  publisher,
		uow:        u,
		decider:    services.NewInvoiceDueAlertsDecider(),
		location:   location,
		windowDays: windowDays,
		scanLimit:  scanLimit,
		o11y:       o11y,
		dispatched: dispatched,
	}
}

func (uc *EvaluateInvoiceDueAlerts) Execute(ctx context.Context) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "card.usecase.evaluate_invoice_due_alerts")
	defer span.End()

	now := time.Now().UTC()
	loc := uc.location
	if loc == nil {
		loc = time.UTC
	}

	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return struct{}{}, uc.executeInTx(ctx, tx, now, loc)
	})
	if err != nil {
		span.RecordError(err)
		uc.o11y.Logger().Warn(ctx, "card.usecase.evaluate_invoice_due_alerts.failed",
			observability.Error(err),
		)
		return err
	}
	return nil
}

func (uc *EvaluateInvoiceDueAlerts) executeInTx(ctx context.Context, tx database.DBTX, now time.Time, loc *time.Location) error {
	cardRepo := uc.factory.CardRepository(tx)
	sentRepo := uc.factory.InvoiceDueAlertSentRepository(tx)

	cards, err := cardRepo.FindCardsWithInvoiceDueWithin(ctx, uc.windowDays, uc.scanLimit)
	if err != nil {
		return fmt.Errorf("card.usecase.evaluate_invoice_due_alerts: listar cartoes: %w", err)
	}
	if len(cards) == 0 {
		return nil
	}

	candidates := make([]services.InvoiceDueCandidate, 0, len(cards))
	for _, c := range cards {
		candidates = append(candidates, services.InvoiceDueCandidate{
			UserID:     c.UserID,
			CardID:     c.ID,
			CardName:   c.Name.String(),
			Cycle:      c.Cycle,
			LimitCents: c.LimitCents,
		})
	}

	alerts := uc.decider.Decide(candidates, uc.windowDays, now, loc)
	if len(alerts) == 0 {
		return nil
	}

	dueDates := make([]time.Time, 0, len(alerts))
	for _, a := range alerts {
		dueDates = append(dueDates, a.DueDate.UTC().Truncate(24*time.Hour))
	}
	sent, err := sentRepo.ListSentForDueDates(ctx, dueDates)
	if err != nil {
		return fmt.Errorf("card.usecase.evaluate_invoice_due_alerts: listar enviados: %w", err)
	}

	alreadySent := make(map[invoiceDueSentKey]struct{}, len(sent))
	for _, s := range sent {
		alreadySent[invoiceDueSentKey{
			userID:     s.UserID,
			cardID:     s.CardID,
			refDueDate: s.RefDueDate.UTC().Truncate(24 * time.Hour),
		}] = struct{}{}
	}

	for _, alert := range alerts {
		key := invoiceDueSentKey{
			userID:     alert.UserID,
			cardID:     alert.CardID,
			refDueDate: alert.DueDate.UTC().Truncate(24 * time.Hour),
		}
		if _, ok := alreadySent[key]; ok {
			continue
		}
		if err := uc.publishAndPersist(ctx, tx, sentRepo, alert, now); err != nil {
			return err
		}
	}
	return nil
}

func (uc *EvaluateInvoiceDueAlerts) publishAndPersist(
	ctx context.Context,
	tx database.DBTX,
	sentRepo interfaces.InvoiceDueAlertSentRepository,
	alert services.InvoiceDueAlert,
	now time.Time,
) error {
	if err := uc.publisher.Publish(ctx, tx, alert, now); err != nil {
		return fmt.Errorf("card.usecase.evaluate_invoice_due_alerts: publicar: %w", err)
	}
	if err := sentRepo.InsertSent(ctx, alert.UserID, alert.CardID, alert.DueDate); err != nil {
		return fmt.Errorf("card.usecase.evaluate_invoice_due_alerts: marcar enviado: %w", err)
	}
	uc.dispatched.Add(ctx, 1)
	uc.o11y.Logger().Info(ctx, "card.usecase.evaluate_invoice_due_alerts.dispatched",
		observability.String("card_id", alert.CardID.String()),
		observability.Int64("days_until", int64(alert.DaysUntil)),
	)
	return nil
}

type invoiceDueSentKey struct {
	userID     uuid.UUID
	cardID     uuid.UUID
	refDueDate time.Time
}
