package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

type CreateTransaction struct {
	factory           interfaces.RepositoryFactory
	uow               uow.UnitOfWork
	categoryValidator interfaces.CategoryValidator
	workflow          services.TransactionWorkflow
	publisher         interfaces.TransactionEventPublisher
	o11y              observability.Observability
}

func NewCreateTransaction(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	categoryValidator interfaces.CategoryValidator,
	workflow services.TransactionWorkflow,
	publisher interfaces.TransactionEventPublisher,
	o11y observability.Observability,
) *CreateTransaction {
	return &CreateTransaction{
		factory:           factory,
		uow:               u,
		categoryValidator: categoryValidator,
		workflow:          workflow,
		publisher:         publisher,
		o11y:              o11y,
	}
}

func (uc *CreateTransaction) Execute(ctx context.Context, raw input.RawCreateTransaction) (output.Transaction, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "transactions.usecase.create_transaction")
	defer span.End()

	principal, ok := auth.FromContext(ctx)
	if !ok {
		return output.Transaction{}, ErrUsecaseUnauthorized
	}

	cmd, err := commands.NewCreateTransaction(toCommandRawCreate(raw), principal.UserID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/create_transaction: comando: %w", err)
	}

	catSubID := optSubcategoryUUID(cmd.SubcategoryID)
	catSnap, err := uc.categoryValidator.Validate(ctx, cmd.CategoryID.UUID(), catSubID)
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, fmt.Errorf("transactions/create_transaction: validar categoria: %w", err)
	}

	txID := uuid.New()
	eventID := uuid.New()
	now := time.Now().UTC()

	decision := uc.workflow.DecideCreate(cmd, txID, eventID, now)
	decision.Transaction.SetCategorySnapshots(catSnap.Name, snapSubName(catSubID, catSnap))

	tx, err := uow.Do(ctx, uc.uow, func(ctx context.Context, db database.DBTX) (entities.Transaction, error) {
		repo := uc.factory.TransactionRepository(db)
		if createErr := repo.Create(ctx, &decision.Transaction); createErr != nil {
			return entities.Transaction{}, fmt.Errorf("transactions/create_transaction: persistir: %w", createErr)
		}
		if created, ok := decision.Event.(entities.TransactionCreated); ok {
			if publishErr := uc.publisher.PublishCreated(ctx, db, created); publishErr != nil {
				return entities.Transaction{}, fmt.Errorf("transactions/create_transaction: publicar evento: %w", publishErr)
			}
		}
		return decision.Transaction, nil
	})
	if err != nil {
		span.RecordError(err)
		return output.Transaction{}, err
	}

	return output.TransactionFrom(&tx), nil
}
