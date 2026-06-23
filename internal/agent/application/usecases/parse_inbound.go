package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type IntentInterpreter interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

type ParseInboundInput struct {
	UserID uuid.UUID
	Text   string
}

type ParseInboundOutput struct {
	Intent       intent.Intent
	Confidence   valueobjects.Confidence
	Raw          []byte
	DirectReply  string
	LLMModel     string
	PromptSHA256 string
}

type ParseInbound struct {
	interpreter         IntentInterpreter
	retry               IntentInterpreter
	sanitizer           *sanitize.Sanitizer
	o11y                observability.Observability
	parsedTotal         observability.Counter
	decodeFailedTotal   observability.Counter
	retryTotal          observability.Counter
	confidenceHistogram observability.Histogram
	schema              *interfaces.JSONSchemaSpec
}

func NewParseInbound(interpreter IntentInterpreter, retry IntentInterpreter, maxInputChars int, o11y observability.Observability) (*ParseInbound, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.usecase.parse_inbound: interpreter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.parse_inbound: observability is nil")
	}
	sanitizer, err := sanitize.NewSanitizer(maxInputChars)
	if err != nil {
		return nil, fmt.Errorf("agent.usecase.parse_inbound: sanitizer: %w", err)
	}
	parsedTotal := o11y.Metrics().Counter(
		"agent_intent_parsed_total",
		"Total de intents extraídas do parser PT-BR por kind e outcome",
		"1",
	)
	decodeFailedTotal := o11y.Metrics().Counter(
		"agent_intent_parse_decode_failed_total",
		"Total de falhas de decode do JSON de intent retornado pelo provider LLM por motivo",
		"1",
	)
	retryTotal := o11y.Metrics().Counter(
		"agent_parse_retry_total",
		"Total de re-tentativas do parser via fallback quando o primário retorna unknown, por outcome",
		"1",
	)
	confidenceHistogram := o11y.Metrics().HistogramWithBuckets(
		"agent_intent_confidence_histogram",
		"Distribuição da confiança reportada pelo parser de intent por kind",
		"1",
		[]float64{0.1, 0.25, 0.5, 0.65, 0.8, 0.9, 0.95, 1},
	)
	return &ParseInbound{
		interpreter:         interpreter,
		retry:               retry,
		sanitizer:           sanitizer,
		o11y:                o11y,
		parsedTotal:         parsedTotal,
		decodeFailedTotal:   decodeFailedTotal,
		retryTotal:          retryTotal,
		confidenceHistogram: confidenceHistogram,
		schema: &interfaces.JSONSchemaSpec{
			Name:   "mecontrola_parse_intent",
			Strict: false,
			Schema: prompting.ParseIntentJSONSchema(),
		},
	}, nil
}

var ErrParseInboundEmptyText = errors.New("agent.usecase.parse_inbound: text is empty")

const (
	outcomeOK              = "ok"
	outcomeDirectReply     = "direct_reply"
	outcomeFallbackInvalid = "fallback_invalid_json"
	outcomeFallbackMissing = "fallback_missing_kind"
	outcomeFallbackDomain  = "fallback_domain_invariant"
	outcomeProviderError   = "provider_error"
)

const (
	retryOutcomeRecovered     = "recovered"
	retryOutcomeStillUnknown  = "still_unknown"
	retryOutcomeSkippedNotCmd = "skipped_not_command"
)

func (uc *ParseInbound) Execute(ctx context.Context, input ParseInboundInput) (ParseInboundOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.parse_inbound")
	defer span.End()

	trimmed, err := uc.sanitizer.Clean(input.Text)
	if err != nil {
		if errors.Is(err, sanitize.ErrEmpty) {
			return ParseInboundOutput{}, ErrParseInboundEmptyText
		}
		return ParseInboundOutput{}, fmt.Errorf("agent.usecase.parse_inbound: sanitize: %w", err)
	}

	promptDigest := digestPrompt(trimmed)

	system, err := prompting.RenderSystem()
	if err != nil {
		return ParseInboundOutput{}, fmt.Errorf("agent.usecase.parse_inbound: render system: %w", err)
	}
	user, err := prompting.RenderUser(trimmed)
	if err != nil {
		return ParseInboundOutput{}, fmt.Errorf("agent.usecase.parse_inbound: render user: %w", err)
	}

	req := interfaces.LLMRequest{
		SystemPrompt: system,
		UserMessage:  user,
		JSONSchema:   uc.schema,
	}

	resp, err := uc.interpreter.Interpret(ctx, req)
	if err != nil {
		span.RecordError(err)
		uc.recordOutcome(ctx, intent.KindUnknown, outcomeProviderError)
		fallback, fbErr := intent.NewUnknown(trimmed)
		if fbErr != nil {
			return ParseInboundOutput{}, errors.Join(fmt.Errorf("agent.usecase.parse_inbound: provider: %w", err), fbErr)
		}
		out := ParseInboundOutput{Intent: fallback, PromptSHA256: promptDigest}
		return uc.maybeRetry(ctx, out, req, trimmed, promptDigest), nil
	}

	out, err := uc.fromContent(ctx, resp, trimmed, promptDigest)
	if err != nil {
		return out, err
	}
	return uc.maybeRetry(ctx, out, req, trimmed, promptDigest), nil
}

