package interfaces

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type CategoriesListPort interface {
	List(ctx context.Context, rawFilters json.RawMessage) (string, error)
}

type CategoriesGetPort interface {
	Get(ctx context.Context, rawFilters json.RawMessage) (string, error)
}

type CategoriesListDictionaryPort interface {
	ListDictionary(ctx context.Context, rawFilters json.RawMessage) (string, error)
}

type CategoriesSearchPort interface {
	Search(ctx context.Context, rawFilters json.RawMessage) (string, error)
}

type CardsListPort interface {
	List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type CardsGetPort interface {
	Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type CardsCreatePort interface {
	Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type CardsUpdatePort interface {
	Update(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type CardsDeletePort interface {
	Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type TransactionsListPort interface {
	List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type TransactionsGetPort interface {
	Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
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

type BudgetsGetPort interface {
	Get(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error)
}

type BudgetsCreatePort interface {
	Create(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type BudgetsUpdatePort interface {
	Update(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type BudgetsDeletePort interface {
	Delete(ctx context.Context, userID uuid.UUID, rawPayload json.RawMessage) (string, error)
}

type ModulePorts struct {
	Categories               CategoriesListPort
	CategoriesGet            CategoriesGetPort
	CategoriesListDictionary CategoriesListDictionaryPort
	CategoriesSearch         CategoriesSearchPort
	Cards                    CardsListPort
	CardsGet                 CardsGetPort
	CardsCreate              CardsCreatePort
	CardsUpdate              CardsUpdatePort
	CardsDelete              CardsDeletePort
	Transactions             TransactionsListPort
	TransactionsGet          TransactionsGetPort
	TransactionsCreate       TransactionsCreatePort
	TransactionsDelete       TransactionsDeletePort
	Budgets                  BudgetsListPort
	BudgetsGet               BudgetsGetPort
	BudgetsCreate            BudgetsCreatePort
	BudgetsUpdate            BudgetsUpdatePort
	BudgetsDelete            BudgetsDeletePort
}
