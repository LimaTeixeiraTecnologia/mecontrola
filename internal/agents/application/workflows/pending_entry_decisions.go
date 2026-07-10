package workflows

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

var ErrWriteAcceptedWithoutResource = errors.New("workflows.pending_entry: escrita aceita sem recurso durável")

func DecidePostWrite(outcome agent.ToolOutcome, resourceID uuid.UUID) (PendingStatus, workflow.StepStatus, error) {
	if outcome != agent.ToolOutcomeReplay && resourceID == uuid.Nil {
		return PendingStatusActive, workflow.StepStatusFailed, ErrWriteAcceptedWithoutResource
	}
	return PendingStatusCompleted, workflow.StepStatusCompleted, nil
}

const (
	pendingTTL   = 30 * time.Minute
	maxReprompts = 1

	PaymentMethodCreditCard = "credit_card"
)

func DecideInitialAwaiting(categoryAwaiting AwaitingSlot, paymentMethod string, hasCard bool) AwaitingSlot {
	if categoryAwaiting == AwaitingSlotCategory {
		return AwaitingSlotCategory
	}
	if paymentMethod == "" {
		return AwaitingSlotPaymentMethod
	}
	if paymentMethod == PaymentMethodCreditCard && !hasCard {
		return AwaitingSlotCard
	}
	return categoryAwaiting
}

type PendingAction int

const (
	PendingActionNone PendingAction = iota + 1
	PendingActionExpire
	PendingActionCancel
	PendingActionReplace
	PendingActionFillSlot
	PendingActionReprompt
	PendingActionConfirm
)

type PendingDecision struct {
	Action       PendingAction
	SlotFilled   AwaitingSlot
	FilledValue  string
	ResponseText string
}

type PendingMessage struct {
	Text      string
	MessageID string
}

type ConfirmAction int

const (
	ConfirmActionAccept ConfirmAction = iota + 1
	ConfirmActionCancel
	ConfirmActionReprompt
	ConfirmActionExpire
	ConfirmActionReplay
)

type ConfirmDecision struct {
	Action       ConfirmAction
	ResponseText string
}

type CategoryChoiceAction int

const (
	CategoryChoiceActionSelected CategoryChoiceAction = iota + 1
	CategoryChoiceActionRootOnly
	CategoryChoiceActionAmbiguous
	CategoryChoiceActionReprompt
)

type CategoryChoiceDecision struct {
	Action    CategoryChoiceAction
	Candidate PendingCategoryCandidate
}