func (uc *ParseInbound) maybeRetry(ctx context.Context, out ParseInboundOutput, req interfaces.LLMRequest, trimmed, promptDigest string) ParseInboundOutput {
	if out.Intent.Kind() != intent.KindUnknown || uc.retry == nil {
		return out
	}
	if !looksLikeCommand(trimmed) {
		uc.retryTotal.Add(ctx, 1, observability.String("outcome", retryOutcomeSkippedNotCmd))
		return out
	}

	resp, err := uc.retry.Interpret(ctx, req)
	if err != nil {
		uc.retryTotal.Add(ctx, 1, observability.String("outcome", retryOutcomeStillUnknown))
		return out
	}

	retried, err := uc.fromContent(ctx, resp, trimmed, promptDigest)
	if err != nil || retried.Intent.Kind() == intent.KindUnknown {
		uc.retryTotal.Add(ctx, 1, observability.String("outcome", retryOutcomeStillUnknown))
		return out
	}

	uc.retryTotal.Add(ctx, 1, observability.String("outcome", retryOutcomeRecovered))
	return retried
}

func (uc *ParseInbound) fromContent(ctx context.Context, resp interfaces.LLMResponse, trimmed, promptDigest string) (ParseInboundOutput, error) {
	llmModel := resp.Provider.String()
	parsed, confidence, parseErr := decodeAndBuild(resp.RawJSON, trimmed)
	if parseErr == nil {
		uc.recordOutcome(ctx, parsed.Kind(), outcomeOK)
		uc.confidenceHistogram.Record(ctx, confidence.Value(), observability.String("kind", parsed.Kind().String()))
		return ParseInboundOutput{Intent: parsed, Confidence: confidence, Raw: resp.RawJSON, LLMModel: llmModel, PromptSHA256: promptDigest}, nil
	}

	directReply := strings.TrimSpace(string(resp.RawJSON))
	if directReply != "" && !looksLikeJSON(directReply) {
		uc.recordOutcome(ctx, intent.KindUnknown, outcomeDirectReply)
		fallback, fbErr := intent.NewUnknown(trimmed)
		if fbErr != nil {
			return ParseInboundOutput{}, fbErr
		}
		return ParseInboundOutput{Intent: fallback, Raw: resp.RawJSON, DirectReply: directReply, LLMModel: llmModel, PromptSHA256: promptDigest}, nil
	}

	if parseErr.Outcome == outcomeFallbackInvalid {
		uc.o11y.Logger().Warn(ctx, "agent.usecase.parse_inbound.decode_failed",
			observability.String("outcome", parseErr.Outcome),
			observability.String("raw", truncateRunes(string(resp.RawJSON), decodeFailureLogRunes)),
			observability.Error(parseErr),
		)
	}
	uc.recordOutcome(ctx, intent.KindUnknown, parseErr.Outcome)
	uc.decodeFailedTotal.Add(ctx, 1, observability.String("reason", parseErr.Outcome))
	fallback, fbErr := intent.NewUnknown(trimmed)
	if fbErr != nil {
		return ParseInboundOutput{}, errors.Join(parseErr, fbErr)
	}
	return ParseInboundOutput{Intent: fallback, Raw: resp.RawJSON, LLMModel: llmModel, PromptSHA256: promptDigest}, nil
}

const decodeFailureLogRunes = 200

const defaultConfidence = 1.0

func looksLikeJSON(text string) bool {
	trimmed := strings.TrimSpace(stripFencesString(text))
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '{', '[':
		return true
	default:
		return false
	}
}

