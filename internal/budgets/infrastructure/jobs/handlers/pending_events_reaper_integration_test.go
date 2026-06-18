//go:build integration

package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

func TestPendingEventsReaperIntegration_PendingEventIsProcessed(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)

	catReader := mocks.NewCategoriesReader(t)
	catReader.On("ValidateExpenseSubcategory", mock.Anything, mock.Anything).
		Return("expense.prazeres", false, nil).Maybe()
	catReader.On("EditorialVersion", mock.Anything).Return(int64(1), nil).Maybe()

	expPub := mocks.NewExpenseCommittedPublisher(t)
	expPub.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(factory)
	upsertExpense := usecases.NewUpsertExpense(factory, catReader, expPub, autoDraft, uow.NewUnitOfWork(db), o11y, time.UTC)
	deleteExpense := usecases.NewDeleteExpense(factory, expPub, uow.NewUnitOfWork(db), o11y, time.UTC)
	applyPending := usecases.NewApplyPendingEvent(factory, upsertExpense, deleteExpense, 72*time.Hour, o11y)
	runReaper := usecases.NewRunPendingEventsReaper(factory, applyPending, unitOfWork, o11y)
	job := handlers.NewPendingEventsReaper(runReaper, configs.BudgetsConfig{PendingReaperInterval: "@every 1m"})

	pendingRepo := factory.PendingEventRepository(db)
	subcategoryID := uuid.New()
	payload := []byte(`{"subcategory_id":"` + subcategoryID.String() + `","competence":"2026-06","amount_cents":5000,"occurred_at":"2026-06-01T12:00:00Z"}`)
	evt := entities.NewPendingEvent(
		uuid.New(),
		mustProducerSource(t, "api"),
		uuid.New(),
		mustExternalTransactionID(t, uuid.New().String()),
		0,
		valueobjects.MutationKindCreate,
		payload,
		time.Now().UTC(),
	)
	require.NoError(t, pendingRepo.Insert(ctx, evt))

	require.NoError(t, job.Run(ctx))

	var state int
	err := db.QueryRowContext(ctx,
		`SELECT state FROM mecontrola.budgets_expense_events_pending WHERE id = $1`,
		evt.ID(),
	).Scan(&state)
	require.NoError(t, err)
	require.NotEqual(t, int(entities.PendingStatePending), state)
}

func TestPendingEventsReaperIntegration_AlreadyAppliedEventIsNotReprocessed(t *testing.T) {
	db, _ := testcontainer.Postgres(t)
	o11y := noop.NewProvider()
	ctx := context.Background()

	factory := repositories.NewRepositoryFactory(o11y)
	unitOfWork := uow.NewUnitOfWork(db)

	catReader := mocks.NewCategoriesReader(t)
	catReader.On("ValidateExpenseSubcategory", mock.Anything, mock.Anything).
		Return("expense.prazeres", false, nil).Maybe()
	catReader.On("EditorialVersion", mock.Anything).Return(int64(1), nil).Maybe()

	expPub := mocks.NewExpenseCommittedPublisher(t)
	expPub.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	autoDraft := usecases.NewCreateOrAutoDraftForExpense(factory)
	upsertExpense := usecases.NewUpsertExpense(factory, catReader, expPub, autoDraft, uow.NewUnitOfWork(db), o11y, time.UTC)
	deleteExpense := usecases.NewDeleteExpense(factory, expPub, uow.NewUnitOfWork(db), o11y, time.UTC)
	applyPending := usecases.NewApplyPendingEvent(factory, upsertExpense, deleteExpense, 72*time.Hour, o11y)
	runReaper := usecases.NewRunPendingEventsReaper(factory, applyPending, unitOfWork, o11y)
	job := handlers.NewPendingEventsReaper(runReaper, configs.BudgetsConfig{PendingReaperInterval: "@every 1m"})

	evtID := uuid.New()
	userID := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO mecontrola.budgets_expense_events_pending
		        (id, event_id, source, user_id, external_transaction_id, expected_version, mutation_kind, payload, state, received_at, transitioned_at, reason)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		uuid.New(), evtID, "api", userID, uuid.New().String(),
		0, int(valueobjects.MutationKindCreate), []byte(`{}`),
		int(entities.PendingStateApplied), time.Now().UTC(), time.Now().UTC(), "applied",
	)
	require.NoError(t, err)

	require.NoError(t, job.Run(ctx))

	var state int
	err = db.QueryRowContext(ctx,
		`SELECT state FROM mecontrola.budgets_expense_events_pending WHERE event_id = $1`,
		evtID,
	).Scan(&state)
	require.NoError(t, err)
	require.Equal(t, int(entities.PendingStateApplied), state)
}

func mustProducerSource(t *testing.T, s string) valueobjects.ProducerSource {
	t.Helper()
	ps, err := valueobjects.NewProducerSource(s)
	require.NoError(t, err)
	return ps
}

func mustExternalTransactionID(t *testing.T, s string) valueobjects.ExternalTransactionID {
	t.Helper()
	e, err := valueobjects.NewExternalTransactionID(s)
	require.NoError(t, err)
	return e
}
