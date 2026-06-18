package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkCard struct{}

func (_m *UnitOfWorkCard) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkCard) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkCard(t interface{ Cleanup(func()) }) *UnitOfWorkCard { return &UnitOfWorkCard{} }
