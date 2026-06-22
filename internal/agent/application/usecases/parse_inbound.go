package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/sanitize"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
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
	Raw          []byte
	DirectReply  string
	LLMModel     string
	PromptSHA256 string
}

type ParseInbound struct {
	interpreter       IntentInterpreter
	sanitizer         *sanitize.Sanitizer
	o11y              observability.Observability
	parsedTotal       observability.Counter
	decodeFailedTotal observability.Counter
	schema            *interfaces.JSONSchemaSpec
}

func NewParseInbound(interpreter IntentInterpreter, maxInputChars int, o11y observability.Observability) (*ParseInbound, error) {
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
	return &ParseInbound{
		interpreter:       interpreter,
		sanitizer:         sanitizer,
		o11y:              o11y,
		parsedTotal:       parsedTotal,
		decodeFailedTotal: decodeFailedTotal,
		schema: &interfaces.JSONSchemaSpec{
			Name:   "mecontrola_parse_intent",
			Strict: true,
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

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: system,
		UserMessage:  user,
		JSONSchema:   uc.schema,
	})
	if err != nil {
		span.RecordError(err)
		uc.recordOutcome(ctx, intent.KindUnknown, outcomeProviderError)
		fallback, fbErr := intent.NewUnknown(trimmed)
		if fbErr != nil {
			return ParseInboundOutput{}, errors.Join(fmt.Errorf("agent.usecase.parse_inbound: provider: %w", err), fbErr)
		}
		return ParseInboundOutput{Intent: fallback, PromptSHA256: promptDigest}, nil
	}

	return uc.fromContent(ctx, resp, trimmed, promptDigest)
}

func (uc *ParseInbound) fromContent(ctx context.Context, resp interfaces.LLMResponse, trimmed, promptDigest string) (ParseInboundOutput, error) {
	llmModel := resp.Provider.String()
	parsed, parseErr := decodeAndBuild(resp.RawJSON, trimmed)
	if parseErr == nil {
		uc.recordOutcome(ctx, parsed.Kind(), outcomeOK)
		return ParseInboundOutput{Intent: parsed, Raw: resp.RawJSON, LLMModel: llmModel, PromptSHA256: promptDigest}, nil
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
	Kind          string `json:"kind"`
	AmountCents   int64  `json:"amount_cents"`
	Merchant      string `json:"merchant"`
	CategoryHint  string `json:"category_hint"`
	PaymentMethod string `json:"payment_method"`
	CardHint      string `json:"card_hint"`
	CategoryName  string `json:"category_name"`
	GoalName      string `json:"goal_name"`
	CardName      string `json:"card_name"`
	CardNickname  string `json:"nickname"`
	RefMonth      string `json:"ref_month"`
	RawText       string `json:"raw_text"`
	Installments  int    `json:"installments"`
	Direction     string `json:"direction"`
	Frequency     string `json:"frequency"`
	DayOfMonth    int    `json:"day_of_month"`
	ClosingDay    int    `json:"closing_day"`
	DueDay        int    `json:"due_day"`
	LimitCents    int64  `json:"limit_cents"`
}

func decodeAndBuild(raw []byte, fallbackText string) (intent.Intent, *parseInboundError) {
	cleaned := stripFences(raw)
	if len(cleaned) == 0 {
		return intent.Intent{}, &parseInboundError{Outcome: outcomeFallbackInvalid, Err: fmt.Errorf("agent.usecase.parse_inbound: empty payload")}
	}

	var dto rawIntentDTO
	if err := json.Unmarshal(cleaned, &dto); err != nil {
		return intent.Intent{}, &parseInboundError{Outcome: outcomeFallbackInvalid, Err: fmt.Errorf("agent.usecase.parse_inbound: unmarshal: %w", err)}
	}

	if strings.TrimSpace(dto.Kind) == "" {
		return intent.Intent{}, &parseInboundError{Outcome: outcomeFallbackMissing, Err: fmt.Errorf("agent.usecase.parse_inbound: missing kind")}
	}

	kind, err := intent.ParseKind(dto.Kind)
	if err != nil {
		return intent.Intent{}, &parseInboundError{Outcome: outcomeFallbackMissing, Err: err}
	}

	built, err := build(kind, dto, fallbackText)
	if err != nil {
		return intent.Intent{}, &parseInboundError{Outcome: outcomeFallbackDomain, Err: err}
	}
	return built, nil
}

func build(kind intent.Kind, dto rawIntentDTO, fallbackText string) (intent.Intent, error) { //nolint:revive // dispatch exaustivo por intent kind
	switch kind {
	case intent.KindLogExpense:
		return intent.NewLogExpense(intent.LogExpenseFields{
			AmountCents:   dto.AmountCents,
			Merchant:      dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
			CardHint:      dto.CardHint,
		})
	case intent.KindLogIncome:
		return intent.NewLogIncome(intent.LogIncomeFields{
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
	case intent.KindLogCardPurchase:
		return intent.NewLogCardPurchase(intent.LogCardPurchaseFields{
			AmountCents:  dto.AmountCents,
			Merchant:     dto.Merchant,
			CategoryHint: dto.CategoryHint,
			CardHint:     dto.CardHint,
			Installments: dto.Installments,
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

var recurringIncomeCues = []string{
	"salário", "salario", "recebo", "recebimento", "pró-labore", "pro-labore",
	"prolabore", "rendimento", "provento", "pensão", "pensao", "aposentadoria",
	"aluguel recebido", "freela", "freelance",
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
