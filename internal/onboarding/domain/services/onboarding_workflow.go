package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

var (
	ErrOnboardingInvalidInput = errors.New("onboarding: invalid input")
	ErrOnboardingUnsupported  = errors.New("onboarding: unsupported transition")
)

type DecisionKind uint8

const (
	DecisionKindNoOp DecisionKind = iota + 1
	DecisionKindAdvanceState
	DecisionKindReplyOnly
	DecisionKindComplete
)

type InboundMessage struct {
	Text string
}

type DecisionOutcome struct {
	Kind         DecisionKind
	NewState     valueobjects.OnboardingState
	NewPayload   entities.OnboardingSessionPayload
	OutboundText string
	DomainEvents []entities.OnboardingDomainEvent
}

type OnboardingWorkflow struct{}

func NewOnboardingWorkflow() OnboardingWorkflow {
	return OnboardingWorkflow{}
}

func (w OnboardingWorkflow) DecideNext(
	session entities.OnboardingSession,
	inbound InboundMessage,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	text := strings.TrimSpace(inbound.Text)
	state := session.State()
	payload := session.Payload()
	channel := session.Channel().String()

	switch state {
	case valueobjects.OnboardingStateAwaitingToken:
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Para ativar, envie: ATIVAR <seu codigo>.",
		}, nil

	case valueobjects.OnboardingStateAwaitingIncome:
		return decideIncome(session, payload, channel, text, eventIDs, now)

	case valueobjects.OnboardingStateAwaitingCardDecision:
		return decideCardDecision(payload, text)

	case valueobjects.OnboardingStateAwaitingCardName:
		return decideCardName(payload, text)

	case valueobjects.OnboardingStateAwaitingCardLimit:
		return decideCardLimit(payload, text)

	case valueobjects.OnboardingStateAwaitingCardClosingDay:
		return decideCardClosingDay(payload, text)

	case valueobjects.OnboardingStateAwaitingCardDueDay:
		return decideCardDueDay(session, payload, channel, text, eventIDs, now)

	case valueobjects.OnboardingStateAwaitingMoreCards:
		return decideMoreCards(payload, text)

	case valueobjects.OnboardingStateAwaitingSplitConfirm:
		return decideSplitConfirm(session, payload, channel, text, eventIDs, now)

	case valueobjects.OnboardingStateActive:
		return DecisionOutcome{Kind: DecisionKindNoOp}, nil

	default:
		return DecisionOutcome{}, fmt.Errorf("%w: state=%s", ErrOnboardingUnsupported, state.String())
	}
}

func decideIncome(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	text string,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	cents, ok := parseMonetary(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Nao entendi o valor. Envie por exemplo: R$ 3500 ou 3500.",
		}, nil
	}
	income, err := valueobjects.NewMonthlyIncome(cents)
	if err != nil {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Esse valor parece fora do esperado. Envie um valor mensal realista (a partir de R$ 500).",
		}, nil
	}
	if len(eventIDs) < 1 {
		return DecisionOutcome{}, fmt.Errorf("onboarding: decide income: missing event id")
	}
	payload.IncomeCents = income.Cents()
	evt := entities.IncomeRegistered{
		EventID:     eventIDs[0],
		UserID:      session.UserID(),
		Channel:     channel,
		IncomeCents: income.Cents(),
		OccurredAt:  now,
	}
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardDecision,
		NewPayload:   payload,
		OutboundText: "Otimo! Quer cadastrar um cartao de credito agora? Responda sim ou nao.",
		DomainEvents: []entities.OnboardingDomainEvent{evt},
	}, nil
}

func decideCardDecision(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	yes, ok := parseYesNo(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Responda sim ou nao para cadastrar um cartao.",
		}, nil
	}
	if !yes {
		next, out, err := transitionToSplit(payload)
		if err != nil {
			return DecisionOutcome{}, err
		}
		return DecisionOutcome{
			Kind:         DecisionKindAdvanceState,
			NewState:     valueobjects.OnboardingStateAwaitingSplitConfirm,
			NewPayload:   next,
			OutboundText: out,
		}, nil
	}
	payload.HasPending = true
	payload.PendingCard = entities.OnboardingCardDraft{}
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardName,
		NewPayload:   payload,
		OutboundText: "Qual o nome do cartao?",
	}, nil
}

