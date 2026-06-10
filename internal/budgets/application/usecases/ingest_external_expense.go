package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
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
	pending        interfaces.PendingEventRepository
	upsert         *UpsertExpense
	delete         *DeleteExpense
	uow            uow.UnitOfWork[struct{}]
	o11y           observability.Observability
	sourceRejected observability.Counter
	invalidFields  observability.Counter
}

func NewIngestExternalExpense(
	pending interfaces.PendingEventRepository,
	upsert *UpsertExpense,
	del *DeleteExpense,
	u uow.UnitOfWork[struct{}],
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
		pending:        pending,
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

	if !budgetsconfig.IsAllowedProducerSource(in.Source) {
		uc.sourceRejected.Add(ctx, 1)
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.source_rejected",
			observability.String("source", in.Source),
		)
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

	mutationKind, err := valueobjects.ParseMutationKind(in.Operation)
	if err != nil {
		return ErrIngestExternalExpenseInvalidOperation
	}

	if mutationKind == valueobjects.MutationKindCreate && in.Version != 1 {
		uc.invalidFields.Add(ctx, 1, observability.String("reason", "create_version_not_one"))
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.invalid_version_for_create",
			observability.String("event_id", in.EventID),
			observability.Int64("version", in.Version),
		)
		return ErrIngestExternalExpenseInvalidVersionForCreate
	}

	if mutationKind != valueobjects.MutationKindDelete && in.AmountCents <= 0 {
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.ingest_external_expense.invalid_amount",
			observability.String("event_id", in.EventID),
			observability.Int64("amount_cents", in.AmountCents),
		)
		return ErrUpsertExpenseInvalidAmount
	}

	eventID, err := uuid.Parse(in.EventID)
	if err != nil {
		return ErrIngestExternalExpenseInvalidEventID
	}

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return ErrIngestExternalExpenseInvalidUserID
	}

	source, err := valueobjects.NewProducerSource(in.Source)
	if err != nil {
		return ErrIngestExternalExpenseInvalidSource
	}

	extID, err := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	if err != nil {
		return ErrIngestExternalExpenseInvalidExternalID
	}

	if mutationKind == valueobjects.MutationKindCreate {
		_, execErr := uc.upsert.Execute(ctx, input.UpsertExpenseInput{
			UserID:                in.UserID,
			Source:                in.Source,
			ExternalTransactionID: in.ExternalTransactionID,
			SubcategoryID:         in.SubcategoryID,
			Competence:            in.Competence,
			AmountCents:           in.AmountCents,
			OccurredAt:            in.OccurredAt,
		})
		if execErr == nil || errors.Is(execErr, interfaces.ErrExpenseConflict) {
			return nil
		}
		return fmt.Errorf("budgets.usecase.ingest_external_expense: upsert direto: %w", execErr)
	}

	if mutationKind != valueobjects.MutationKindCreate {
		expectedVersion := in.Version - 1
		var execErr error
		if mutationKind == valueobjects.MutationKindDelete {
			execErr = uc.delete.Execute(ctx, input.DeleteExpenseInput{
				UserID:                in.UserID,
				Source:                in.Source,
				ExternalTransactionID: in.ExternalTransactionID,
				ExpectedVersion:       expectedVersion,
			})
		} else {
			_, execErr = uc.upsert.Execute(ctx, input.UpsertExpenseInput{
				UserID:                in.UserID,
				Source:                in.Source,
				ExternalTransactionID: in.ExternalTransactionID,
				SubcategoryID:         in.SubcategoryID,
				Competence:            in.Competence,
				AmountCents:           in.AmountCents,
				OccurredAt:            in.OccurredAt,
				ExpectedVersion:       &expectedVersion,
			})
		}
		if execErr == nil {
			return nil
		}
		if !errors.Is(execErr, interfaces.ErrExpenseConflict) && !errors.Is(execErr, interfaces.ErrExpenseNotFound) {
			return fmt.Errorf("budgets.usecase.ingest_external_expense: aplicar direto: %w", execErr)
		}
	}

	pendingPayload, marshalErr := uc.buildPendingPayload(in)
	if marshalErr != nil {
		return fmt.Errorf("budgets.usecase.ingest_external_expense: serializar payload pendente: %w", marshalErr)
	}

	pendingEvt := entities.NewPendingEvent(
		eventID,
		source,
		userID,
		extID,
		in.Version,
		mutationKind,
		pendingPayload,
		time.Now().UTC(),
	)

	_, insertErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		if err := uc.pending.Insert(ctx, tx, pendingEvt); err != nil {
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

func (uc *IngestExternalExpense) buildPendingPayload(in IngestExternalExpenseInput) ([]byte, error) {
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
