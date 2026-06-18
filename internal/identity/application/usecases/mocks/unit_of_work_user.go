package mocks

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type UnitOfWorkUser struct{}

func (_m *UnitOfWorkUser) DBTX() database.DBTX { return nil }

func (_m *UnitOfWorkUser) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

func NewUnitOfWorkUser(t interface{ Cleanup(func()) }) *UnitOfWorkUser { return &UnitOfWorkUser{} }