func stripFencesString(text string) string {
	return string(stripFences([]byte(text)))
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func digestPrompt(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func (uc *ParseInbound) recordOutcome(ctx context.Context, kind intent.Kind, outcome string) {
	uc.parsedTotal.Add(ctx, 1,
		observability.String("kind", kind.String()),
		observability.String("outcome", outcome),
	)
}

type parseInboundError struct {
	Outcome string
	Err     error
}

func (e *parseInboundError) Error() string { return e.Err.Error() }
func (e *parseInboundError) Unwrap() error { return e.Err }

type rawIntentDTO struct {
	Kind          string   `json:"kind"`
	AmountCents   int64    `json:"amount_cents"`
	Merchant      string   `json:"merchant"`
	CategoryHint  string   `json:"category_hint"`
	PaymentMethod string   `json:"payment_method"`
	CardHint      string   `json:"card_hint"`
	CategoryName  string   `json:"category_name"`
	GoalName      string   `json:"goal_name"`
	CardName      string   `json:"card_name"`
	CardNickname  string   `json:"nickname"`
	RefMonth      string   `json:"ref_month"`
	RawText       string   `json:"raw_text"`
	Installments  int      `json:"installments"`
	Direction     string   `json:"direction"`
	Frequency     string   `json:"frequency"`
	DayOfMonth    int      `json:"day_of_month"`
	ClosingDay    int      `json:"closing_day"`
	DueDay        int      `json:"due_day"`
	LimitCents    int64    `json:"limit_cents"`
	Percentage    int      `json:"percentage"`
	NewNickname   *string  `json:"new_nickname"`
	NewName       *string  `json:"new_name"`
	NewClosingDay *int     `json:"new_closing_day"`
	NewDueDay     *int     `json:"new_due_day"`
	Confidence    *float64 `json:"confidence"`
}

func decodeAndBuild(raw []byte, fallbackText string) (intent.Intent, valueobjects.Confidence, *parseInboundError) {
	cleaned := stripFences(raw)
	if len(cleaned) == 0 {
		return intent.Intent{}, valueobjects.Confidence{}, &parseInboundError{Outcome: outcomeFallbackInvalid, Err: fmt.Errorf("agent.usecase.parse_inbound: empty payload")}
	}

	var dto rawIntentDTO
	if err := json.Unmarshal(cleaned, &dto); err != nil {
		return intent.Intent{}, valueobjects.Confidence{}, &parseInboundError{Outcome: outcomeFallbackInvalid, Err: fmt.Errorf("agent.usecase.parse_inbound: unmarshal: %w", err)}
	}

	if strings.TrimSpace(dto.Kind) == "" {
		return intent.Intent{}, valueobjects.Confidence{}, &parseInboundError{Outcome: outcomeFallbackMissing, Err: fmt.Errorf("agent.usecase.parse_inbound: missing kind")}
	}

	kind, err := intent.ParseKind(dto.Kind)
	if err != nil {
		return intent.Intent{}, valueobjects.Confidence{}, &parseInboundError{Outcome: outcomeFallbackMissing, Err: err}
	}

	built, err := build(kind, dto, fallbackText)
	if err != nil {
		return intent.Intent{}, valueobjects.Confidence{}, &parseInboundError{Outcome: outcomeFallbackDomain, Err: err}
	}
	return built, resolveConfidence(dto.Confidence), nil
}

func resolveConfidence(raw *float64) valueobjects.Confidence {
	value := defaultConfidence
	if raw != nil {
		value = min(max(*raw, 0), 1)
	}
	confidence, err := valueobjects.NewConfidence(value)
	if err != nil {
		neutral, _ := valueobjects.NewConfidence(defaultConfidence)
		return neutral
	}
	return confidence
}

func build(kind intent.Kind, dto rawIntentDTO, fallbackText string) (intent.Intent, error) { //nolint:revive // dispatch exaustivo por intent kind
	switch kind {
	case intent.KindRecordExpense:
		return intent.NewRecordExpense(intent.RecordExpenseFields{
			AmountCents:   dto.AmountCents,
			Merchant:      dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
			CardHint:      dto.CardHint,
		})
	case intent.KindRecordIncome:
		return intent.NewRecordIncome(intent.RecordIncomeFields{
			AmountCents:   dto.AmountCents,
			Source:        dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
		})
	case intent.KindQueryCategory:
		return intent.NewQueryCategory(dto.CategoryName)
	case intent.KindQueryGoal:
		return intent.NewQueryGoal(dto.GoalName)
	case intent.KindQueryCard:
		return intent.NewQueryCard(dto.CardName)
	case intent.KindMonthlySummary:
		return intent.NewMonthlySummary(dto.RefMonth)
	case intent.KindHowAmIDoing:
		return intent.NewHowAmIDoing(), nil
	case intent.KindConfigureBudget:
		return intent.NewConfigureBudget(), nil
	case intent.KindRecordCardPurchase:
		return intent.NewRecordCardPurchase(intent.RecordCardPurchaseFields{
			AmountCents:  dto.AmountCents,
			Merchant:     dto.Merchant,
			CategoryHint: dto.CategoryHint,
			CardHint:     dto.CardHint,
			Installments: resolveInstallments(dto.Installments, fallbackText),
		})
	case intent.KindListTransactions:
		return intent.NewListTransactions(dto.RefMonth)
	case intent.KindDeleteLastTransaction:
		return intent.NewDeleteLastTransaction(), nil
	case intent.KindEditLastTransaction:
		return intent.NewEditLastTransaction(dto.AmountCents)
	case intent.KindCreateRecurring:
		dayOfMonth := dto.DayOfMonth
		if dayOfMonth <= 0 {
			dayOfMonth = 1
		}
		return intent.NewCreateRecurring(intent.CreateRecurringFields{
			AmountCents:  dto.AmountCents,
			Merchant:     dto.Merchant,
			CategoryHint: dto.CategoryHint,
			Direction:    inferRecurringDirection(dto.Direction, dto.Merchant, fallbackText),
			Frequency:    dto.Frequency,
			DayOfMonth:   dayOfMonth,
		})
	case intent.KindListRecurring:
		return intent.NewListRecurring(), nil
	case intent.KindListCards:
		return intent.NewListCards(), nil
	case intent.KindCreateCard:
		return intent.NewCreateCard(intent.CreateCardFields{
			Nickname:   dto.CardNickname,
			Name:       dto.CardName,
			ClosingDay: dto.ClosingDay,
			DueDay:     dto.DueDay,
			LimitCents: dto.LimitCents,
		})
	case intent.KindCountCards:
		return intent.NewCountCards(), nil
	case intent.KindUpdateCard:
		return intent.NewUpdateCard(intent.UpdateCardFields{
			CardName:   dto.CardName,
			Nickname:   dto.NewNickname,
			Name:       dto.NewName,
			ClosingDay: nilIfZeroDay(dto.NewClosingDay),
			DueDay:     nilIfZeroDay(dto.NewDueDay),
		})
	case intent.KindDeleteCard:
		return intent.NewDeleteCard(dto.CardName)
	case intent.KindEditCategoryPercentage:
		return intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
			CategoryName: dto.CategoryName,
			Percentage:   dto.Percentage,
		})
	case intent.KindUnknown:
		raw := dto.RawText
		if strings.TrimSpace(raw) == "" {
			raw = fallbackText
		}
		return intent.NewUnknown(raw)
	default:
		return intent.Intent{}, fmt.Errorf("agent.usecase.parse_inbound: unsupported kind %v", kind)
	}
}

