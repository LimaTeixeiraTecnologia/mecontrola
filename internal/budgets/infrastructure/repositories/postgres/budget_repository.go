package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type budgetRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewBudgetRepository(o11y observability.Observability, db database.DBTX) interfaces.BudgetRepository {
	return &budgetRepository{db: db, o11y: o11y}
}

func (r *budgetRepository) GetByUserCompetence(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) (entities.Budget, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.get_by_user_competence")
	defer span.End()

	const query = `
		SELECT b.id, b.user_id, b.competence, b.total_cents, b.state,
		       b.activated_at, b.auto_draft, b.created_at, b.updated_at,
		       a.root_slug, a.basis_points, a.planned_cents
		  FROM mecontrola.budgets b
		  LEFT JOIN mecontrola.budgets_allocations a ON a.budget_id = b.id
		 WHERE b.user_id = $1 AND b.competence = $2
		 ORDER BY a.root_slug
	`

	rows, err := r.db.QueryContext(ctx, query, userID, c.String())
	if err != nil {
		span.RecordError(err)
		return entities.Budget{}, fmt.Errorf("budgets/postgres: get_by_user_competence: %w", err)
	}
	defer func() { _ = rows.Close() }()

	budgets, err := r.scanBudgetList(rows)
	if err != nil {
		span.RecordError(err)
		return entities.Budget{}, err
	}
	if len(budgets) == 0 {
		return entities.Budget{}, fmt.Errorf("budgets/postgres: get_by_user_competence: %w", interfaces.ErrBudgetNotFound)
	}
	return budgets[0], nil
}

func (r *budgetRepository) CreateDraft(ctx context.Context, b entities.Budget) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.create_draft")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budgets
		       (id, user_id, competence, total_cents, state, activated_at, auto_draft, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		b.ID(), b.UserID(), b.Competence().String(), b.TotalCents(),
		int(b.State()), b.ActivatedAt(), b.AutoDraft(),
		b.CreatedAt(), b.UpdatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: create_draft: %w", err)
	}

	for _, alloc := range b.Allocations() {
		if allocErr := r.upsertAllocation(ctx, alloc); allocErr != nil {
			span.RecordError(allocErr)
			return fmt.Errorf("budgets/postgres: create_draft allocation: %w", allocErr)
		}
	}
	return nil
}

func (r *budgetRepository) Activate(ctx context.Context, b entities.Budget) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.activate")
	defer span.End()

	const query = `
		UPDATE mecontrola.budgets
		   SET state        = $1,
		       total_cents  = $2,
		       activated_at = $3,
		       updated_at   = $4
		 WHERE id = $5
	`

	result, err := r.db.ExecContext(ctx, query,
		int(b.State()), b.TotalCents(), b.ActivatedAt(), b.UpdatedAt(), b.ID(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: activate: %w", err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("budgets/postgres: activate rows_affected: %w", rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("budgets/postgres: activate: %w", interfaces.ErrBudgetNotFound)
	}

	for _, alloc := range b.Allocations() {
		if allocErr := r.upsertAllocation(ctx, alloc); allocErr != nil {
			span.RecordError(allocErr)
			return fmt.Errorf("budgets/postgres: activate allocation: %w", allocErr)
		}
	}
	return nil
}

func (r *budgetRepository) DeleteDraft(ctx context.Context, userID uuid.UUID, c valueobjects.Competence) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.delete_draft")
	defer span.End()

	const query = `
		DELETE FROM mecontrola.budgets
		 WHERE user_id = $1 AND competence = $2 AND state = $3
	`

	result, err := r.db.ExecContext(ctx, query, userID, c.String(), int(entities.BudgetStateDraft))
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: delete_draft: %w", err)
	}

	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("budgets/postgres: delete_draft rows_affected: %w", rowsErr)
	}
	if affected == 0 {
		return fmt.Errorf("budgets/postgres: delete_draft: %w", interfaces.ErrBudgetNotFound)
	}
	return nil
}

