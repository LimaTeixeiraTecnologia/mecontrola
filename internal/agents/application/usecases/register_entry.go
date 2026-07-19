package usecases

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
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
	CategoryText    string
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
	CategoryText    string
}

type CreateRecurrenceCommand struct {
	UserID          uuid.UUID
	ThreadID        string
	WAMID           string
	ItemSeq         int
	Direction       string
	PaymentMethod   string
	CardID          *uuid.UUID
	AmountCents     int64
	Description     string
	CategoryID      uuid.UUID
	SubcategoryID   uuid.UUID
	CategoryVersion int64
	CategoryText    string
	Frequency       string
	DayOfMonth      int
	StartedAt       string
}

type EditEntryCommand struct {
	UserID              uuid.UUID
	ThreadID            string
	WAMID               string
	ItemSeq             int
	TargetTransactionID uuid.UUID
	AmountCents         int64
	Description         string
	OccurredAt          string
	PaymentMethod       string
	CategoryID          uuid.UUID
	SubcategoryID       uuid.UUID
	CategoryVersion     int64
	SearchAmountCents   int64
	SearchTerm          string
}

func currentCategoryIDs(current interfaces.Entry) (uuid.UUID, uuid.UUID, error) {
	rootID, rootErr := uuid.Parse(current.CategoryID)
	if rootErr != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("categoryId inválido: %w", rootErr)
	}
	var subID uuid.UUID
	if current.SubcategoryID != nil {
		parsed, subErr := uuid.Parse(*current.SubcategoryID)
		if subErr != nil {
			return uuid.Nil, uuid.Nil, fmt.Errorf("subcategoryId inválido: %w", subErr)
		}
		subID = parsed
	}
	return rootID, subID, nil
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
