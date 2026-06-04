package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

// TxRunner abstrai a execução de uma função dentro de uma transação Postgres.
// Permite substituição por implementação fake em testes unitários sem banco real.
// A implementação de produção delega para database.UnitOfWork[T].
type TxRunner[T any] interface {
	Do(ctx context.Context, fn func(ctx context.Context, tx database.DBTX) (T, error)) (T, error)
}