func (r *budgetRepository) ListFutureNotActivated(ctx context.Context, userID uuid.UUID, from valueobjects.Competence, max int) ([]entities.Budget, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.list_future_not_activated")
	defer span.End()

	const query = `
		SELECT b.id, b.user_id, b.competence, b.total_cents, b.state,
		       b.activated_at, b.auto_draft, b.created_at, b.updated_at,
		       a.root_slug, a.basis_points, a.planned_cents
		  FROM mecontrola.budgets b
		  LEFT JOIN mecontrola.budgets_allocations a ON a.budget_id = b.id
		 WHERE b.user_id = $1 AND b.competence > $2 AND b.state = $3
		 ORDER BY b.competence ASC, a.root_slug
		 LIMIT $4
	`

	rows, err := r.db.QueryContext(ctx, query, userID, from.String(), int(entities.BudgetStateDraft), max)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: list_future_not_activated: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result, err := r.scanBudgetList(rows)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (r *budgetRepository) ListAbandonedDrafts(ctx context.Context, before valueobjects.Competence, limit int) ([]entities.Budget, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.list_abandoned_drafts")
	defer span.End()

	const query = `
		SELECT b.id, b.user_id, b.competence, b.total_cents, b.state,
		       b.activated_at, b.auto_draft, b.created_at, b.updated_at,
		       a.root_slug, a.basis_points, a.planned_cents
		  FROM mecontrola.budgets b
		  LEFT JOIN mecontrola.budgets_allocations a ON a.budget_id = b.id
		 WHERE b.state = $1 AND b.competence < $2
		 ORDER BY b.competence ASC, a.root_slug
		 LIMIT $3
	`

	rows, err := r.db.QueryContext(ctx, query, int(entities.BudgetStateDraft), before.String(), limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("budgets/postgres: list_abandoned_drafts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result, err := r.scanBudgetList(rows)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	return result, nil
}

func (r *budgetRepository) SignalAbandoned(ctx context.Context, budgetID uuid.UUID) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.signal_abandoned")
	defer span.End()

	const query = `
		INSERT INTO mecontrola.budgets_abandoned_draft_signals (budget_id, signaled_at)
		VALUES ($1, now())
		ON CONFLICT (budget_id) DO NOTHING
	`

	_, err := r.db.ExecContext(ctx, query, budgetID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("budgets/postgres: signal_abandoned: %w", err)
	}
	return nil
}

func (r *budgetRepository) IsSignaledAbandoned(ctx context.Context, budgetID uuid.UUID) (bool, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "budgets.repository.budget.is_signaled_abandoned")
	defer span.End()

	const query = `
		SELECT COUNT(1) FROM mecontrola.budgets_abandoned_draft_signals WHERE budget_id = $1
	`

	var count int64
	if err := r.db.QueryRowContext(ctx, query, budgetID).Scan(&count); err != nil {
		span.RecordError(err)
		return false, fmt.Errorf("budgets/postgres: is_signaled_abandoned: %w", err)
	}
	return count > 0, nil
}

func (r *budgetRepository) upsertAllocation(ctx context.Context, a entities.Allocation) error {
	const query = `
		INSERT INTO mecontrola.budgets_allocations (budget_id, root_slug, basis_points, planned_cents)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (budget_id, root_slug) DO UPDATE
		   SET basis_points  = EXCLUDED.basis_points,
		       planned_cents = EXCLUDED.planned_cents
	`

	_, err := r.db.ExecContext(ctx, query, a.BudgetID(), a.RootSlug().String(), a.BasisPoints(), a.PlannedCents())
	if err != nil {
		return fmt.Errorf("upsert allocation %s: %w", a.RootSlug(), err)
	}
	return nil
}

func (r *budgetRepository) scanBudgetList(rows database.Rows) ([]entities.Budget, error) {
	byID := make(map[uuid.UUID]*entities.Budget)
	order := make([]uuid.UUID, 0)

	for rows.Next() {
		var (
			budgetID     uuid.UUID
			userID       uuid.UUID
			competence   string
			totalCents   int64
			state        int
			activatedAt  sql.NullTime
			autoDraft    bool
			createdAt    time.Time
			updatedAt    time.Time
			rootSlugStr  sql.NullString
			basisPoints  sql.NullInt32
			plannedCents sql.NullInt64
		)

		err := rows.Scan(
			&budgetID, &userID, &competence, &totalCents, &state,
			&activatedAt, &autoDraft, &createdAt, &updatedAt,
			&rootSlugStr, &basisPoints, &plannedCents,
		)
		if err != nil {
			return nil, fmt.Errorf("budgets/postgres: scan budget row: %w", err)
		}

		if _, seen := byID[budgetID]; !seen {
			comp, compErr := valueobjects.NewCompetence(competence)
			if compErr != nil {
				return nil, fmt.Errorf("budgets/postgres: parse competence %q: %w", competence, compErr)
			}

			var activatedAtPtr *time.Time
			if activatedAt.Valid {
				t := activatedAt.Time
				activatedAtPtr = &t
			}

			b := entities.HydrateBudget(
				budgetID, userID, comp, totalCents,
				entities.BudgetState(state), activatedAtPtr, autoDraft,
				nil, createdAt, updatedAt,
			)
			byID[budgetID] = &b
			order = append(order, budgetID)
		}

		if rootSlugStr.Valid && basisPoints.Valid && plannedCents.Valid {
			slug, slugErr := valueobjects.ParseRootSlug(rootSlugStr.String)
			if slugErr != nil {
				return nil, fmt.Errorf("budgets/postgres: parse root_slug %q: %w", rootSlugStr.String, slugErr)
			}
			b := byID[budgetID]
			alloc := entities.NewAllocation(budgetID, slug, int(basisPoints.Int32), plannedCents.Int64)
			b.SetAllocations(append(b.Allocations(), alloc))
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("budgets/postgres: budget rows iteration: %w", err)
	}

	result := make([]entities.Budget, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, nil
}
