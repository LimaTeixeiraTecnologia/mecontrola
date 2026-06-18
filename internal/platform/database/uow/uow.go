package uow

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var ErrTransactionAlreadyFinished = errors.New("transaction has already been committed or rolled back")

type UnitOfWork interface {
	DBTX() database.DBTX
	Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error
}

type unitOfWork struct {
	db      *sqlx.DB
	options *sql.TxOptions
}

type UnitOfWorkOption func(*unitOfWork)

func WithIsolationLevel(level sql.IsolationLevel) UnitOfWorkOption {
	return func(u *unitOfWork) {
		if u.options == nil {
			u.options = &sql.TxOptions{}
		}
		u.options.Isolation = level
	}
}

func WithReadOnly(readOnly bool) UnitOfWorkOption {
	return func(u *unitOfWork) {
		if u.options == nil {
			u.options = &sql.TxOptions{}
		}
		u.options.ReadOnly = readOnly
	}
}

func NewUnitOfWork(db *sqlx.DB, opts ...UnitOfWorkOption) UnitOfWork {
	u := &unitOfWork{db: db, options: nil}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *unitOfWork) DBTX() database.DBTX {
	return u.db
}

func (u *unitOfWork) Do(ctx context.Context, fn func(ctx context.Context, db database.DBTX) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before transaction start: %w", err)
	}

	tx, err := u.db.BeginTx(ctx, u.options)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	var finished bool

	defer func() {
		if p := recover(); p != nil {
			if !finished {
				if rbErr := tx.Rollback(); rbErr != nil {
					if !errors.Is(rbErr, sql.ErrTxDone) {
						panic(fmt.Sprintf("panic during transaction with rollback failure: panic=%v, rollback_error=%v", p, rbErr))
					}
				}
			}
			panic(p)
		}
	}()

	if err = fn(database.WithTx(ctx, tx), tx); err != nil {
		finished = true
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	if err = ctx.Err(); err != nil {
		finished = true
		if rbErr := rollbackTx(tx); rbErr != nil {
			return fmt.Errorf("context cancelled during transaction: %w, rollback error: %v", err, rbErr)
		}
		return fmt.Errorf("context cancelled during transaction: %w", err)
	}

	finished = true
	if err = tx.Commit(); err != nil {
		if rbErr := rollbackTx(tx); rbErr != nil {
			if !errors.Is(rbErr, ErrTransactionAlreadyFinished) {
				return fmt.Errorf("commit error: %w, rollback error: %v", err, rbErr)
			}
		}
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func rollbackTx(tx *sql.Tx) error {
	if tx == nil {
		return ErrTransactionAlreadyFinished
	}

	if err := tx.Rollback(); err != nil {
		if errors.Is(err, sql.ErrTxDone) {
			return ErrTransactionAlreadyFinished
		}
		return err
	}

	return nil
}
