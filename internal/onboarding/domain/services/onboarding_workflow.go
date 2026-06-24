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
		return decideCardName(session, payload, channel, text, eventIDs, now)

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

func decideCardName(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	text string,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	if strings.TrimSpace(text) == "" {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie o nome do cartao (ex: Nubank).",
		}, nil
	}
	if card, ok := parseCardShortcut(text); ok {
		return finalizeCard(session, payload, channel, card, eventIDs, now)
	}
	pending := payload.PendingCard
	pending.Name = strings.TrimSpace(text)
	payload.PendingCard = pending
	payload.HasPending = true
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardLimit,
		NewPayload:   payload,
		OutboundText: "Qual o limite total do cartao? (ex: R$ 5000). Dica: voce pode mandar tudo de uma vez, ex: Nubank 5000 fecha 27 vence 5.",
	}, nil
}

func finalizeCard(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	card entities.OnboardingCardDraft,
	eventIDs []uuid.UUID,
	now time.Time,
) (DecisionOutcome, error) {
	if len(eventIDs) < 1 {
		return DecisionOutcome{}, fmt.Errorf("onboarding: finalize card: missing event id")
	}
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
		OccurredAt: now,
	}
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingMoreCards,
		NewPayload:   payload,
		OutboundText: cardSavedSummary(card),
		DomainEvents: []entities.OnboardingDomainEvent{evt},
	}, nil
}

func cardSavedSummary(card entities.OnboardingCardDraft) string {
	return fmt.Sprintf(
		"Cartao salvo: %s, limite R$ %s, fecha dia %d e vence dia %d. Quer cadastrar outro? sim ou nao.",
		card.Name,
		formatCents(card.LimitCents),
		card.ClosingDay,
		card.DueDay,
	)
}

func formatCents(cents int64) string {
	return strconv.FormatInt(cents/100, 10)
}

func decideCardLimit(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	cents, ok := parseMonetary(text)
	if !ok || cents <= 0 {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Nao entendi o limite. Envie so o valor, por exemplo: R$ 5000.",
		}, nil
	}
	pending := payload.PendingCard
	pending.LimitCents = cents
	payload.PendingCard = pending
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardClosingDay,
		NewPayload:   payload,
		OutboundText: "Qual o dia de fechamento da fatura? Envie um numero entre 1 e 31 (ex: 27).",
	}, nil
}

func decideCardClosingDay(payload entities.OnboardingSessionPayload, text string) (DecisionOutcome, error) {
	day, ok := parseDay(text)
	if !ok {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie o dia de fechamento entre 1 e 31 (ex: 27).",
		}, nil
	}
	closing, err := valueobjects.NewCardClosingDay(day)
	if err != nil {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie o dia de fechamento entre 1 e 31 (ex: 27).",
		}, nil
	}
	pending := payload.PendingCard
	pending.ClosingDay = closing.Value()
	payload.PendingCard = pending
	return DecisionOutcome{
		Kind:         DecisionKindAdvanceState,
		NewState:     valueobjects.OnboardingStateAwaitingCardDueDay,
		NewPayload:   payload,
		OutboundText: "E o dia de vencimento? Envie um numero entre 1 e 31 (ex: 5).",
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
			OutboundText: "Envie o dia de vencimento entre 1 e 31 (ex: 5).",
		}, nil
	}
	due, err := valueobjects.NewCardDueDay(day)
	if err != nil {
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Envie o dia de vencimento entre 1 e 31 (ex: 5).",
		}, nil
	}
	pending := payload.PendingCard
	pending.DueDay = due.Value()
	return finalizeCard(session, payload, channel, pending, eventIDs, now)
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
	yes, ok := parseYesNo(text)
	switch {
	case ok && yes:
		return completeSplitConfirm(session, payload, channel, eventIDs, now, "Pronto! Onboarding concluido. Bora comecar.")
	case isAdjustIntent(text) || (ok && !yes):
		return completeSplitConfirm(session, payload, channel, eventIDs, now, "Sem problema! Vou aplicar a sugestao padrao das 5 categorias para voce ja comecar. Da pra ajustar depois aqui no app.")
	default:
		return DecisionOutcome{
			Kind:         DecisionKindReplyOnly,
			OutboundText: "Nao entendi. Responda sim para usar a sugestao das 5 categorias e concluir, ou nao para aplicar a sugestao padrao mesmo assim.",
		}, nil
	}
}

func completeSplitConfirm(
	session entities.OnboardingSession,
	payload entities.OnboardingSessionPayload,
	channel string,
	eventIDs []uuid.UUID,
	now time.Time,
	outbound string,
) (DecisionOutcome, error) {
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
		OutboundText: outbound,
		DomainEvents: []entities.OnboardingDomainEvent{splitEvt, completeEvt},
	}, nil
}

