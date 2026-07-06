package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func toCommandRawCreate(raw input.RawCreateTransaction) commands.RawCreateTransaction {
	occAt, _ := parseISO8601(raw.OccurredAt)
	catStr := ""
	if raw.CategoryID != uuid.Nil {
		catStr = raw.CategoryID.String()
	}
	subStr := ""
	if raw.SubcategoryID != nil {
		subStr = raw.SubcategoryID.String()
	}
	cardStr := ""
	if raw.CardID != nil {
		cardStr = raw.CardID.String()
	}
	return commands.RawCreateTransaction{
		Direction:     raw.Direction,
		PaymentMethod: raw.PaymentMethod,
		AmountCents:   raw.AmountCents,
		Description:   raw.Description,
		CategoryID:    catStr,
		SubcategoryID: subStr,
		CardID:        cardStr,
		Installments:  raw.Installments,
		OccurredAt:    occAt,
	}
}

func toCommandRawUpdate(raw input.RawUpdateTransaction, id string) commands.RawUpdateTransaction {
	occAt, _ := parseISO8601(raw.OccurredAt)
	catStr := ""
	if raw.CategoryID != uuid.Nil {
		catStr = raw.CategoryID.String()
	}
	subStr := ""
	if raw.SubcategoryID != nil {
		subStr = raw.SubcategoryID.String()
	}
	cardStr := ""
	if raw.CardID != nil {
		cardStr = raw.CardID.String()
	}
	return commands.RawUpdateTransaction{
		TransactionID: id,
		Direction:     raw.Direction,
		PaymentMethod: raw.PaymentMethod,
		AmountCents:   raw.AmountCents,
		Description:   raw.Description,
		CategoryID:    catStr,
		SubcategoryID: subStr,
		CardID:        cardStr,
		Installments:  raw.Installments,
		OccurredAt:    occAt,
		Version:       raw.Version,
	}
}

func optSubcategoryUUID(subID option.Option[valueobjects.SubcategoryID]) *uuid.UUID {
	if v, ok := subID.Get(); ok {
		u := v.UUID()
		return &u
	}
	return nil
}

func snapSubName(subID *uuid.UUID, snap interfaces.CategorySnapshot) string {
	if subID == nil {
		return ""
	}
	if snap.ParentName != "" {
		return snap.ParentName
	}
	return snap.Name
}

func guardSubcategoryRequired(dir valueobjects.Direction, subcategoryPresent bool) error {
	var errs []error
	if !subcategoryPresent {
		errs = append(errs, fmt.Errorf("subcategory_id: %w", ErrTransactionRequiresSubcategory))
	}
	return errors.Join(errs...)
}

func guardPaymentMethodMigration(current, next valueobjects.PaymentMethod) error {
	if current.IsCreditCard() != next.IsCreditCard() {
		return ErrPaymentMethodMigrationNotAllowed
	}
	return nil
}

type categoryEvidence struct {
	Source      string
	Outcome     string
	Score       float64
	Confidence  string
	Quality     string
	SignalType  string
	MatchedTerm string
	MatchReason string
	Version     int64
}

func evidenceFromRawCreate(raw input.RawCreateTransaction) categoryEvidence {
	return categoryEvidence{
		Source:      raw.CategorySource,
		Outcome:     raw.CategoryOutcome,
		Score:       raw.CategoryScore,
		Confidence:  raw.CategoryConfidence,
		Quality:     raw.CategoryQuality,
		SignalType:  raw.CategorySignalType,
		MatchedTerm: raw.CategoryMatchedTerm,
		MatchReason: raw.CategoryMatchReason,
		Version:     raw.CategoryVersion,
	}
}

func evidenceFromRawUpdate(raw input.RawUpdateTransaction) categoryEvidence {
	return categoryEvidence{
		Source:      raw.CategorySource,
		Outcome:     raw.CategoryOutcome,
		Score:       raw.CategoryScore,
		Confidence:  raw.CategoryConfidence,
		Quality:     raw.CategoryQuality,
		SignalType:  raw.CategorySignalType,
		MatchedTerm: raw.CategoryMatchedTerm,
		MatchReason: raw.CategoryMatchReason,
		Version:     raw.CategoryVersion,
	}
}

func approveUpdateCategory(ctx context.Context, gate interfaces.CategoryWriteGate, ev categoryEvidence, direction, surface string, rootID uuid.UUID, subID *uuid.UUID) (valueobjects.CategoryWriteEvidence, error) {
	if subID == nil {
		return valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryEvidenceRequired
	}
	source := valueobjects.CategoryDecisionSourceManualCanonicalID
	if ev.Source != "" {
		if parsed, parseErr := valueobjects.ParseCategoryDecisionSource(ev.Source); parseErr == nil {
			source = parsed
		}
	}
	return gate.Approve(ctx, interfaces.CategoryWriteGateInput{
		Direction:       direction,
		RootCategoryID:  rootID,
		SubcategoryID:   *subID,
		Source:          source,
		Outcome:         ev.Outcome,
		Score:           ev.Score,
		Confidence:      ev.Confidence,
		Quality:         ev.Quality,
		SignalType:      ev.SignalType,
		MatchedTerm:     ev.MatchedTerm,
		MatchReason:     ev.MatchReason,
		ExpectedVersion: ev.Version,
		Surface:         surface,
	})
}

func guardCategoryKindDirection(dir valueobjects.Direction, categoryKind string) error {
	var errs []error
	if categoryKind != "" && categoryKind != kindForDirection(dir) {
		errs = append(errs, fmt.Errorf("category_id: %w", ErrCategoryKindDirectionMismatch))
	}
	return errors.Join(errs...)
}

func kindForDirection(dir valueobjects.Direction) string {
	if dir == valueobjects.DirectionIncome {
		return "income"
	}
	return "expense"
}
