package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

const defaultSearchLimit = 10

type searchTransactionsUseCase interface {
	Execute(ctx context.Context, query, refMonth string, limit int) ([]transactionsoutput.Transaction, error)
}

type TransactionSearcherAdapter struct {
	uc    searchTransactionsUseCase
	limit int
}

func NewTransactionSearcherAdapter(uc searchTransactionsUseCase) *TransactionSearcherAdapter {
	return &TransactionSearcherAdapter{uc: uc, limit: defaultSearchLimit}
}

func (a *TransactionSearcherAdapter) Execute(ctx context.Context, in tools.TransactionSearchInput) (tools.TransactionSearchResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return tools.TransactionSearchResult{}, fmt.Errorf("agent: transaction searcher: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, userID)

	limit := in.Limit
	if limit <= 0 {
		limit = a.limit
	}

	results, err := a.uc.Execute(ctx, in.Query, in.RefMonth, limit)
	if err != nil {
		return tools.TransactionSearchResult{}, fmt.Errorf("agent: transaction searcher: %w", err)
	}
	views := make([]tools.TransactionView, 0, len(results))
	for _, t := range results {
		views = append(views, transactionViewFrom(t))
	}
	return tools.TransactionSearchResult{Candidates: views}, nil
}
