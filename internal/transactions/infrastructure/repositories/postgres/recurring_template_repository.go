package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

var ErrRecurringTemplateNotFound = errors.New("transactions: template recorrente não encontrado")
var ErrRecurringTemplateVersionConflict = errors.New("transactions: conflito de versão no template recorrente")

type recurringTemplateRepository struct {
	db   database.DBTX
	o11y observability.Observability
}

func NewRecurringTemplateRepository(o11y observability.Observability, db database.DBTX) interfaces.RecurringTemplateRepository {
	return &recurringTemplateRepository{db: db, o11y: o11y}
}

func (r *recurringTemplateRepository) Create(ctx context.Context, t *entities.RecurringTemplate) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.create")
	defer span.End()

	const q = `
		INSERT INTO mecontrola.transactions_recurring_templates
			(id, user_id, direction, payment_method, card_id, amount_cents, description,
			 category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
			 frequency, day_of_month, installments_total, started_at, ended_at,
			 version, deleted_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`

	var cardID *uuid.UUID
	if cid, ok := t.CardID().Get(); ok {
		v := cid.UUID()
		cardID = &v
	}

	var subID *uuid.UUID
	if sub, ok := t.SubcategoryID().Get(); ok {
		v := sub.UUID()
		subID = &v
	}

	var endedAt *time.Time
	if ea, ok := t.EndedAt().Get(); ok {
		endedAt = &ea
	}

	_, err := r.db.ExecContext(ctx, q,
		t.ID(), t.UserID().UUID(), int(t.Direction()), int(t.PaymentMethod()),
		cardID, t.Amount().Cents(), t.Description().String(),
		t.CategoryID().UUID(), subID,
		t.CategoryNameSnapshot(), t.SubcategoryNameSnapshot(),
		int(t.Frequency()), t.DayOfMonth().Value(), t.InstallmentsTotal().Value(),
		t.StartedAt(), endedAt,
		t.Version(), t.DeletedAt(), t.CreatedAt(), t.UpdatedAt(),
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: criar template recorrente: %w", err)
	}
	return nil
}

func (r *recurringTemplateRepository) UpdateWithVersion(ctx context.Context, t *entities.RecurringTemplate, expectedVersion int64) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.update")
	defer span.End()

	const q = `
		UPDATE mecontrola.transactions_recurring_templates
		   SET direction=$1, payment_method=$2, card_id=$3, amount_cents=$4, description=$5,
		       category_id=$6, subcategory_id=$7, category_name_snapshot=$8, subcategory_name_snapshot=$9,
		       frequency=$10, day_of_month=$11, installments_total=$12, started_at=$13, ended_at=$14,
		       version=$15, updated_at=$16
		 WHERE id=$17 AND user_id=$18 AND version=$19 AND deleted_at IS NULL
	`

	var cardID *uuid.UUID
	if cid, ok := t.CardID().Get(); ok {
		v := cid.UUID()
		cardID = &v
	}

	var subID *uuid.UUID
	if sub, ok := t.SubcategoryID().Get(); ok {
		v := sub.UUID()
		subID = &v
	}

	var endedAt *time.Time
	if ea, ok := t.EndedAt().Get(); ok {
		endedAt = &ea
	}

	res, err := r.db.ExecContext(ctx, q,
		int(t.Direction()), int(t.PaymentMethod()), cardID, t.Amount().Cents(), t.Description().String(),
		t.CategoryID().UUID(), subID, t.CategoryNameSnapshot(), t.SubcategoryNameSnapshot(),
		int(t.Frequency()), t.DayOfMonth().Value(), t.InstallmentsTotal().Value(),
		t.StartedAt(), endedAt,
		t.Version(), t.UpdatedAt(),
		t.ID(), t.UserID().UUID(), expectedVersion,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: atualizar template recorrente: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: rows affected template recorrente: %w", err)
	}
	if rows == 0 {
		return ErrRecurringTemplateVersionConflict
	}
	return nil
}

