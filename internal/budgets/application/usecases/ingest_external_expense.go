package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	budgetsconfig "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/config"
)

var ErrIngestExternalExpenseInvalidOperation = errors.New("budgets: operation inválida no evento externo")

var ErrIngestExternalExpenseInvalidEventID = errors.New("budgets: event_id inválido no evento externo")

var ErrIngestExternalExpenseInvalidUserID = errors.New("budgets: user_id inválido no evento externo")

var ErrIngestExternalExpenseInvalidSource = errors.New("budgets: source inválido no evento externo")

var ErrIngestExternalExpenseInvalidExternalID = errors.New("budgets: external_transaction_id inválido no evento externo")

var ErrIngestExternalExpenseInvalidFields = errors.New("budgets: evento externo com campos obrigatórios ausentes")

var ErrIngestExternalExpenseInvalidVersionForCreate = errors.New("budgets: evento de criação deve ter version=1")

var ErrIngestExternalExpenseSourceRejected = errors.New("budgets: source fora da allowlist de produtores autorizados")

type IngestExternalExpenseInput struct {
	EventID               string
	Source                string
	ExternalTransactionID string
	OccurredAt            time.Time
	UserID                string
	Operation             string
	Version               int64
	SubcategoryID         string
	Competence            string
	AmountCents           int64
}

type IngestExternalExpense struct {
	factory        interfaces.RepositoryFactory
	upsert         *UpsertExpense
	delete         *DeleteExpense
	uow            uow.UnitOfWork
	o11y           observability.Observability
	sourceRejected observability.Counter
	invalidFields  observability.Counter
}

func NewIngestExternalExpense(
	factory interfaces.RepositoryFactory,
	upsert *UpsertExpense,
	del *DeleteExpense,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *IngestExternalExpense {
	sourceRejected := o11y.Metrics().Counter(
		"budgets_external_expense_source_rejected_total",
		"Total de eventos externos rejeitados por source fora da allowlist",
		"1",
	)
	invalidFields := o11y.Metrics().Counter(
		"budgets_external_expense_invalid_fields_total",
		"Total de eventos externos rejeitados por campos obrigatórios ausentes ou versão inválida",
		"1",
	)
	return &IngestExternalExpense{
		factory:        factory,
		upsert:         upsert,
		delete:         del,
		uow:            u,
		o11y:           o11y,
		sourceRejected: sourceRejected,
		invalidFields:  invalidFields,
	}
}

func (uc *IngestExternalExpense) Execute(ctx context.Context, in IngestExternalExpenseInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.ingest_external_expense")
	defer span.End()

	if err := uc.preValidate(ctx, in); err != nil {
		return err
	}

	cmd, err := commands.NewIngestExternalExpenseCommand(
		in.EventID, in.UserID, in.Source, in.ExternalTransactionID,
		in.SubcategoryID, in.Competence, in.Operation, in.Version, in.AmountCents, in.OccurredAt,
	)
	if err != nil {
		return uc.mapCommandError(ctx, in, err)
	}

	if cmd.MutationKind == valueobjects.MutationKindCreate {
		return uc.applyCreate(ctx, in)
	}

	if err := uc.applyMutation(ctx, in, cmd.MutationKind); err == nil {
		return nil
	} else if !errors.Is(err, interfaces.ErrExpenseConflict) && !errors.Is(err, interfaces.ErrExpenseNotFound) {
		return err
	}

	return uc.queuePending(ctx, cmd, in)
}

func (uc *IngestExternalExpense) preValidate(ctx context.Context, in IngestExternalExpenseInput) error {
	if !budgetsconfig.IsAllowedProducerSource(in.Source) {
		uc.sourceRejected.Add(ctx, 1)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.source_rejected", observability.String("source", in.Source))
		return ErrIngestExternalExpenseSourceRejected
	}

	if reason := missingFieldReason(in); reason != "" {
		uc.invalidFields.Add(ctx, 1, observability.String("reason", reason))
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.invalid_fields",
			observability.String("event_id", in.EventID),
			observability.String("reason", reason),
		)
		return fmt.Errorf("%w: %s", ErrIngestExternalExpenseInvalidFields, reason)
	}

	return nil
}

