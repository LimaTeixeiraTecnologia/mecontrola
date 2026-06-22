package usecases

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type onboardingUoWStub struct{}

func (u *onboardingUoWStub) DBTX() database.DBTX { return nil }

func (u *onboardingUoWStub) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type onboardingFixedIDGen struct {
	id string
}

func (f *onboardingFixedIDGen) NewID() string { return f.id }
