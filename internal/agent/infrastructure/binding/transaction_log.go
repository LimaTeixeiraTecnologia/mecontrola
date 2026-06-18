package binding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

type transactionsCreateUseCase interface {
	Execute(ctx context.Context, raw transactionsinput.RawCreateTransaction) (transactionsoutput.Transaction, error)
}

type TransactionCreatorAdapter struct {
	uc transactionsCreateUseCase
}

func NewTransactionCreatorAdapter(uc transactionsCreateUseCase) *TransactionCreatorAdapter {
	return &TransactionCreatorAdapter{uc: uc}
}

func (a *TransactionCreatorAdapter) Execute(ctx context.Context, in usecases.CreateTransactionCommand) (usecases.CreateTransactionResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return usecases.CreateTransactionResult{}, fmt.Errorf("agent: transaction creator: user id: %w", err)
	}
	rootID, err := uuid.Parse(strings.TrimSpace(in.RootCategoryID))
	if err != nil {
		return usecases.CreateTransactionResult{}, fmt.Errorf("agent: transaction creator: category id: %w", err)
	}
	var subID *uuid.UUID
	if trimmed := strings.TrimSpace(in.SubcategoryID); trimmed != "" {
		parsed, parseErr := uuid.Parse(trimmed)
		if parseErr != nil {
			return usecases.CreateTransactionResult{}, fmt.Errorf("agent: transaction creator: subcategory id: %w", parseErr)
		}
		subID = &parsed
	}
	if _, ok := auth.FromContext(ctx); !ok {
		ctx = auth.WithPrincipal(ctx, auth.Principal{UserID: userID, Source: auth.SourceWhatsApp})
	}
	out, err := a.uc.Execute(ctx, transactionsinput.RawCreateTransaction{
		Direction:     in.Direction,
		PaymentMethod: in.PaymentMethod,
		AmountCents:   in.AmountCents,
		Description:   in.Description,
		CategoryID:    rootID,
		SubcategoryID: subID,
		OccurredAt:    in.OccurredAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return usecases.CreateTransactionResult{}, err
	}
	return usecases.CreateTransactionResult{AmountCents: out.AmountCents, Direction: out.Direction}, nil
}

type TransactionLoggerAdapter struct {
	uc *usecases.LogTransactionFromAgent
}

func NewTransactionLoggerAdapter(uc *usecases.LogTransactionFromAgent) *TransactionLoggerAdapter {
	return &TransactionLoggerAdapter{uc: uc}
}

func (a *TransactionLoggerAdapter) Execute(ctx context.Context, in appservices.ExpenseLoggerInput) (appservices.ExpenseLoggerResult, error) {
	result, err := a.uc.Execute(ctx, usecases.LogTransactionFromAgentInput{
		UserID: in.UserID,
		Intent: in.Intent,
	})
	if err != nil {
		return appservices.ExpenseLoggerResult{}, err
	}
	return appservices.ExpenseLoggerResult{
		Persisted:    result.Persisted,
		AmountCents:  result.AmountCents,
		CategoryPath: result.CategoryPath,
		OccurredAt:   result.OccurredAt,
	}, nil
}