func (uc *IngestExternalExpense) mapCommandError(ctx context.Context, in IngestExternalExpenseInput, err error) error {
	switch {
	case errors.Is(err, commands.ErrCommandInvalidEventID):
		return ErrIngestExternalExpenseInvalidEventID
	case errors.Is(err, commands.ErrCommandInvalidUserID):
		return ErrIngestExternalExpenseInvalidUserID
	case errors.Is(err, commands.ErrCommandInvalidSource):
		return ErrIngestExternalExpenseInvalidSource
	case errors.Is(err, commands.ErrCommandInvalidExternalID):
		return ErrIngestExternalExpenseInvalidExternalID
	case errors.Is(err, commands.ErrCommandInvalidMutationKind):
		return ErrIngestExternalExpenseInvalidOperation
	case errors.Is(err, commands.ErrCommandVersionRequired):
		uc.invalidFields.Add(ctx, 1, observability.String("reason", "create_version_not_one"))
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.invalid_version_for_create",
			observability.String("event_id", in.EventID),
			observability.Int64("version", in.Version),
		)
		return ErrIngestExternalExpenseInvalidVersionForCreate
	case errors.Is(err, commands.ErrCommandInvalidAmount):
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.invalid_amount",
			observability.String("event_id", in.EventID),
			observability.Int64("amount_cents", in.AmountCents),
		)
		return ErrUpsertExpenseInvalidAmount
	}
	return err
}

func (uc *IngestExternalExpense) applyCreate(ctx context.Context, in IngestExternalExpenseInput) error {
	_, err := uc.upsert.Execute(ctx, input.UpsertExpenseInput{
		UserID:                in.UserID,
		Source:                in.Source,
		ExternalTransactionID: in.ExternalTransactionID,
		SubcategoryID:         in.SubcategoryID,
		Competence:            in.Competence,
		AmountCents:           in.AmountCents,
		OccurredAt:            in.OccurredAt,
	})
	if err == nil || errors.Is(err, interfaces.ErrExpenseConflict) {
		return nil
	}
	return fmt.Errorf("budgets.usecase.ingest_external_expense: upsert direto: %w", err)
}

func (uc *IngestExternalExpense) applyMutation(ctx context.Context, in IngestExternalExpenseInput, mutationKind valueobjects.MutationKind) error {
	expectedVersion := in.Version - 1
	if mutationKind == valueobjects.MutationKindDelete {
		err := uc.delete.Execute(ctx, input.DeleteExpenseInput{
			UserID:                in.UserID,
			Source:                in.Source,
			ExternalTransactionID: in.ExternalTransactionID,
			ExpectedVersion:       expectedVersion,
		})
		if err == nil {
			return nil
		}
		return fmt.Errorf("budgets.usecase.ingest_external_expense: aplicar direto: %w", err)
	}

	_, err := uc.upsert.Execute(ctx, input.UpsertExpenseInput{
		UserID:                in.UserID,
		Source:                in.Source,
		ExternalTransactionID: in.ExternalTransactionID,
		SubcategoryID:         in.SubcategoryID,
		Competence:            in.Competence,
		AmountCents:           in.AmountCents,
		OccurredAt:            in.OccurredAt,
		ExpectedVersion:       &expectedVersion,
	})
	if err == nil {
		return nil
	}
	return fmt.Errorf("budgets.usecase.ingest_external_expense: aplicar direto: %w", err)
}

func (uc *IngestExternalExpense) queuePending(ctx context.Context, cmd commands.IngestExternalExpenseCommand, in IngestExternalExpenseInput) error {
	pendingPayload, marshalErr := buildPendingPayload(in)
	if marshalErr != nil {
		return fmt.Errorf("budgets.usecase.ingest_external_expense: serializar payload pendente: %w", marshalErr)
	}

	pendingEvt := entities.NewPendingEvent(
		cmd.EventID,
		cmd.Source,
		cmd.UserID,
		cmd.ExtID,
		cmd.Version,
		cmd.MutationKind,
		pendingPayload,
		time.Now().UTC(),
	)

	_, insertErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		pending := uc.factory.PendingEventRepository(tx)
		if err := pending.Insert(ctx, pendingEvt); err != nil {
			if errors.Is(err, interfaces.ErrPendingEventDuplicate) {
				return struct{}{}, nil
			}
			return struct{}{}, fmt.Errorf("budgets.usecase.ingest_external_expense: inserir pendente: %w", err)
		}
		return struct{}{}, nil
	})
	return insertErr
}

func missingFieldReason(in IngestExternalExpenseInput) string {
	switch {
	case in.EventID == "":
		return "event_id_empty"
	case in.Source == "":
		return "source_empty"
	case in.ExternalTransactionID == "":
		return "external_transaction_id_empty"
	case in.OccurredAt.IsZero():
		return "occurred_at_zero"
	case in.UserID == "":
		return "user_id_empty"
	case in.Operation == "":
		return "operation_empty"
	case in.Version == 0:
		return "version_zero"
	default:
		return ""
	}
}

func buildPendingPayload(in IngestExternalExpenseInput) ([]byte, error) {
	type pendingPayload struct {
		SubcategoryID string    `json:"subcategory_id"`
		Competence    string    `json:"competence"`
		AmountCents   int64     `json:"amount_cents"`
		OccurredAt    time.Time `json:"occurred_at"`
	}
	return json.Marshal(pendingPayload{
		SubcategoryID: in.SubcategoryID,
		Competence:    in.Competence,
		AmountCents:   in.AmountCents,
		OccurredAt:    in.OccurredAt,
	})
}
