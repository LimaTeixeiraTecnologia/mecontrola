package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

// PgxUserRepository implementa interfaces.UserRepository usando pgx/v5
// via database.Manager. SoftDelete e LinkNewNumber abrem UnitOfWork interna.
type PgxUserRepository struct {
	manager     *database.Manager
	idGenerator interfaces.IDGenerator
}

// NewPgxUserRepository cria um PgxUserRepository pronto para uso.
func NewPgxUserRepository(
	manager *database.Manager,
	idGenerator interfaces.IDGenerator,
) *PgxUserRepository {
	return &PgxUserRepository{
		manager:     manager,
		idGenerator: idGenerator,
	}
}

// UpsertByWhatsAppNumber retorna o User ativo com o número, criando-o se não existir.
// Race condition de INSERT simultâneo retorna ErrDuplicateWhatsAppNumber.
func (r *PgxUserRepository) UpsertByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber, now time.Time) (*entities.User, error) {
	dbtx := r.manager.DBTX(ctx)
	mapper := rowMapper{}

	row := dbtx.QueryRowContext(ctx, querySelectUserByWhatsApp, number.String())
	existing, err := r.scanUser(row, mapper)
	if err != nil && !isNoRows(err) {
		return nil, fmt.Errorf("postgres user repository: upsert por whatsapp: %w", err)
	}

	if err == nil {
		return existing, nil
	}

	insertRow := dbtx.QueryRowContext(ctx, queryInsertUser,
		r.idGenerator.NewID(),
		number.String(),
		nil,
		nil,
		false,
		valueobjects.UserStatusActive.String(),
		now,
		now,
	)

	user, err := r.scanUser(insertRow, mapper)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrDuplicateWhatsAppNumber
		}
		return nil, fmt.Errorf("postgres user repository: upsert por whatsapp: %w", err)
	}
	return user, nil
}

// FindByID localiza um User ativo pelo seu ID. Retorna ErrUserNotFound quando ausente ou soft-deleted.
func (r *PgxUserRepository) FindByID(ctx context.Context, id entities.UserID) (*entities.User, error) {
	dbtx := r.manager.DBTX(ctx)
	mapper := rowMapper{}

	row := dbtx.QueryRowContext(ctx, querySelectUserByID, id.String())
	user, err := r.scanUser(row, mapper)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres user repository: find by id: %w", err)
	}
	return user, nil
}

// FindByWhatsAppNumber localiza um User ativo pelo número WhatsApp normalizado.
// Retorna ErrUserNotFound quando ausente ou soft-deleted.
func (r *PgxUserRepository) FindByWhatsAppNumber(ctx context.Context, number valueobjects.WhatsAppNumber) (*entities.User, error) {
	dbtx := r.manager.DBTX(ctx)
	mapper := rowMapper{}

	row := dbtx.QueryRowContext(ctx, querySelectUserByWhatsApp, number.String())
	user, err := r.scanUser(row, mapper)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("postgres user repository: find by whatsapp: %w", err)
	}
	return user, nil
}

// SoftDelete marca o User como deletado e desativa todos os registros em
// user_whatsapp_history (ADR-009). Retorna ErrUserNotFound se ausente ou já deletado.
func (r *PgxUserRepository) SoftDelete(ctx context.Context, id entities.UserID, now time.Time) error {
	unitOfWork := database.NewUnitOfWork[struct{}](r.manager)
	_, err := unitOfWork.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		result, err := tx.ExecContext(ctx, querySoftDeleteUser, now, id.String())
		if err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: soft delete: %w", err)
		}

		n, err := result.RowsAffected()
		if err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: soft delete rows affected: %w", err)
		}
		if n == 0 {
			return struct{}{}, ErrUserNotFound
		}

		if _, err := tx.ExecContext(ctx, queryDeactivateHistoryOnSoftDelete, now, id.String()); err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: soft delete history: %w", err)
		}

		return struct{}{}, nil
	})
	return err
}

// LinkNewNumber desativa o número atual, registra o histórico e atualiza
// users.whatsapp_number de forma atômica (ADR-010).
// Retorna ErrUserNotFound se o User não existe ou está soft-deleted.
func (r *PgxUserRepository) LinkNewNumber(ctx context.Context, id entities.UserID, number valueobjects.WhatsAppNumber, reason string, now time.Time) error {
	unitOfWork := database.NewUnitOfWork[struct{}](r.manager)
	_, err := unitOfWork.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		historyID := r.idGenerator.NewID()

		result, err := tx.ExecContext(ctx, queryUpdateUserWhatsApp, number.String(), now, id.String())
		if err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: link number update user: %w", err)
		}

		n, err := result.RowsAffected()
		if err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: link number rows affected: %w", err)
		}
		if n == 0 {
			return struct{}{}, ErrUserNotFound
		}

		if _, err := tx.ExecContext(ctx, queryDeactivateHistory, now, reason, id.String()); err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: link number deactivate history: %w", err)
		}

		if _, err := tx.ExecContext(ctx, queryInsertHistory, historyID, id.String(), number.String(), now); err != nil {
			return struct{}{}, fmt.Errorf("postgres user repository: link number insert history: %w", err)
		}

		return struct{}{}, nil
	})
	return err
}

func (r *PgxUserRepository) scanUser(row database.Row, mapper rowMapper) (*entities.User, error) {
	var ur userRow
	err := row.Scan(
		&ur.ID,
		&ur.WhatsAppNumber,
		&ur.DisplayName,
		&ur.Email,
		&ur.IsAdmin,
		&ur.Status,
		&ur.CreatedAt,
		&ur.UpdatedAt,
		&ur.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return mapper.HydrateUser(ur)
}
