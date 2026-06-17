package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
)

const (
	defaultProseMaxTokens         = 200
	conversationalRedirectMessage = "Estou aqui para cuidar das suas finanças: categorias, cartões, orçamento e lançamentos. Como posso te ajudar com isso? 💪"
)

var ErrComposeEmptyText = errors.New("agent.llm.usecase.compose_conversational_reply: empty text")

type conversationalInterpreter interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

type ComposeConversationalReply struct {
	interpreter  conversationalInterpreter
	o11y         observability.Observability
	systemPrompt string
	maxTokens    int
	repliedTotal observability.Counter
}

type ComposeConversationalInput struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

type ComposeConversationalOutput struct {
	Reply string
}

func NewComposeConversationalReply(interpreter conversationalInterpreter, maxTokens int, o11y observability.Observability) (*ComposeConversationalReply, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.llm.usecase.compose_conversational_reply: interpreter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.llm.usecase.compose_conversational_reply: observability is nil")
	}
	if maxTokens <= 0 {
		maxTokens = defaultProseMaxTokens
	}
	systemPrompt, err := prompting.RenderPersonaSystem(prompting.PersonaSystemData{})
	if err != nil {
		return nil, fmt.Errorf("agent.llm.usecase.compose_conversational_reply: render persona: %w", err)
	}
	repliedTotal := o11y.Metrics().Counter(
		"agent_conversational_reply_total",
		"Total de respostas conversacionais do agent por outcome",
		"1",
	)
	return &ComposeConversationalReply{
		interpreter:  interpreter,
		o11y:         o11y,
		systemPrompt: systemPrompt,
		maxTokens:    maxTokens,
		repliedTotal: repliedTotal,
	}, nil
}

func (uc *ComposeConversationalReply) Execute(ctx context.Context, in ComposeConversationalInput) (ComposeConversationalOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.llm.usecase.compose_conversational_reply")
	defer span.End()

	trimmed := strings.TrimSpace(in.Text)
	if trimmed == "" {
		return ComposeConversationalOutput{}, ErrComposeEmptyText
	}

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: uc.systemPrompt,
		UserMessage:  trimmed,
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
	})
	if err != nil {
		span.RecordError(err)
		outcome := "provider_error"
		if errors.Is(err, services.ErrFallbackChainExhausted) {
			outcome = "provider_exhausted"
		}
		uc.repliedTotal.Add(ctx, 1, observability.String("outcome", outcome))
		return ComposeConversationalOutput{Reply: conversationalRedirectMessage}, nil
	}

	reply := strings.TrimSpace(string(resp.RawJSON))
	if reply == "" {
		uc.repliedTotal.Add(ctx, 1, observability.String("outcome", "empty_reply"))
		return ComposeConversationalOutput{Reply: conversationalRedirectMessage}, nil
	}

	uc.repliedTotal.Add(ctx, 1, observability.String("outcome", "replied"))
	return ComposeConversationalOutput{Reply: reply}, nil
}