func parseCardShortcut(text string) (entities.OnboardingCardDraft, bool) {
	tokens := strings.Fields(strings.TrimSpace(text))
	if len(tokens) < 4 {
		return entities.OnboardingCardDraft{}, false
	}
	name, rest := splitCardName(tokens)
	if name == "" || len(rest) == 0 {
		return entities.OnboardingCardDraft{}, false
	}
	limit, closing, due, ok := extractCardFields(rest)
	if !ok {
		return entities.OnboardingCardDraft{}, false
	}
	if _, err := valueobjects.NewCardClosingDay(closing); err != nil {
		return entities.OnboardingCardDraft{}, false
	}
	if _, err := valueobjects.NewCardDueDay(due); err != nil {
		return entities.OnboardingCardDraft{}, false
	}
	return entities.OnboardingCardDraft{
		Name:       name,
		LimitCents: limit,
		ClosingDay: closing,
		DueDay:     due,
	}, true
}

func splitCardName(tokens []string) (string, []string) {
	nameParts := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		if isCardFieldToken(tok) {
			return strings.Join(nameParts, " "), tokens[i:]
		}
		nameParts = append(nameParts, tok)
	}
	return "", nil
}

func isCardFieldToken(tok string) bool {
	if _, ok := parseMonetary(tok); ok {
		return true
	}
	return isCardFieldKeyword(tok)
}

func isCardFieldKeyword(tok string) bool {
	switch strings.ToLower(tok) {
	case "limite", "lim", "fecha", "fechamento", "fechar", "vence", "vencimento", "vencer", "r$", "rs":
		return true
	}
	return false
}

func extractCardFields(tokens []string) (int64, int, int, bool) {
	var limit int64
	var closing, due int
	haveLimit, haveClosing, haveDue := false, false, false
	positional := make([]string, 0, len(tokens))
	field := ""
	for _, tok := range tokens {
		low := strings.ToLower(tok)
		switch {
		case isLimitKeyword(low):
			field = "limit"
		case isClosingKeyword(low):
			field = "closing"
		case isDueKeyword(low):
			field = "due"
		case low == "r$" || low == "rs":
			field = "limit"
		default:
			if !applyCardFieldValue(field, tok, &limit, &closing, &due, &haveLimit, &haveClosing, &haveDue) {
				positional = append(positional, tok)
			}
			field = ""
		}
	}
	return resolveCardFields(positional, limit, closing, due, haveLimit, haveClosing, haveDue)
}

func applyCardFieldValue(
	field, tok string,
	limit *int64,
	closing, due *int,
	haveLimit, haveClosing, haveDue *bool,
) bool {
	switch field {
	case "limit":
		if cents, ok := parseMonetary(tok); ok && cents > 0 {
			*limit, *haveLimit = cents, true
			return true
		}
	case "closing":
		if d, ok := parseDay(tok); ok {
			*closing, *haveClosing = d, true
			return true
		}
	case "due":
		if d, ok := parseDay(tok); ok {
			*due, *haveDue = d, true
			return true
		}
	}
	return false
}

func resolveCardFields(
	positional []string,
	limit int64,
	closing, due int,
	haveLimit, haveClosing, haveDue bool,
) (int64, int, int, bool) {
	for _, tok := range positional {
		switch {
		case !haveLimit:
			cents, ok := parseMonetary(tok)
			if !ok || cents <= 0 {
				return 0, 0, 0, false
			}
			limit, haveLimit = cents, true
		case !haveClosing:
			d, ok := parseDay(tok)
			if !ok {
				return 0, 0, 0, false
			}
			closing, haveClosing = d, true
		case !haveDue:
			d, ok := parseDay(tok)
			if !ok {
				return 0, 0, 0, false
			}
			due, haveDue = d, true
		default:
			return 0, 0, 0, false
		}
	}
	if !haveLimit || !haveClosing || !haveDue {
		return 0, 0, 0, false
	}
	return limit, closing, due, true
}

func isLimitKeyword(s string) bool {
	return s == "limite" || s == "lim"
}

func isClosingKeyword(s string) bool {
	return s == "fecha" || s == "fechamento" || s == "fechar"
}

func isDueKeyword(s string) bool {
	return s == "vence" || s == "vencimento" || s == "vencer"
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
	s := normalizeYesNo(text)
	if s == "" {
		return false, false
	}
	if isNegation(s) {
		return false, true
	}
	if isAffirmation(s) {
		return true, true
	}
	return false, false
}

func normalizeYesNo(text string) string {
	s := strings.ToLower(strings.TrimSpace(text))
	s = strings.TrimRight(s, "!.? ")
	return strings.TrimSpace(s)
}

func isNegation(s string) bool {
	switch s {
	case "nao", "não", "n", "no", "agora nao", "agora não", "negativo", "nope":
		return true
	}
	return false
}

func isAffirmation(s string) bool {
	switch s {
	case "sim", "s", "claro", "quero", "ok", "okay", "vamos", "yes", "y",
		"pode", "manda", "isso", "confirmo", "confirmar", "confirma",
		"bora", "claro que sim", "sim cadastrar", "sim, cadastrar",
		"ta bom", "tá bom", "ta", "tá", "beleza", "blz", "pode sim", "pode ser":
		return true
	}
	switch {
	case strings.HasPrefix(s, "sim "),
		strings.HasPrefix(s, "sim,"),
		strings.HasPrefix(s, "pode "),
		strings.HasPrefix(s, "claro "):
		return true
	}
	return false
}

func isAdjustIntent(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(s, "ajustar") || strings.Contains(s, "mudar") || strings.Contains(s, "trocar")
}
