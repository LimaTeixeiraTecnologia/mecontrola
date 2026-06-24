package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
)

func TestPayloadJSON_RoundTrip_NewFields(t *testing.T) {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	completedAt := time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC)

	original := onboardingSessionPayloadJSON{
		IncomeCents: 500000,
		RecentTurns: []onboardingTurnJSON{
			{Role: "user", Text: "Quero investir", OccurredAt: now},
			{Role: "assistant", Text: "Entendido!", OccurredAt: now.Add(time.Second)},
		},
		WelcomeSentAt:    &now,
		CompletedAt:      &completedAt,
		ObjectiveProfile: "invest",
	}

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded onboardingSessionPayloadJSON
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, original.IncomeCents, decoded.IncomeCents)
	assert.Equal(t, original.ObjectiveProfile, decoded.ObjectiveProfile)
	require.NotNil(t, decoded.WelcomeSentAt)
	assert.True(t, original.WelcomeSentAt.Equal(*decoded.WelcomeSentAt))
	require.NotNil(t, decoded.CompletedAt)
	assert.True(t, original.CompletedAt.Equal(*decoded.CompletedAt))
	require.Len(t, decoded.RecentTurns, 2)
	assert.Equal(t, "user", decoded.RecentTurns[0].Role)
	assert.Equal(t, "Quero investir", decoded.RecentTurns[0].Text)
	assert.True(t, original.RecentTurns[0].OccurredAt.Equal(decoded.RecentTurns[0].OccurredAt))
}

func TestPayloadJSON_Omitempty_NilFieldsAbsent(t *testing.T) {
	pj := onboardingSessionPayloadJSON{
		IncomeCents: 100000,
	}

	raw, err := json.Marshal(pj)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))

	_, hasRecentTurns := m["recent_turns"]
	_, hasWelcomeSentAt := m["welcome_sent_at"]
	_, hasCompletedAt := m["completed_at"]
	_, hasObjectiveProfile := m["objective_profile"]

	assert.False(t, hasRecentTurns, "recent_turns deve estar ausente quando nil/vazio")
	assert.False(t, hasWelcomeSentAt, "welcome_sent_at deve estar ausente quando nil")
	assert.False(t, hasCompletedAt, "completed_at deve estar ausente quando nil")
	assert.False(t, hasObjectiveProfile, "objective_profile deve estar ausente quando vazio")
}

func TestToFromTurnsJSON_RoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	turns := []entities.OnboardingTurn{
		{Role: "user", Text: "msg1", OccurredAt: now},
		{Role: "assistant", Text: "reply1", OccurredAt: now.Add(time.Second)},
	}

	jsonTurns := toTurnsJSON(turns)
	require.Len(t, jsonTurns, 2)
	assert.Equal(t, "user", jsonTurns[0].Role)
	assert.Equal(t, "msg1", jsonTurns[0].Text)
	assert.True(t, now.Equal(jsonTurns[0].OccurredAt))

	domainTurns := fromTurnsJSON(jsonTurns)
	require.Len(t, domainTurns, 2)
	assert.Equal(t, turns[0].Role, domainTurns[0].Role)
	assert.Equal(t, turns[0].Text, domainTurns[0].Text)
	assert.True(t, turns[0].OccurredAt.Equal(domainTurns[0].OccurredAt))
	assert.Equal(t, turns[1].Role, domainTurns[1].Role)
	assert.Equal(t, turns[1].Text, domainTurns[1].Text)
}

func TestToFromTurnsJSON_Empty(t *testing.T) {
	assert.Empty(t, toTurnsJSON(nil))
	assert.Empty(t, fromTurnsJSON(nil))

	assert.Empty(t, toTurnsJSON([]entities.OnboardingTurn{}))
	assert.Empty(t, fromTurnsJSON([]onboardingTurnJSON{}))
}

func TestPayloadJSON_ObjectiveProfile_Preserved(t *testing.T) {
	pj := onboardingSessionPayloadJSON{
		ObjectiveProfile: "payoff_debt",
	}

	raw, err := json.Marshal(pj)
	require.NoError(t, err)

	var decoded onboardingSessionPayloadJSON
	require.NoError(t, json.Unmarshal(raw, &decoded))

	assert.Equal(t, "payoff_debt", decoded.ObjectiveProfile)
}
