package services

import (
	"encoding/json"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type TurnHistory struct{}

func (TurnHistory) Deserialize(raw []byte) ([]entities.ConversationMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var turns []entities.ConversationMessage
	if err := json.Unmarshal(raw, &turns); err != nil {
		return nil, err
	}
	return turns, nil
}

func (TurnHistory) Serialize(turns []entities.ConversationMessage) ([]byte, error) {
	if len(turns) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(turns)
}

func (TurnHistory) Append(turns []entities.ConversationMessage, userMsg, assistantMsg string, now time.Time, maxPairs int) []entities.ConversationMessage {
	turns = append(turns,
		entities.ConversationMessage{Role: "user", Content: userMsg, At: now},
		entities.ConversationMessage{Role: "assistant", Content: assistantMsg, At: now},
	)
	if maxPairs > 0 && len(turns) > maxPairs*2 {
		turns = turns[len(turns)-maxPairs*2:]
	}
	return turns
}

func (TurnHistory) ToLLMMessages(turns []entities.ConversationMessage) []interfaces.ConversationMessage {
	msgs := make([]interfaces.ConversationMessage, 0, len(turns))
	for _, t := range turns {
		msgs = append(msgs, interfaces.ConversationMessage{Role: t.Role, Content: t.Content})
	}
	return msgs
}
