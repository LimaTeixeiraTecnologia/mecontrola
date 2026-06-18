package uow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

func Do[T any](ctx context.Context, unit UnitOfWork, fn func(ctx context.Context, db database.DBTX) (T, error)) (T, error) {
	var result T
	err := unit.Do(ctx, func(ctx context.Context, db database.DBTX) error {
		value, fnErr := fn(ctx, db)
		if fnErr != nil {
			return fnErr
		}
		result = value
		return nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}
