package workflows

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	cardManageConfirmTTL       = 15 * time.Minute
	cardManageMaxReprompts     = 1
	cardManageSlotTTL          = 30 * time.Minute
	cardManageMaxSlotReprompts = 2
	cardManageMaxClosingEcho   = 1
	cardManageMaxNicknameRunes = 32
)

type CardManageSlotAction int

const (
	CardManageSlotFill CardManageSlotAction = iota + 1
	CardManageSlotReprompt
	CardManageSlotDisambiguate
	CardManageSlotCancel
)

func (a CardManageSlotAction) String() string {
	switch a {
	case CardManageSlotFill:
		return "fill"
	case CardManageSlotReprompt:
		return "reprompt"
	case CardManageSlotDisambiguate:
		return "disambiguate"
	case CardManageSlotCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

func (a CardManageSlotAction) IsValid() bool {
	return a >= CardManageSlotFill && a <= CardManageSlotCancel
}

type CardManageSlotDecision struct {
	Action CardManageSlotAction
	Text   string
	Day    int
}

const cardManageSlotCutset = " .,;:!?\"'"

var cardManageSlotLeadIns = [][]string{
	{"o", "apelido", "dele", "e"},
	{"o", "apelido", "e"},
	{"apelido", "e"},
	{"vou", "chamar", "de"},
	{"quero", "chamar", "de"},
	{"pode", "chamar", "de"},
	{"o", "nome", "e"},
	{"nome", "e"},
	{"o", "banco", "e"},
	{"banco", "e"},
	{"o", "banco"},
	{"apelido"},
	{"banco"},
	{"chama"},
	{"e", "o"},
	{"e", "a"},
	{"e"},
}

var cardManageSlotTrailers = [][]string{
	{"por", "favor"},
	{"mesmo"},
	{"pf"},
}

func ShouldReleaseCardManageSlot(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasSuffix(trimmed, "?") {
		return true
	}
	return reLaunchVerbs.MatchString(trimmed) || isNewCompleteOperation(trimmed)
}

func isExplicitSlotCancel(text string) bool {
	trimmed := strings.TrimSpace(text)
	if isCancelMessage(trimmed) {
		return true
	}
	tokens := strings.Fields(normalizeText(strings.Trim(trimmed, cardManageSlotCutset)))
	if len(tokens) == 0 || len(tokens) > 2 {
		return false
	}
	for _, token := range tokens {
		if !confirmNoTokens[token] {
			return false
		}
	}
	return true
}

func matchesTokenPrefix(norm, phrase []string) bool {
	if len(phrase) > len(norm) {
		return false
	}
	for i, token := range phrase {
		if norm[i] != token {
			return false
		}
	}
	return true
}

func matchesTokenSuffix(norm, phrase []string) bool {
	if len(phrase) >= len(norm) {
		return false
	}
	offset := len(norm) - len(phrase)
	for i, token := range phrase {
		if norm[offset+i] != token {
			return false
		}
	}
	return true
}

func normalizeCardSlotAnswer(text string) string {
	fields := strings.Fields(strings.Trim(strings.TrimSpace(text), cardManageSlotCutset))
	tokens := make([]string, 0, len(fields))
	norm := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned := strings.Trim(field, cardManageSlotCutset)
		if cleaned == "" {
			continue
		}
		normalized := normalizeText(cleaned)
		if normalized == "" {
			continue
		}
		tokens = append(tokens, cleaned)
		norm = append(norm, normalized)
	}
	if len(tokens) == 0 {
		return ""
	}

	for changed := true; changed; {
		changed = false
		for _, phrase := range cardManageSlotLeadIns {
			if matchesTokenPrefix(norm, phrase) {
				tokens = tokens[len(phrase):]
				norm = norm[len(phrase):]
				changed = true
				break
			}
		}
	}

	for changed := true; changed; {
		changed = false
		for _, phrase := range cardManageSlotTrailers {
			if matchesTokenSuffix(norm, phrase) {
				tokens = tokens[:len(tokens)-len(phrase)]
				norm = norm[:len(norm)-len(phrase)]
				changed = true
				break
			}
		}
	}

	return strings.Join(tokens, " ")
}

func decideCardManageTextSlot(text string) CardManageSlotDecision {
	if isExplicitSlotCancel(text) {
		return CardManageSlotDecision{Action: CardManageSlotCancel}
	}
	cleaned := normalizeCardSlotAnswer(text)
	if cleaned == "" || len([]rune(cleaned)) > cardManageMaxNicknameRunes {
		return CardManageSlotDecision{Action: CardManageSlotReprompt}
	}
	return CardManageSlotDecision{Action: CardManageSlotFill, Text: cleaned}
}

func DecideCardManageNicknameAnswer(text string) CardManageSlotDecision {
	return decideCardManageTextSlot(text)
}

func DecideCardManageBankAnswer(text string) CardManageSlotDecision {
	return decideCardManageTextSlot(text)
}

func DecideCardManageDueDayAnswer(text string) CardManageSlotDecision {
	if isExplicitSlotCancel(text) {
		return CardManageSlotDecision{Action: CardManageSlotCancel}
	}
	day := ParseRecurrenceDayOfMonth(text)
	if day <= 0 || day > 31 {
		return CardManageSlotDecision{Action: CardManageSlotReprompt}
	}
	return CardManageSlotDecision{Action: CardManageSlotFill, Day: day}
}

func DecideCardManageClosingDayAnswer(state CardManageState, text string) CardManageSlotDecision {
	if isExplicitSlotCancel(text) {
		return CardManageSlotDecision{Action: CardManageSlotCancel}
	}
	day := ParseRecurrenceDayOfMonth(text)
	if day <= 0 || day > 31 {
		return CardManageSlotDecision{Action: CardManageSlotReprompt}
	}
	if day == state.DueDay && state.ClosingDayEcho < cardManageMaxClosingEcho {
		return CardManageSlotDecision{Action: CardManageSlotDisambiguate, Day: day}
	}
	return CardManageSlotDecision{Action: CardManageSlotFill, Day: day}
}

func DecideCardManageNextSlot(state CardManageState) CardManageAwaiting {
	switch {
	case strings.TrimSpace(state.Nickname) == "":
		return CardManageAwaitingNickname
	case strings.TrimSpace(state.Bank) == "":
		return CardManageAwaitingBank
	case state.DueDay < 1 || state.DueDay > 31:
		return CardManageAwaitingDueDay
	case !state.BankRecognized && !state.ClosingDayProvided:
		return CardManageAwaitingClosingDay
	default:
		return CardManageAwaitingConfirm
	}
}

func isCardManageSlotExpired(state CardManageState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > cardManageSlotTTL
}

var ErrCardManageAcceptedWithoutResource = ErrWriteAcceptedWithoutResource

func DecideCardManagePostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (workflow.StepStatus, error) {
	if outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil {
		return workflow.StepStatusFailed, ErrCardManageAcceptedWithoutResource
	}
	return workflow.StepStatusCompleted, nil
}

type CardManageAction int

const (
	CardManageActionAccept CardManageAction = iota + 1
	CardManageActionCancel
	CardManageActionReprompt
	CardManageActionExpire
	CardManageActionReplay
)

func isCardManageExpired(state CardManageState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > cardManageConfirmTTL
}

func DecideCardManageConfirmation(state CardManageState, msg PendingMessage, now time.Time) CardManageAction {
	if isCardManageExpired(state, now) {
		return CardManageActionExpire
	}

	if msg.MessageID != "" && msg.MessageID == state.ProcessedMessageID {
		return CardManageActionReplay
	}

	switch DecideConfirmAnswer(msg.Text) {
	case ConfirmAnswerYes:
		return CardManageActionAccept
	case ConfirmAnswerNo:
		return CardManageActionCancel
	}

	if state.ConfirmReprompt >= cardManageMaxReprompts {
		return CardManageActionCancel
	}

	return CardManageActionReprompt
}
