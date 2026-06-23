package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

const (
	defaultProseMaxTokens         = 200
	conversationalRedirectMessage = "Estou aqui para cuidar das suas finanças: categorias, cartões, orçamento e lançamentos. Como posso te ajudar com isso? 💪"
)

var ErrComposeEmptyText = errors.New("agent.llm.usecase.compose_conversational_reply: empty text")

var updateWorkingMemoryTool = interfaces.ToolSpec{
	Name:        "updateWorkingMemory",
	Description: "Atualiza a memória persistente do usuário com novas informações aprendidas nesta conversa. Passe o template completo preenchido como string.",
	Parameters: map[string]any{
		"type": "object",
		"properties": map[string]any{
			"memory": map[string]any{
				"type":        "string",
				"description": "Conteúdo completo atualizado da memória em markdown (string, nunca objeto).",
			},
		},
		"required":             []string{"memory"},
		"additionalProperties": false,
	},
}

type conversationalInterpreter interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

type sessionReader interface {
	GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (interfaces.AgentSessionRecord, error)
	Upsert(ctx context.Context, record interfaces.AgentSessionRecord) error
}

type workingMemoryReader interface {
	Get(ctx context.Context, userID uuid.UUID) (entities.WorkingMemory, bool, error)
	Upsert(ctx context.Context, wm entities.WorkingMemory) error
}

type observationLoader interface {
	LoadContext(ctx context.Context, userID uuid.UUID, channel string) string
	MaybeTrigger(ctx context.Context, userID uuid.UUID, channel string, turns []entities.ConversationMessage)
}

type textSanitizer interface {
	Clean(raw string) (string, error)
}

type ComposeConversationalReply struct {
	interpreter    conversationalInterpreter
	o11y           observability.Observability
	maxTokens      int
	repliedTotal   observability.Counter
	turnHistory    services.TurnHistory
	sessionRepo    sessionReader
	wmRepo         workingMemoryReader
	observationSvc observationLoader
	sanitizer      textSanitizer
}

type ComposeConversationalInput struct {
	UserID  uuid.UUID
	Channel string
	Text    string
}

type ComposeConversationalOutput struct {
	Reply string
}

func NewComposeConversationalReply(
	interpreter conversationalInterpreter,
	maxTokens int,
	o11y observability.Observability,
	sessionRepo sessionReader,
	wmRepo workingMemoryReader,
	observationSvc observationLoader,
	sanitizer textSanitizer,
) (*ComposeConversationalReply, error) {
	if interpreter == nil {
		return nil, fmt.Errorf("agent.llm.usecase.compose_conversational_reply: interpreter is nil")
	}
	if o11y == nil {
		return nil, fmt.Errorf("agent.llm.usecase.compose_conversational_reply: observability is nil")
	}
	if maxTokens <= 0 {
		maxTokens = defaultProseMaxTokens
	}
	repliedTotal := o11y.Metrics().Counter(
		"agent_conversational_reply_total",
		"Total de respostas conversacionais do agent por outcome",
		"1",
	)
	return &ComposeConversationalReply{
		interpreter:    interpreter,
		o11y:           o11y,
		maxTokens:      maxTokens,
		repliedTotal:   repliedTotal,
		sessionRepo:    sessionRepo,
		wmRepo:         wmRepo,
		observationSvc: observationSvc,
		sanitizer:      sanitizer,
	}, nil
}

func (uc *ComposeConversationalReply) Execute(ctx context.Context, in ComposeConversationalInput) (ComposeConversationalOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "agent.llm.usecase.compose_conversational_reply")
	defer span.End()

	trimmed := strings.TrimSpace(in.Text)
	if trimmed == "" {
		return ComposeConversationalOutput{}, ErrComposeEmptyText
	}

	systemPrompt, err := uc.buildSystemPrompt(ctx, in.UserID, in.Channel)
	if err != nil {
		span.RecordError(err)
		return ComposeConversationalOutput{}, err
	}

	llmMessages, sessionRecord, sessionFound := uc.loadHistory(ctx, in.UserID, in.Channel)

	var tools []interfaces.ToolSpec
	if uc.wmRepo != nil {
		tools = []interfaces.ToolSpec{updateWorkingMemoryTool}
	}

	resp, err := uc.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: systemPrompt,
		UserMessage:  trimmed,
		Messages:     llmMessages,
		FreeText:     true,
		MaxTokens:    uc.maxTokens,
		Tools:        tools,
		ToolChoice:   "auto",
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

	uc.processWMToolCalls(ctx, in.UserID, resp.ToolCalls)

	reply := strings.TrimSpace(string(resp.RawJSON))
	if reply == "" {
		uc.repliedTotal.Add(ctx, 1, observability.String("outcome", "empty_reply"))
		return ComposeConversationalOutput{Reply: conversationalRedirectMessage}, nil
	}
	if looksLikePseudoCode(reply) {
		uc.repliedTotal.Add(ctx, 1, observability.String("outcome", "pseudocode_blocked"))
		return ComposeConversationalOutput{Reply: conversationalRedirectMessage}, nil
	}

	uc.persistTurns(ctx, in.UserID, in.Channel, trimmed, reply, sessionRecord, sessionFound)

	uc.repliedTotal.Add(ctx, 1, observability.String("outcome", "replied"))
	return ComposeConversationalOutput{Reply: reply}, nil
}

