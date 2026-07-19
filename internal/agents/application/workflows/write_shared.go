package workflows

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	catusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const PaymentMethodCreditCard = "credit_card"

const MultiItemOrientationMessage = "Percebi mais de um lançamento na mesma mensagem. Por segurança, registro um de cada vez — me manda o primeiro (ex.: \"gastei 30 no ônibus\") que eu já cuido dele. 🙂"

var ErrWriteAcceptedWithoutResource = errors.New("workflows.write_shared: escrita aceita sem recurso durável")

type IdempotentWriteFn func(ctx context.Context) (resourceID uuid.UUID, reconciled bool, err error)

type DomainErrorClassifier func(error) bool

type IdempotentWriter interface {
	Execute(
		ctx context.Context,
		userID uuid.UUID,
		wamid string,
		itemSeq int,
		operation string,
		resourceKind string,
		write IdempotentWriteFn,
		isDomainErr DomainErrorClassifier,
	) (resourceID uuid.UUID, outcome agent.ToolOutcome, err error)
}

type cardNicknameSolver interface {
	ResolveCardByNickname(ctx context.Context, userID uuid.UUID, nickname string) (interfaces.Card, error)
}

type categoryValidator interface {
	SearchDictionary(ctx context.Context, term, kind string) (interfaces.CategorySearchResult, error)
	ResolveForWrite(ctx context.Context, input interfaces.CategoryWriteRequest) (interfaces.CategoryWriteDecision, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]interfaces.Category, error)
	CatalogVersion(ctx context.Context) (int64, error)
}

type PendingMessage struct {
	Text      string
	MessageID string
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

	reMoney       = regexp.MustCompile(`(?i)(R\$\s*\d[\d.,]*|\b\d[\d.,]*\s*(?:reais|real|contos?|pilas?|mangos?)\b|\b\d{1,3}(?:\.\d{3})*,\d{2}\b)`)
	reLaunchVerbs = regexp.MustCompile(`(?i)\b(gastei|paguei|comprei|recebi|ganhei)\b`)

	reDescriptionParaphrase = regexp.MustCompile(`(?i)^(?:compras?|pagamentos?|recebimentos?|gastos?|paguei|comprei|recebi|gastei)\s+(?:de|do|da|dos|das|no|na|nos|nas|em|com)\s+(.+)$`)

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
		"doc":             "doc",
		"transferencia":   "transferencia",
		"apple pay":       "apple_pay",
		"applepay":        "apple_pay",
		"google pay":      "google_pay",
		"googlepay":       "google_pay",
		"picpay":          "picpay",
		"mercado pago":    "mercado_pago",
		"mercadopago":     "mercado_pago",
		"cheque":          "cheque",

		"cartao de credito": "credit_card",
		"cartao de debito":  "debit_card",
		"cartao credito":    "credit_card",
		"cartao debito":     "debit_card",
		"vale refeicao":     "vale_refeicao",
		"vale-refeicao":     "vale_refeicao",
		"vr":                "vale_refeicao",
		"vale alimentacao":  "vale_alimentacao",
		"vale-alimentacao":  "vale_alimentacao",
		"va":                "vale_alimentacao",
	}

	paymentMethodLeadWords = map[string]bool{
		"paguei": true,
		"pagou":  true,
		"pago":   true,
		"foi":    true,
		"usei":   true,
		"no":     true,
		"na":     true,
		"em":     true,
		"com":    true,
		"de":     true,
		"do":     true,
		"da":     true,
		"pelo":   true,
		"pela":   true,
	}
)

func isSim(s string) bool {
	switch s {
	case "sim", "confirmar", "confirmo", "ok", "pode", "yes", "s":
		return true
	default:
		return false
	}
}

