package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	categoriesvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
)

type CardPurchaseCreator interface {
	Execute(ctx context.Context, in CreateCardPurchaseCommand) (CreateCardPurchaseResult, error)
}

type CreateCardPurchaseCommand struct {
	UserID         string
	CardHint       string
	Description    string
	RootCategoryID string
	SubcategoryID  string
	AmountCents    int64
	Installments   int
}

type CreateCardPurchaseResult struct {
	CardFound bool
	CardName  string
}

type RecordCardPurchaseFromAgent struct {
	resolver       CategoryResolver
	creator        CardPurchaseCreator
	o11y           observability.Observability
	persisted      observability.Counter
	resolveBad     observability.Counter
	scoreHistogram observability.Histogram
}

func NewRecordCardPurchaseFromAgent(
	resolver CategoryResolver,
	creator CardPurchaseCreator,
	o11y observability.Observability,
) *RecordCardPurchaseFromAgent {
	persisted := o11y.Metrics().Counter(
		"agent_log_card_purchase_persisted_total",
		"Total de compras parceladas persistidas a partir de intent do agente",
		"1",
	)
	resolveBad := o11y.Metrics().Counter(
		"agent_log_card_purchase_failed_total",
		"Total de tentativas de compra parcelada que falharam ao resolver categoria, cartão ou persistir",
		"1",
	)
	return &RecordCardPurchaseFromAgent{
		resolver:       resolver,
		creator:        creator,
		o11y:           o11y,
		persisted:      persisted,
		resolveBad:     resolveBad,
		scoreHistogram: newMatchScoreHistogram(o11y),
	}
}

type RecordCardPurchaseFromAgentInput struct {
	UserID        string
	Intent        intent.Intent
	ForceCategory *string
	AmountCents   int64
	Merchant      string
	PaymentMethod string
	CardHint      string
	Installments  int
}

type RecordCardPurchaseFromAgentResult struct {
	Persisted    bool
	CardFound    bool
	CardName     string
	AmountCents  int64
	Installments int
	CategoryPath string
}

func (uc *RecordCardPurchaseFromAgent) Execute(ctx context.Context, in RecordCardPurchaseFromAgentInput) (RecordCardPurchaseFromAgentResult, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.log_card_purchase_from_agent")
	defer span.End()

	if strings.TrimSpace(in.UserID) == "" {
		return RecordCardPurchaseFromAgentResult{}, errors.New("agent: log card purchase: user id vazio")
	}

	if in.ForceCategory != nil && strings.TrimSpace(*in.ForceCategory) != "" {
		return uc.executeForced(ctx, in)
	}

	if in.Intent.Kind() != intent.KindRecordCardPurchase {
		return RecordCardPurchaseFromAgentResult{}, ErrLogTransactionInvalidIntent
	}
	if in.Intent.AmountCents() <= 0 {
		return RecordCardPurchaseFromAgentResult{}, errors.New("agent: log card purchase: amount invalido")
	}

	hint := resolveHint(in.Intent.CategoryHint(), in.Intent.Merchant())
	if hint == "" {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "no_hint"))
		return RecordCardPurchaseFromAgentResult{}, ErrLogTransactionNoCategoryHint
	}

	candidate, path, err := resolveCategoryCandidate(ctx, uc.resolver, uc.resolveBad, uc.scoreHistogram, hint, categoriesvo.KindExpense)
	if err != nil {
		return RecordCardPurchaseFromAgentResult{}, err
	}

	description := strings.TrimSpace(in.Intent.Merchant())
	if description == "" {
		description = path
	}

	sub := ""
	if subID := candidateSubcategoryUUID(candidate); subID != nil {
		sub = subID.String()
	}

	result, err := uc.creator.Execute(ctx, CreateCardPurchaseCommand{
		UserID:         in.UserID,
		CardHint:       in.Intent.CardHint(),
		Description:    description,
		RootCategoryID: candidate.RootCategoryID.String(),
		SubcategoryID:  sub,
		AmountCents:    in.Intent.AmountCents(),
		Installments:   in.Intent.Installments(),
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "create_failed"))
		return RecordCardPurchaseFromAgentResult{}, err
	}
	if !result.CardFound {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "card_not_found"))
		return RecordCardPurchaseFromAgentResult{
			Persisted:    false,
			CardFound:    false,
			AmountCents:  in.Intent.AmountCents(),
			Installments: in.Intent.Installments(),
			CategoryPath: path,
		}, nil
	}

	uc.persisted.Add(ctx, 1)
	return RecordCardPurchaseFromAgentResult{
		Persisted:    true,
		CardFound:    true,
		CardName:     result.CardName,
		AmountCents:  in.Intent.AmountCents(),
		Installments: in.Intent.Installments(),
		CategoryPath: path,
	}, nil
}

func (uc *RecordCardPurchaseFromAgent) executeForced(ctx context.Context, in RecordCardPurchaseFromAgentInput) (RecordCardPurchaseFromAgentResult, error) {
	forcedPath := strings.TrimSpace(*in.ForceCategory)
	description := strings.TrimSpace(in.Merchant)
	if description == "" {
		description = forcedPath
	}
	cardHint := strings.TrimSpace(in.CardHint)
	amountCents := in.AmountCents
	installments := in.Installments

	result, err := uc.creator.Execute(ctx, CreateCardPurchaseCommand{
		UserID:         in.UserID,
		CardHint:       cardHint,
		Description:    description,
		RootCategoryID: forcedPath,
		AmountCents:    amountCents,
		Installments:   installments,
	})
	if err != nil {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "force_category_create_failed"))
		return RecordCardPurchaseFromAgentResult{}, fmt.Errorf("agent: log card purchase: force category: %w", err)
	}
	if !result.CardFound {
		uc.resolveBad.Add(ctx, 1, observability.String("reason", "card_not_found"))
		return RecordCardPurchaseFromAgentResult{
			Persisted:    false,
			CardFound:    false,
			AmountCents:  amountCents,
			Installments: installments,
			CategoryPath: forcedPath,
		}, nil
	}
	uc.persisted.Add(ctx, 1)
	return RecordCardPurchaseFromAgentResult{
		Persisted:    true,
		CardFound:    true,
		CardName:     result.CardName,
		AmountCents:  amountCents,
		Installments: installments,
		CategoryPath: forcedPath,
	}, nil
}
