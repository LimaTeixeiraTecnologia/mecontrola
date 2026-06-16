package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
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
	Intent intent.Intent
	Raw    []byte
}

type ParseInbound struct {
	interpreter       IntentInterpreter
	o11y              observability.Observability
	parsedTotal       observability.Counter
	decodeFailedTotal observability.Counter
	schema            *interfaces.JSONSchemaSpec
}

func NewParseInbound(interpreter IntentInterpreter, o11y observability.Observability) (*ParseInbound, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.usecase.parse_inbound: interpreter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.usecase.parse_inbound: observability is nil")
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
	schema := &interfaces.JSONSchemaSpec{
		Name:   "mecontrola_parse_intent",
		Strict: true,
		Schema: prompting.ParseIntentJSONSchema(),
	}
	return &ParseInbound{
		interpreter:       interpreter,
		o11y:              o11y,
		parsedTotal:       parsedTotal,
		decodeFailedTotal: decodeFailedTotal,
		schema:            schema,
	}, nil
}

var ErrParseInboundEmptyText = errors.New("agent.usecase.parse_inbound: text is empty")

const (
	outcomeOK              = "ok"
	outcomeFallbackInvalid = "fallback_invalid_json"
	outcomeFallbackMissing = "fallback_missing_kind"
	outcomeFallbackDomain  = "fallback_domain_invariant"
	outcomeProviderError   = "provider_error"
)

func (uc *ParseInbound) Execute(ctx context.Context, input ParseInboundInput) (ParseInboundOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.usecase.parse_inbound")
	defer span.End()

	trimmed := strings.TrimSpace(input.Text)
	if trimmed == "" {
		return ParseInboundOutput{}, ErrParseInboundEmptyText
	}

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
		return ParseInboundOutput{Intent: fallback}, nil
	}

	parsed, parseErr := decodeAndBuild(resp.RawJSON, trimmed)
	if parseErr != nil {
		span.RecordError(parseErr)
		uc.recordOutcome(ctx, intent.KindUnknown, parseErr.Outcome)
		uc.decodeFailedTotal.Add(ctx, 1, observability.String("reason", parseErr.Outcome))
		fallback, fbErr := intent.NewUnknown(trimmed)
		if fbErr != nil {
			return ParseInboundOutput{}, errors.Join(parseErr, fbErr)
		}
		return ParseInboundOutput{Intent: fallback, Raw: resp.RawJSON}, nil
	}

	uc.recordOutcome(ctx, parsed.Kind(), outcomeOK)
	return ParseInboundOutput{Intent: parsed, Raw: resp.RawJSON}, nil
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
	RefMonth      string `json:"ref_month"`
	RawText       string `json:"raw_text"`
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

func build(kind intent.Kind, dto rawIntentDTO, fallbackText string) (intent.Intent, error) {
	switch kind {
	case intent.KindLogExpense:
		return intent.NewLogExpense(intent.LogExpenseFields{
			AmountCents:   dto.AmountCents,
			Merchant:      dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
			CardHint:      dto.CardHint,
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
