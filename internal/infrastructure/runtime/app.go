package runtime

import (
	"context"
	"errors"
	"fmt"
)

type App interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Subsystem interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string
}

type app struct {
	mode       AppMode
	subsystems []Subsystem
}

func (a *app) Run(ctx context.Context) error {
	for _, s := range a.subsystems {
		if err := s.Start(ctx); err != nil {
			return fmt.Errorf("iniciando subsistema %q: %w", s.Name(), err)
		}
	}
	return nil
}

// Shutdown continua tentando parar os demais subsistemas mesmo que um falhe, acumulando erros.
func (a *app) Shutdown(ctx context.Context) error {
	var errs []error
	for i := len(a.subsystems) - 1; i >= 0; i-- {
		s := a.subsystems[i]
		if err := s.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("parando subsistema %q: %w", s.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("erros durante shutdown: %w", errors.Join(errs...))
	}
	return nil
}