var (
	weekdayNames = map[string]time.Weekday{
		"segunda": time.Monday,
		"terca":   time.Tuesday,
		"quarta":  time.Wednesday,
		"quinta":  time.Thursday,
		"sexta":   time.Friday,
		"sabado":  time.Saturday,
		"domingo": time.Sunday,
	}

	reMoney       = regexp.MustCompile(`(?i)R\$\s*[\d.,]+`)
	reLaunchVerbs = regexp.MustCompile(`(?i)\b(gastei|paguei|comprei|recebi|ganhei)\b`)

	reCancelPhrases = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^cancela(r)?$`),
		regexp.MustCompile(`(?i)^deixa\s+pra\s+lá$`),
		regexp.MustCompile(`(?i)^não\s+registra(r)?$`),
		regexp.MustCompile(`(?i)^nao\s+registra(r)?$`),
	}

	reConfirmYes = regexp.MustCompile(`(?i)^(sim|confirmar|confirma|ok|pode)$`)
	reConfirmNo  = regexp.MustCompile(`(?i)^(não|nao|cancela|cancels|deixa\s+pra\s+lá|não\s+registra)$`)

	knownPaymentMethods = map[string]string{
		"pix":             "pix",
		"debito":          "debit_card",
		"debito em conta": "debit_in_account",
		"credito":         "credit_card",
		"cartao":          "credit_card",
		"dinheiro":        "cash",
		"especie":         "cash",
		"boleto":          "boleto",
		"ted":             "ted",
		"doc":             "ted",
		"transferencia":   "ted",
	}
)

func isExpired(state PendingEntryState, now time.Time) bool {
	return !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > pendingTTL
}

func isCancelMessage(text string) bool {
	normalized := strings.TrimSpace(text)
	for _, re := range reCancelPhrases {
		if re.MatchString(normalized) {
			return true
		}
	}
	return false
}

func isNewCompleteOperation(text string) bool {
	return reMoney.MatchString(text) && reLaunchVerbs.MatchString(text)
}

func recognizePaymentMethod(text string) string {
	normalized := normalizeText(text)
	if pm, ok := knownPaymentMethods[normalized]; ok {
		return pm
	}
	return ""
}

func parseWeekday(text string, now time.Time) (string, bool) {
	normalized := normalizeText(text)
	past := false
	if strings.HasSuffix(normalized, " passada") {
		past = true
		normalized = strings.TrimSuffix(normalized, " passada")
	} else if strings.HasSuffix(normalized, " passado") {
		past = true
		normalized = strings.TrimSuffix(normalized, " passado")
	}
	normalized = strings.TrimSuffix(normalized, "-feira")
	wd, ok := weekdayNames[normalized]
	if !ok {
		return "", false
	}
	loc := now.Location()
	today := now.In(loc)
	daysBack := (int(today.Weekday()) - int(wd) + 7) % 7
	result := today.AddDate(0, 0, -daysBack)
	if past {
		result = result.AddDate(0, 0, -7)
	}
	return result.Format("2006-01-02"), true
}

func parseInputDate(text string, now time.Time) string {
	lower := normalizeText(text)
	switch lower {
	case "hoje", "today":
		return now.Format("2006-01-02")
	case "ontem", "yesterday":
		return now.Add(-24 * time.Hour).Format("2006-01-02")
	case "anteontem":
		return now.Add(-48 * time.Hour).Format("2006-01-02")
	}
	if d, ok := parseWeekday(text, now); ok {
		return d
	}
	if len(text) == 5 && text[2] == '/' {
		day, errD := strconv.Atoi(text[:2])
		month, errM := strconv.Atoi(text[3:])
		if errD == nil && errM == nil && day >= 1 && day <= 31 && month >= 1 && month <= 12 {
			return time.Date(now.Year(), time.Month(month), day, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		}
	}
	if t, err := time.Parse("2006-01-02", text); err == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

func DecidePendingResume(state PendingEntryState, msg PendingMessage, now time.Time) (PendingDecision, error) {
	if isExpired(state, now) {
		return PendingDecision{Action: PendingActionExpire}, nil
	}

	text := strings.TrimSpace(msg.Text)

	if isCancelMessage(text) {
		return PendingDecision{Action: PendingActionCancel}, nil
	}

	if isNewCompleteOperation(text) {
		return PendingDecision{Action: PendingActionReplace}, nil
	}

	if state.Awaiting == AwaitingSlotPaymentMethod {
		if pm := recognizePaymentMethod(text); pm != "" {
			return PendingDecision{Action: PendingActionFillSlot, SlotFilled: AwaitingSlotPaymentMethod, FilledValue: pm}, nil
		}
	}

	if state.Awaiting == AwaitingSlotDate {
		if d := parseInputDate(text, now); d != "" {
			return PendingDecision{Action: PendingActionFillSlot, SlotFilled: AwaitingSlotDate, FilledValue: d}, nil
		}
	}

	if state.Awaiting == AwaitingSlotConfirmation {
		return PendingDecision{Action: PendingActionFillSlot, SlotFilled: AwaitingSlotConfirmation, FilledValue: text}, nil
	}

	if state.RepromptCount >= maxReprompts {
		return PendingDecision{Action: PendingActionCancel}, nil
	}

	return PendingDecision{Action: PendingActionReprompt}, nil
}

func DecideConfirmation(state PendingEntryState, msg PendingMessage, now time.Time) (ConfirmDecision, error) {
	if isExpired(state, now) {
		return ConfirmDecision{Action: ConfirmActionExpire}, nil
	}

	if msg.MessageID != "" && msg.MessageID == state.ProcessedMessageID {
		return ConfirmDecision{Action: ConfirmActionReplay}, nil
	}

	text := strings.TrimSpace(msg.Text)

	if reConfirmYes.MatchString(text) {
		return ConfirmDecision{Action: ConfirmActionAccept}, nil
	}

	if reConfirmNo.MatchString(text) || isCancelMessage(text) {
		return ConfirmDecision{Action: ConfirmActionCancel}, nil
	}

	if state.ConfirmRepromptCount >= maxReprompts {
		return ConfirmDecision{Action: ConfirmActionCancel}, nil
	}

	return ConfirmDecision{Action: ConfirmActionReprompt}, nil
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	result := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range result {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func DecideCategoryChoice(state PendingEntryState, candidates []PendingCategoryCandidate, text string) (CategoryChoiceDecision, error) {
	text = strings.TrimSpace(text)

	if idx, err := strconv.Atoi(text); err == nil {
		if idx >= 1 && idx <= len(candidates) {
			c := candidates[idx-1]
			if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID {
				return CategoryChoiceDecision{Action: CategoryChoiceActionRootOnly, Candidate: c}, nil
			}
			return CategoryChoiceDecision{Action: CategoryChoiceActionSelected, Candidate: c}, nil
		}
		return CategoryChoiceDecision{Action: CategoryChoiceActionReprompt}, nil
	}

	normalized := normalizeText(text)
	var matches []PendingCategoryCandidate
	for _, c := range candidates {
		if normalizeText(c.SubcategorySlug) == normalized || normalizeText(c.Path) == normalized {
			matches = append(matches, c)
		}
	}

	if len(matches) == 1 {
		c := matches[0]
		if c.SubcategoryID == (uuid.UUID{}) || c.SubcategoryID == c.RootCategoryID || c.SubcategorySlug == "" {
			return CategoryChoiceDecision{Action: CategoryChoiceActionRootOnly, Candidate: c}, nil
		}
		return CategoryChoiceDecision{Action: CategoryChoiceActionSelected, Candidate: c}, nil
	}

	if len(matches) > 1 {
		return CategoryChoiceDecision{Action: CategoryChoiceActionAmbiguous}, nil
	}

	return CategoryChoiceDecision{Action: CategoryChoiceActionReprompt}, nil
}

func DecideNewOperationReplacement(state PendingEntryState, msg PendingMessage) PendingDecision {
	if isNewCompleteOperation(strings.TrimSpace(msg.Text)) {
		return PendingDecision{Action: PendingActionReplace}
	}
	return PendingDecision{Action: PendingActionNone}
}