func (r *recurringTemplateRepository) SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.soft_delete")
	defer span.End()

	const q = `
		UPDATE mecontrola.transactions_recurring_templates
		   SET deleted_at=$1, version=version+1, updated_at=$2
		 WHERE id=$3 AND user_id=$4 AND version=$5 AND deleted_at IS NULL
	`

	res, err := r.db.ExecContext(ctx, q, now, now, id, userID, expectedVersion)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: soft-delete template recorrente: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("transactions/repository: rows affected soft-delete template recorrente: %w", err)
	}
	if rows == 0 {
		return ErrRecurringTemplateVersionConflict
	}
	return nil
}

func (r *recurringTemplateRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.RecurringTemplate, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.get_by_id")
	defer span.End()

	const q = `
		SELECT id, user_id, direction, payment_method, card_id, amount_cents, description,
		       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		       frequency, day_of_month, installments_total, started_at, ended_at,
		       version, deleted_at, created_at, updated_at
		  FROM mecontrola.transactions_recurring_templates
		 WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL
	`

	rows, err := r.db.QueryContext(ctx, q, id, userID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("transactions/repository: buscar template recorrente: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("transactions/repository: rows template recorrente: %w", err)
		}
		return nil, ErrRecurringTemplateNotFound
	}

	t, err := r.scan(rows)
	if err != nil {
		return nil, err
	}
	return t, rows.Err()
}

func (r *recurringTemplateRepository) List(ctx context.Context, userID uuid.UUID, activeOnly bool, cursor interfaces.Cursor, limit int) ([]*entities.RecurringTemplate, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.list")
	defer span.End()

	base := `
		SELECT id, user_id, direction, payment_method, card_id, amount_cents, description,
		       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		       frequency, day_of_month, installments_total, started_at, ended_at,
		       version, deleted_at, created_at, updated_at
		  FROM mecontrola.transactions_recurring_templates
		 WHERE user_id=$1 AND deleted_at IS NULL
	`

	args := []any{userID}
	idx := 2

	if activeOnly {
		base += " AND (ended_at IS NULL OR ended_at >= NOW())"
	}

	if cursor.Value != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(cursor.Value)
		if decErr == nil {
			base += fmt.Sprintf(" AND created_at < $%d", idx)
			args = append(args, string(decoded))
			idx++
		}
	}

	base += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", idx)
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, base, args...)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: listar templates recorrentes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.RecurringTemplate
	for rows.Next() {
		t, scanErr := r.scan(rows)
		if scanErr != nil {
			return nil, interfaces.Cursor{}, scanErr
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: rows listar templates: %w", err)
	}

	var nextCursor interfaces.Cursor
	if len(result) > limit {
		last := result[limit-1]
		nextCursor.Value = base64.StdEncoding.EncodeToString([]byte(last.CreatedAt().Format(time.RFC3339Nano)))
		result = result[:limit]
	}

	return result, nextCursor, nil
}