func decideCardName(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	if text == "" {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie o nome do cartao (ex: Nubank).",
		}, nil
	}
	pending := payload.PendingCard
	pending.Name = text
	payload.PendingCard = pending
	payload.HasPending = true
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardLimit,
		NewPayload:   payload,
		OutboundText: "Qual o limite total do cartao?",
	}, nil
}

func decideCardLimit(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	cents, ok := parseMonetary(text)
	if !ok || cents <= 0 {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Nao entendi o limite. Envie por exemplo: R$ 5000.",
		}, nil
	}
	pending := payload.PendingCard
	pending.LimitCents = cents
	payload.PendingCard = pending
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardClosingDay,
		NewPayload:   payload,
		OutboundText: "Qual o dia de fechamento da fatura (1 a 31)?",
	}, nil
}

func decideCardClosingDay(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	day, ok := parseDay(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie um dia entre 1 e 31.",
		}, nil
	}
	closing, err := valueobjects.NewCardClosingDay(day)
	if err != nil {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie um dia entre 1 e 31.",
		}, nil
	}
	pending := payload.PendingCard
	pending.ClosingDay = closing.Value()
	payload.PendingCard = pending
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardDueDay,
		NewPayload:   payload,
		OutboundText: "E o dia de vencimento (1 a 31)?",
	}, nil
}

func decideCardDueDay(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	text string,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	day, ok := parseDay(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie um dia entre 1 e 31.",
		}, nil
	}
	due, err := valueobjects.NewCardDueDay(day)
	if err != nil {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie um dia entre 1 e 31.",
		}, nil
	}
	if len(eventIDs) < 1 {
		return DecisionOutcome{}, fmt.Errorf("onboarding: decide card due day: missing event id")
	}
	pending := payload.PendingCard
	pending.DueDay = due.Value()
	card := pending
	payload.Cards = append(payload.Cards, card)
	payload.PendingCard = entities.OnboardingCardDraft{}
	payload.HasPending = false
	evt := entities.CardRegistered{
		EventID:    eventIDs[0],
		UserID:     session.UserID(),
		Channel:    channel,
		Name:       card.Name,
		LimitCents: card.LimitCents,
		ClosingDay: card.ClosingDay,
		DueDay:     card.DueDay,
		OccurredAt: now,
	}
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingMoreCards,
		NewPayload:   payload,
		OutboundText: "Cartao salvo. Quer cadastrar outro? sim ou nao.",
		DomainEvents: []entities.OnboardingDomainEvent{evt},
	}, nil
}

func decideMoreCards(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	yes, ok := parseYesNo(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Responda sim ou nao para cadastrar outro cartao.",
		}, nil
	}
	if yes {
		payload.HasPending = true
		payload.PendingCard = entities.OnboardingCardDraft{}
		return DecisionOutcome{
			Kind:         DecisionKindAdvanceState,
			NewState:     valueobjects.OnboardingStateAwaitingCardName,
			NewPayload:   payload,
			OutboundText: "Qual o nome do proximo cartao?",
		}, nil
	}
	next, out, err := transitionToSplit(payload)
	if err != nil {
		return DecisionOutcome{}, err
	}
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingSplitConfirm,
		NewPayload:   next,
		OutboundText: out,
	}, nil
}

func decideSplitConfirm(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	text string,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	if isAdjustIntent(text) {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Ajuste manual ainda nao esta disponivel por aqui. Confirme com sim para usar a sugestao.",
		}, nil
	}
	yes, ok := parseYesNo(text)
	if !ok || !yes {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Confirme com sim para usar a sugestao das categorias.",
		}, nil
	}
	if len(eventIDs) < 2 {
		return DecisionOutcome{}, fmt.Errorf("onboarding: decide split confirm: missing event ids")
	}
	splitEvt := entities.SplitsCalculated{
		EventID:     eventIDs[0],
		UserID:      session.UserID(),
		Channel:     channel,
		IncomeCents: payload.IncomeCents,
		Allocations: toSplitsCalculatedEntries(payload.Split),
		OccurredAt:  now,
	}
	completeEvt := entities.OnboardingCompleted{
		EventID:    eventIDs[1],
		UserID:     session.UserID(),
		Channel:    channel,
		OccurredAt: now,
	}
	return DecisionOutcome{
		Kind:         DecisionKindComplete,
		NewState:     valueobjects.OnboardingStateActive,
		NewPayload:   payload,
		OutboundText: "Pronto! Onboarding concluido. Bora comecar.",
		DomainEvents: []entities.OnboardingDomainEvent{splitEvt, completeEvt},
	}, nil
}