func isNao(s string) bool {
	switch s {
	case "não", "nao", "cancelar", "cancelo", "no", "n":
		return true
	default:
		return false
	}
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

func NormalizeEntryDescription(description string) string {
	trimmed := strings.TrimSpace(description)
	if match := reDescriptionParaphrase.FindStringSubmatch(trimmed); match != nil {
		if rest := strings.TrimSpace(match[1]); rest != "" {
			return rest
		}
	}
	return trimmed
}

type PaymentAnswer struct {
	Method   string
	CardHint string
}

func DecidePaymentAnswer(text string) PaymentAnswer {
	words := strings.Fields(normalizeText(text))
	for {
		if pm, ok := knownPaymentMethods[strings.Join(words, " ")]; ok {
			return PaymentAnswer{Method: pm}
		}
		if len(words) > 1 && paymentMethodLeadWords[words[0]] {
			words = words[1:]
			continue
		}
		break
	}
	for i := len(words) - 1; i >= 1; i-- {
		pm, ok := knownPaymentMethods[strings.Join(words[:i], " ")]
		if !ok || pm != PaymentMethodCreditCard {
			continue
		}
		return PaymentAnswer{Method: pm, CardHint: strings.Join(words[i:], " ")}
	}
	return PaymentAnswer{}
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

func isCategoryBusinessRejection(err error) bool {
	businessErrs := []error{
		catusecases.ErrRootCategoryNotFound,
		catusecases.ErrSubcategoryNotFound,
		catusecases.ErrRootWithoutLeaf,
		catusecases.ErrLeafNotFromRoot,
		catusecases.ErrCategoryDeprecated,
		catusecases.ErrKindMismatch,
		catusecases.ErrVersionDrift,
	}
	for _, be := range businessErrs {
		if errors.Is(err, be) {
			return true
		}
	}
	return false
}

var paymentMethodLabels = map[string]string{
	"pix":              "pix",
	"debit_card":       "débito",
	"debit_in_account": "débito em conta",
	"cash":             "dinheiro",
	"boleto":           "boleto",
	"credit_card":      "crédito",
	"vale_refeicao":    "vale-refeição",
	"vale_alimentacao": "vale-alimentação",
	"ted":              "TED",
	"doc":              "DOC",
	"transferencia":    "transferência",
	"apple_pay":        "Apple Pay",
	"google_pay":       "Google Pay",
	"picpay":           "PicPay",
	"mercado_pago":     "Mercado Pago",
	"cheque":           "cheque",
}

func formatPaymentLabel(method string) string {
	if label, ok := paymentMethodLabels[method]; ok {
		return label
	}
	return method
}

func buildCandidatesPrompt(candidates []PendingCategoryCandidate) string {
	if len(candidates) == 0 {
		return messages.ClarificationQuestion(messages.MissingFieldCategory)
	}
	if candidates[0].SubcategoryID == uuid.Nil {
		return messages.CategoryRootOptions(candidatePathOptions(candidates))
	}
	rootName, sameRoot := candidatesShareRoot(candidates)
	if sameRoot {
		return messages.CategorySubcategoryOptions(rootName, candidateLeafOptions(candidates))
	}
	return messages.CategoryLeafOptions(candidatePathOptions(candidates))
}

func candidatePathOptions(candidates []PendingCategoryCandidate) []string {
	options := make([]string, 0, len(candidates))
	for _, c := range candidates {
		options = append(options, c.Path)
	}
	return options
}

func candidateLeafOptions(candidates []PendingCategoryCandidate) []string {
	options := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if leaf := candidateLeafName(c); leaf != "" {
			options = append(options, leaf)
			continue
		}
		options = append(options, c.Path)
	}
	return options
}

func candidatesShareRoot(candidates []PendingCategoryCandidate) (string, bool) {
	root, _, ok := splitCandidatePath(candidates[0].Path)
	if !ok {
		return "", false
	}
	for _, c := range candidates[1:] {
		other, _, otherOK := splitCandidatePath(c.Path)
		if !otherOK || other != root {
			return "", false
		}
	}
	return root, true
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