func (r *recurringTemplateRepository) FindActiveByDayOfMonth(ctx context.Context, day int, asOf time.Time, cursor interfaces.Cursor, batchSize int) ([]*entities.RecurringTemplate, interfaces.Cursor, error) {
	ctx, span := r.o11y.Tracer().Start(ctx, "transactions.repository.recurring_template.find_active_by_day_of_month")
	defer span.End()

	base := `
		SELECT id, user_id, direction, payment_method, card_id, amount_cents, description,
		       category_id, subcategory_id, category_name_snapshot, subcategory_name_snapshot,
		       frequency, day_of_month, installments_total, started_at, ended_at,
		       version, deleted_at, created_at, updated_at
		  FROM mecontrola.transactions_recurring_templates
		 WHERE day_of_month=$1
		   AND deleted_at IS NULL
		   AND started_at <= $2
		   AND (ended_at IS NULL OR ended_at >= $2)
	`

	args := []any{day, asOf}
	idx := 3

	if cursor.Value != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(cursor.Value)
		if decErr == nil {
			base += fmt.Sprintf(" AND id > $%d", idx)
			args = append(args, string(decoded))
			idx++
		}
	}

	base += fmt.Sprintf(" ORDER BY id LIMIT $%d", idx)
	args = append(args, batchSize+1)

	rows, err := r.db.QueryContext(ctx, base, args...)
	if err != nil {
		span.RecordError(err)
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: buscar ativos por dia: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []*entities.RecurringTemplate
	for rows.Next() {
		t, scanErr := r.scan(rows)
		if scanErr != nil {
			return nil, interfaces.Cursor{}, scanErr
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, interfaces.Cursor{}, fmt.Errorf("transactions/repository: rows ativos por dia: %w", err)
	}

	var nextCursor interfaces.Cursor
	if len(result) > batchSize {
		last := result[batchSize-1]
		nextCursor.Value = base64.StdEncoding.EncodeToString([]byte(last.ID().String()))
		result = result[:batchSize]
	}

	return result, nextCursor, nil
}

func (r *recurringTemplateRepository) scan(rows database.Rows) (*entities.RecurringTemplate, error) {
	var (
		id                      uuid.UUID
		userID                  uuid.UUID
		direction               int
		paymentMethod           int
		cardIDPtr               *uuid.UUID
		amountCents             int64
		description             string
		categoryID              uuid.UUID
		subcategoryIDPtr        *uuid.UUID
		categoryNameSnapshot    string
		subcategoryNameSnapshot string
		frequency               int
		dayOfMonth              int
		installmentsTotal       int
		startedAt               time.Time
		endedAtPtr              *time.Time
		version                 int64
		deletedAt               *time.Time
		createdAt               time.Time
		updatedAt               time.Time
	)

	if err := rows.Scan(
		&id, &userID, &direction, &paymentMethod, &cardIDPtr, &amountCents, &description,
		&categoryID, &subcategoryIDPtr, &categoryNameSnapshot, &subcategoryNameSnapshot,
		&frequency, &dayOfMonth, &installmentsTotal, &startedAt, &endedAtPtr,
		&version, &deletedAt, &createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecurringTemplateNotFound
		}
		return nil, fmt.Errorf("transactions/repository: scan template recorrente: %w", err)
	}

	dir, _ := valueobjects.DirectionFromInt(direction)
	pm, _ := valueobjects.PaymentMethodFromInt(paymentMethod)
	amount, _ := valueobjects.NewMoney(amountCents)
	desc, _ := valueobjects.NewDescription(description)
	catID := valueobjects.CategoryIDFromUUID(categoryID)
	freq, _ := valueobjects.FrequencyFromInt(frequency)
	dom, _ := valueobjects.NewDayOfMonth(dayOfMonth)
	inst, _ := valueobjects.NewInstallmentCount(installmentsTotal)

	var cardIDOpt option.Option[valueobjects.CardID]
	if cardIDPtr != nil {
		cardIDOpt = option.Some(valueobjects.CardIDFromUUID(*cardIDPtr))
	}

	var subOpt option.Option[valueobjects.SubcategoryID]
	if subcategoryIDPtr != nil {
		subOpt = option.Some(valueobjects.SubcategoryIDFromUUID(*subcategoryIDPtr))
	}

	var endedAtOpt option.Option[time.Time]
	if endedAtPtr != nil {
		endedAtOpt = option.Some(*endedAtPtr)
	}

	t := entities.ReconstituteRecurringTemplate(
		id,
		valueobjects.UserIDFromUUID(userID),
		dir, pm,
		cardIDOpt,
		amount, desc, catID,
		subOpt,
		categoryNameSnapshot, subcategoryNameSnapshot,
		freq, dom, inst,
		startedAt, endedAtOpt,
		version, deletedAt, createdAt, updatedAt,
	)
	return &t, nil
}
