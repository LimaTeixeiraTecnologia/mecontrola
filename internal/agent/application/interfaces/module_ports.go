package interfaces

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type CategoriesListPort interface {
	List(ctx context.Context, rawFilters json.RawMessage) (string, error)
}

type CardsListPort interface {
	List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type CardsCreatePort interface {
	Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type TransactionsListPort interface {
	List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type TransactionsCreatePort interface {
	Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type TransactionsDeletePort interface {
	Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type BudgetsListPort interface {
	List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type ModulePorts struct {
	Categories         CategoriesListPort
	Cards              CardsListPort
	CardsCreate        CardsCreatePort
	Transactions       TransactionsListPort
	TransactionsCreate TransactionsCreatePort
	TransactionsDelete TransactionsDeletePort
	Budgets            BudgetsListPort
}
