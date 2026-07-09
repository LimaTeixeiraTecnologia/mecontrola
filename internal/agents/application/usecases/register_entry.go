package usecases

import (
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const (
	registerDirectionIncome     = "income"
	registerIncomePaymentMethod = "pix"
	mecontrolaAgentID           = "mecontrola-agent"
)

type RegisterResult struct {
	ResourceID uuid.UUID
	Kind       string
	Outcome    agent.ToolOutcome
	Message    string
}

type RegisterExpenseCommand struct {
	UserID          uuid.UUID
	ThreadID        string
	WAMID           string
	ItemSeq         int
	AmountCents     int64
	Description     string
	PaymentMethod   string
	CardID          *uuid.UUID
	Installments    int
	OccurredAt      string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
}

type RegisterIncomeCommand struct {
	UserID          uuid.UUID
	ThreadID        string
	WAMID           string
	ItemSeq         int
	AmountCents     int64
	Description     string
	OccurredAt      string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
}

func resolveEntryDate(raw string) string {
	if raw != "" {
		return raw
	}
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	return time.Now().In(loc).Format("2006-01-02")
}