func nilIfZeroDay(day *int) *int {
	if day != nil && *day == 0 {
		return nil
	}
	return day
}

var commandCues = []string{
	"gastei", "gasto", "comprei", "paguei", "recebi", "salário", "salario",
	"cartão", "cartao", "apaga", "apagar", "remove", "remover", "deleta", "deletar",
	"exclui", "excluir", "renomeia", "renomear", "apelido", "vencimento", "vence",
	"fechamento", "fechar", "fatura", "limite",
	"orçamento", "orcamento", "percentual", "%", "por cento", "categoria",
	"lançar", "lancar", "lança", "lanca", "registra", "registrar",
	"edita", "editar", "altera", "alterar", "muda", "mudar", "troca", "trocar",
}

func looksLikeCommand(text string) bool {
	haystack := strings.ToLower(text)
	for _, cue := range commandCues {
		if strings.Contains(haystack, cue) {
			return true
		}
	}
	return false
}

var recurringIncomeCues = []string{
	"salário", "salario", "recebo", "recebimento", "pró-labore", "pro-labore",
	"prolabore", "rendimento", "provento", "pensão", "pensao", "aposentadoria",
	"aluguel recebido", "freela", "freelance",
}

const minCardInstallments = 2

func resolveInstallments(raw int, text string) int {
	if raw >= minCardInstallments {
		return raw
	}
	re, err := regexp.Compile(`(?i)(\d{1,2})\s*(?:x\b|vez(?:es)?|parcelas?)`)
	if err != nil {
		return raw
	}
	match := re.FindStringSubmatch(text)
	if len(match) < 2 {
		return raw
	}
	parsed, convErr := strconv.Atoi(match[1])
	if convErr != nil {
		return raw
	}
	return parsed
}

func inferRecurringDirection(direction, merchant, fallbackText string) string {
	trimmed := strings.ToLower(strings.TrimSpace(direction))
	if trimmed == "income" || trimmed == "outcome" {
		return trimmed
	}
	if trimmed == "expense" {
		return "outcome"
	}
	haystack := strings.ToLower(merchant + " " + fallbackText)
	for _, cue := range recurringIncomeCues {
		if strings.Contains(haystack, cue) {
			return "income"
		}
	}
	return "outcome"
}

func stripFences(raw []byte) []byte {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "```") {
		return []byte(trimmed)
	}
	cleaned := strings.TrimPrefix(trimmed, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return []byte(strings.TrimSpace(cleaned))
}
