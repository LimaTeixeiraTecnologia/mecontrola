package runtime

import (
	"context"
	"errors"
	"fmt"
)

// App define o contrato de ciclo de vida de uma aplicação.
// Bootstrap retorna uma implementação concreta de App.
type App interface {
	// Run inicia todos os subsistemas registrados. Bloqueia até que o contexto
	// seja cancelado ou um subsistema retorne erro.
	Run(ctx context.Context) error
	// Shutdown para todos os subsistemas na ordem inversa de inicialização.
	Shutdown(ctx context.Context) error
}

// Subsystem representa um componente gerenciado pelo runtime.
type Subsystem interface {
	// Start inicia o subsistema.
	Start(ctx context.Context) error
	// Stop para o subsistema graciosamente.
	Stop(ctx context.Context) error
	// Name retorna o identificador do subsistema (para logs).
	Name() string
}

// app é a implementação concreta de App.
type app struct {
	mode       AppMode
	subsystems []Subsystem
}

// Run inicia cada subsistema na ordem de registro. Em caso de erro, interrompe
// a sequência e retorna o erro com contexto do subsistema que falhou.
func (a *app) Run(ctx context.Context) error {
	for _, s := range a.subsystems {
		if err := s.Start(ctx); err != nil {
			return fmt.Errorf("iniciando subsistema %q: %w", s.Name(), err)
		}
	}
	return nil
}

// Shutdown para todos os subsistemas na ordem inversa de inicialização.
// Continua tentando parar os demais mesmo que um falhe, acumulando erros.
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