func (uc *ComposeConversationalReply) buildSystemPrompt(ctx context.Context, userID uuid.UUID, channel string) (string, error) {
	wmContent := ""
	if uc.wmRepo != nil {
		wm, _, wmErr := uc.wmRepo.Get(ctx, userID)
		if wmErr != nil {
			uc.o11y.Logger().Warn(ctx, "agent.llm.compose_conversational_reply.wm_fetch_failed", observability.Error(wmErr))
		} else {
			wmContent = wm.Content
		}
	}

	obsContext := ""
	if uc.observationSvc != nil {
		obsContext = uc.observationSvc.LoadContext(ctx, userID, channel)
	}

	built, err := prompting.BuildContext(prompting.ContextInput{
		WorkingMemory:      wmContent,
		ObservationContext: obsContext,
	})
	if err != nil {
		return "", fmt.Errorf("agent.llm.usecase.compose_conversational_reply: build context: %w", err)
	}
	return built.SystemPrompt, nil
}

func (uc *ComposeConversationalReply) loadHistory(ctx context.Context, userID uuid.UUID, channel string) ([]interfaces.ConversationMessage, interfaces.AgentSessionRecord, bool) {
	if uc.sessionRepo == nil || userID == uuid.Nil {
		return nil, interfaces.AgentSessionRecord{}, false
	}
	rec, err := uc.sessionRepo.GetByUserAndChannel(ctx, userID, channel)
	if err != nil {
		return nil, interfaces.AgentSessionRecord{}, false
	}
	turns, deErr := uc.turnHistory.Deserialize(rec.RecentTurns)
	if deErr != nil || len(turns) == 0 {
		return nil, rec, true
	}
	sanitizedTurns := uc.sanitizeTurns(turns)
	return uc.turnHistory.ToLLMMessages(sanitizedTurns), rec, true
}

func (uc *ComposeConversationalReply) sanitizeTurns(turns []entities.ConversationMessage) []entities.ConversationMessage {
	if uc.sanitizer == nil {
		return turns
	}
	out := make([]entities.ConversationMessage, len(turns))
	for i, t := range turns {
		cleaned, cleanErr := uc.sanitizer.Clean(t.Content)
		if cleanErr == nil {
			out[i] = entities.ConversationMessage{Role: t.Role, Content: cleaned, At: t.At}
		} else {
			out[i] = t
		}
	}
	return out
}

func looksLikePseudoCode(s string) bool {
	return strings.HasPrefix(s, "print(") ||
		strings.HasPrefix(s, "default_api.") ||
		strings.Contains(s, "updateWorkingMemory(") ||
		strings.Contains(s, "register_expense(")
}

func (uc *ComposeConversationalReply) processWMToolCalls(ctx context.Context, userID uuid.UUID, calls []interfaces.ToolCall) {
	if uc.wmRepo == nil {
		return
	}
	for _, call := range calls {
		if call.FunctionName != "updateWorkingMemory" {
			continue
		}
		memory, ok := call.ArgumentsJSON["memory"].(string)
		if !ok || strings.TrimSpace(memory) == "" {
			continue
		}
		wm, found, _ := uc.wmRepo.Get(ctx, userID)
		if !found {
			wm = entities.NewWorkingMemory(userID)
		}
		wm.Update(strings.TrimSpace(memory), time.Now().UTC())
		_ = uc.wmRepo.Upsert(ctx, wm)
	}
}

func (uc *ComposeConversationalReply) persistTurns(ctx context.Context, userID uuid.UUID, channel, userMsg, reply string, sessionRecord interfaces.AgentSessionRecord, sessionFound bool) {
	if uc.sessionRepo == nil || userID == uuid.Nil {
		return
	}
	var existingTurns []entities.ConversationMessage
	if sessionFound {
		existingTurns, _ = uc.turnHistory.Deserialize(sessionRecord.RecentTurns)
	}
	updatedTurns := uc.turnHistory.Append(existingTurns, userMsg, reply, time.Now().UTC(), 3)
	serialized, serErr := uc.turnHistory.Serialize(updatedTurns)
	if serErr != nil {
		return
	}
	newRecord := sessionRecord
	if !sessionFound {
		newRecord = interfaces.AgentSessionRecord{
			ID:            uuid.New(),
			UserID:        userID,
			Channel:       channel,
			PendingAction: []byte("{}"),
		}
	}
	newRecord.RecentTurns = serialized
	newRecord.UpdatedAt = time.Now().UTC()
	newRecord.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	_ = uc.sessionRepo.Upsert(ctx, newRecord)
	if uc.observationSvc != nil {
		uc.observationSvc.MaybeTrigger(ctx, userID, channel, updatedTurns)
	}
}
