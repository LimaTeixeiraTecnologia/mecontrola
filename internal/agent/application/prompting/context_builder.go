package prompting

import (
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

const contextSectionSeparator = "\n\n---\n\n"

type ContextInput struct {
	History            []entities.ConversationMessage
	WorkingMemory      string
	ObservationContext string
	JourneyHint        string
	MaxHistoryPairs    int
}

type ContextMessage struct {
	Role    string
	Content string
}

type ContextResult struct {
	SystemPrompt string
	History      []ContextMessage
}

func BuildContext(in ContextInput) (ContextResult, error) {
	workingMemory := strings.TrimSpace(in.WorkingMemory)
	observation := strings.TrimSpace(in.ObservationContext)
	journey := strings.TrimSpace(in.JourneyHint)

	persona, err := RenderPersonaSystem(PersonaSystemData{
		JourneyHint:        journey,
		WorkingMemory:      workingMemory,
		ObservationContext: observation,
	})
	if err != nil {
		return ContextResult{}, fmt.Errorf("agent.application.prompting.context_builder: persona: %w", err)
	}
	budgets, err := RenderBudgetsPersona(BudgetsPersonaData{JourneyHint: journey})
	if err != nil {
		return ContextResult{}, fmt.Errorf("agent.application.prompting.context_builder: budgets: %w", err)
	}
	memory, err := RenderWorkingMemorySystem(WorkingMemorySystemData{WorkingMemory: workingMemory})
	if err != nil {
		return ContextResult{}, fmt.Errorf("agent.application.prompting.context_builder: working_memory: %w", err)
	}

	systemPrompt := strings.Join([]string{persona, budgets, memory}, contextSectionSeparator)

	return ContextResult{
		SystemPrompt: systemPrompt,
		History:      buildHistory(in.History, in.MaxHistoryPairs),
	}, nil
}

func buildHistory(turns []entities.ConversationMessage, maxPairs int) []ContextMessage {
	if len(turns) == 0 {
		return nil
	}
	trimmed := turns
	if maxPairs > 0 && len(trimmed) > maxPairs*2 {
		trimmed = trimmed[len(trimmed)-maxPairs*2:]
	}
	out := make([]ContextMessage, 0, len(trimmed))
	for _, t := range trimmed {
		content := strings.TrimSpace(t.Content)
		if content == "" {
			continue
		}
		out = append(out, ContextMessage{Role: t.Role, Content: content})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
