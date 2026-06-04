package database

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
)

const technology = "database"

type adapter struct {
	name   string
	runner consumer.Runner
}

func NewAdapter(name string, runner consumer.Runner) *adapter {
	return &adapter{
		name:   name,
		runner: runner,
	}
}

func (a *adapter) Name() string       { return a.name }
func (a *adapter) Technology() string { return technology }

func (a *adapter) Start(ctx context.Context) error {
	return a.runner.Start(ctx)
}

func (a *adapter) Stop(ctx context.Context) error {
	return a.runner.Stop(ctx)
}
