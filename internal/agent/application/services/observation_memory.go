package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type observerInterpreter interface {
	Interpret(ctx context.Context, req interfaces.LLMRequest) (interfaces.LLMResponse, error)
}

const observationPrompt = `Você é um sistema de memória de um assistente financeiro pessoal.
Analise as mensagens recentes e extraia as 3-5 observações mais importantes que o assistente deve lembrar nas próximas conversas.

Formato:
- Alta prioridade: [fatos do usuário, preferências, metas não resolvidas]
- Média prioridade: [informações aprendidas, contexto de uso]
- Contexto recente: [no que o assistente estava trabalhando]

Seja conciso. Estas observações serão o único contexto em conversas futuras.`

type ObservationMemory struct {
	interpreter observerInterpreter
	repo        interfaces.ObservationRepository
	o11y        observability.Observability
	maxTurns    int
	maxKeep     int
}

func NewObservationMemory(
	interpreter observerInterpreter,
	repo interfaces.ObservationRepository,
	o11y observability.Observability,
	maxTurns int,
	maxKeep int,
) *ObservationMemory {
	return &ObservationMemory{
		interpreter: interpreter,
		repo:        repo,
		o11y:        o11y,
		maxTurns:    maxTurns,
		maxKeep:     maxKeep,
	}
}

func (m *ObservationMemory) MaybeTrigger(ctx context.Context, userID uuid.UUID, channel string, turns []entities.ConversationMessage) {
	if len(turns) < m.maxTurns {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("agent.observation_memory.panic_recovered", "panic", r)
			}
		}()

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := m.observe(timeoutCtx, userID, channel, turns); err != nil {
			m.o11y.Logger().Warn(timeoutCtx, "agent.observation_memory.trigger_failed",
				observability.Error(err),
			)
			return
		}

		if err := m.repo.DeleteOldestBeyondLimit(timeoutCtx, userID, channel, m.maxKeep); err != nil {
			m.o11y.Logger().Warn(timeoutCtx, "agent.observation_memory.delete_oldest_failed",
				observability.Error(err),
			)
		}
	}()
}

func (m *ObservationMemory) LoadContext(ctx context.Context, userID uuid.UUID, channel string) string {
	observations, err := m.repo.ListRecent(ctx, userID, channel, 3)
	if err != nil || len(observations) == 0 {
		return ""
	}

	contents := make([]string, len(observations))
	for i, obs := range observations {
		contents[i] = obs.Content
	}

	return strings.Join(contents, "\n\n---\n\n")
}

func (m *ObservationMemory) observe(ctx context.Context, userID uuid.UUID, channel string, turns []entities.ConversationMessage) error {
	conversationText := formatTurns(turns)

	resp, err := m.interpreter.Interpret(ctx, interfaces.LLMRequest{
		SystemPrompt: observationPrompt,
		UserMessage:  conversationText,
		FreeText:     true,
		MaxTokens:    512,
	})
	if err != nil {
		return fmt.Errorf("agent.observation_memory.interpret: %w", err)
	}

	content := string(resp.RawJSON)
	obs := entities.NewObservation(userID, channel, content, time.Now().UTC())

	if err := m.repo.Insert(ctx, obs); err != nil {
		return fmt.Errorf("agent.observation_memory.insert: %w", err)
	}

	return nil
}

func formatTurns(turns []entities.ConversationMessage) string {
	var sb strings.Builder
	for _, t := range turns {
		fmt.Fprintf(&sb, "[%s] %s\n", t.Role, t.Content)
	}
	return sb.String()
}