func transitionToSplit(payload entities.OnboardingSessionPayload) (entities.OnboardingSessionPayload, string, error) {
	split, err := SuggestDefaultSplit()
	if err != nil {
		return payload, "", fmt.Errorf("onboarding: build default split: %w", err)
	}
	payload.Split = make([]entities.OnboardingCardSplitEntry, 0, 5)
	for _, a := range split.Allocations() {
		payload.Split = append(payload.Split, entities.OnboardingCardSplitEntry{
			Kind:    a.Kind.String(),
			Percent: a.Percent,
		})
	}
	out := "Sugestao das 5 categorias: Custo Fixo 40%, Conhecimento 10%, Prazeres 15%, Metas 20%, Liberdade Financeira 15%. Esta otimo? Confirme com sim."
	return payload, out, nil
}

func toSplitsCalculatedEntries(entries []entities.OnboardingCardSplitEntry) []entities.SplitsCalculatedEntry {
	out := make([]entities.SplitsCalculatedEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, entities.SplitsCalculatedEntry(e))
	}
	return out
}

func SuggestDefaultSplit() (valueobjects.CategorySplit, error) {
	return valueobjects.NewCategorySplit([]valueobjects.CategoryAllocation{
		{Kind: valueobjects.CategoryKindFixedCost, Percent: 40},
		{Kind: valueobjects.CategoryKindKnowledge, Percent: 10},
		{Kind: valueobjects.CategoryKindPleasures, Percent: 15},
		{Kind: valueobjects.CategoryKindGoals, Percent: 20},
		{Kind: valueobjects.CategoryKindFinancialFreedom, Percent: 15},
	})
}

func parseMonetary(text string) (int64, bool) {
	s, ok := normalizeMonetaryInput(text)
	if !ok {
		return 0, false
	}
	s = normalizeMonetarySeparators(s)
	if !isValidMonetaryNumber(s) {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return int64(f*100 + 0.5), true
}

func normalizeMonetaryInput(text string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return "", false
	}
	s = strings.TrimPrefix(s, "r$")
	s = strings.TrimPrefix(s, "rs")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return "", false
	}
	return s, true
}

func normalizeMonetarySeparators(s string) string {
	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")
	switch {
	case hasComma && hasDot:
		return normalizeMixedMonetarySeparators(s)
	case hasComma:
		return strings.Replace(s, ",", ".", 1)
	case hasDot && hasThousandsDots(s):
		return strings.ReplaceAll(s, ".", "")
	default:
		return s
	}
}

func normalizeMixedMonetarySeparators(s string) string {
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	if lastComma > lastDot {
		s = strings.ReplaceAll(s, ".", "")
		return strings.Replace(s, ",", ".", 1)
	}
	return strings.ReplaceAll(s, ",", "")
}

func hasThousandsDots(s string) bool {
	if strings.Count(s, ".") > 1 {
		return true
	}
	parts := strings.Split(s, ".")
	return len(parts) == 2 && len(parts[1]) == 3
}

func isValidMonetaryNumber(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' {
			return false
		}
	}
	return true
}

func parseDay(text string) (int, bool) {
	s := strings.ToLower(strings.TrimSpace(text))
	if s == "" {
		return 0, false
	}
	s = strings.TrimPrefix(s, "dia")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseYesNo(text string) (bool, bool) {
	s := strings.ToLower(strings.TrimSpace(text))
	switch s {
	case "sim", "s", "claro", "quero", "ok", "okay", "sim!", "vamos", "yes", "y":
		return true, true
	case "nao", "não", "n", "no", "agora nao", "agora não":
		return false, true
	default:
		return false, false
	}
}

func isAdjustIntent(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(s, "ajustar") || strings.Contains(s, "mudar") || strings.Contains(s, "trocar")
}
