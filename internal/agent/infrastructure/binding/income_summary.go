package binding

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
)

type IncomeSummaryReaderAdapter struct {
	uc    listTransactionsUseCase
	limit int
}

func NewIncomeSummaryReaderAdapter(uc listTransactionsUseCase) *IncomeSummaryReaderAdapter {
	return &IncomeSummaryReaderAdapter{uc: uc, limit: 200}
}

func (a *IncomeSummaryReaderAdapter) Execute(ctx context.Context, in tools.IncomeSummaryInput) (tools.IncomeSummaryResult, error) {
	userID, err := uuid.Parse(strings.TrimSpace(in.UserID))
	if err != nil {
		return tools.IncomeSummaryResult{}, fmt.Errorf("agent: income summary reader: user id: %w", err)
	}
	ctx = withWhatsAppPrincipal(ctx, userID)

	page, err := a.uc.Execute(ctx, in.RefMonth, "", a.limit)
	if err != nil {
		return tools.IncomeSummaryResult{}, fmt.Errorf("agent: income summary reader: %w", err)
	}

	var total int64
	sources := make([]tools.IncomeSourceView, 0)
	for _, t := range page.Transactions {
		if t.Direction != "income" {
			continue
		}
		total += t.AmountCents
		sources = append(sources, tools.IncomeSourceView{
			Description: t.Description,
			AmountCents: t.AmountCents,
		})
	}
	return tools.IncomeSummaryResult{
		RefMonth:   in.RefMonth,
		TotalCents: total,
		Sources:    sources,
	}, nil
}
