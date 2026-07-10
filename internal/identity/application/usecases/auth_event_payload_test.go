package usecases

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

func TestNewAuthEventOutbox_PrincipalEstablished(t *testing.T) {
	t.Parallel()

	eventID := uuid.New().String()
	userID := uuid.New().String()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	ev, err := newAuthEventOutbox(eventID, userID, "principal_established", "whatsapp", "", "", "", "", now)
	require.NoError(t, err)

	require.Equal(t, eventID, ev.ID)
	require.Equal(t, "auth.principal_established", ev.Type)
	require.Equal(t, "auth_event", ev.AggregateType)
	require.Equal(t, userID, ev.AggregateID)
	require.Equal(t, userID, ev.AggregateUserID)
	require.Equal(t, now, ev.OccurredAt)

	var decoded authEventPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &decoded))
	require.Equal(t, eventID, decoded.EventID)
	require.NotNil(t, decoded.UserID)
	require.Equal(t, userID, *decoded.UserID)
	require.Equal(t, "principal_established", decoded.Kind)
	require.Equal(t, "whatsapp", decoded.Source)
	require.Nil(t, decoded.Reason)
	require.Nil(t, decoded.ResolvePath)
}

func TestNewAuthEventOutbox_WithResolvePath(t *testing.T) {
	t.Parallel()

	eventID := uuid.New().String()
	userID := uuid.New().String()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	ev, err := newAuthEventOutbox(eventID, userID, "principal_established", "whatsapp", "", "legacy", "", "", now)
	require.NoError(t, err)

	var decoded authEventPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &decoded))
	require.NotNil(t, decoded.ResolvePath)
	require.Equal(t, "legacy", *decoded.ResolvePath)
}

func TestNewAuthEventOutbox_UnknownUser(t *testing.T) {
	t.Parallel()

	eventID := uuid.New().String()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	ev, err := newAuthEventOutbox(eventID, "", "unknown_user", "whatsapp", "", "", "", "", now)
	require.NoError(t, err)

	require.Equal(t, eventID, ev.AggregateID)
	require.Equal(t, "auth.unknown_user", ev.Type)
	require.Empty(t, ev.AggregateUserID)

	var decoded authEventPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &decoded))
	require.Nil(t, decoded.UserID)
	require.Nil(t, decoded.Reason)
}

func TestNewAuthEventOutbox_WithReason(t *testing.T) {
	t.Parallel()

	eventID := uuid.New().String()
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	ev, err := newAuthEventOutbox(eventID, "", "failed", "whatsapp", "invalid_signature", "", "", "", now)
	require.NoError(t, err)

	var decoded authEventPayload
	require.NoError(t, json.Unmarshal(ev.Payload, &decoded))
	require.NotNil(t, decoded.Reason)
	require.Equal(t, "invalid_signature", *decoded.Reason)
}

func TestParseAuthEvent_Valid(t *testing.T) {
	t.Parallel()

	eventID := uuid.New()
	userID := uuid.New()
	occurredAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	uidStr := userID.String()
	reasonStr := "invalid_signature"
	raw, err := json.Marshal(authEventPayload{
		EventID:    eventID.String(),
		UserID:     &uidStr,
		Kind:       "failed",
		Source:     "whatsapp",
		Reason:     &reasonStr,
		OccurredAt: occurredAt.Format(time.RFC3339),
	})
	require.NoError(t, err)

	ev, err := parseAuthEvent(raw)
	require.NoError(t, err)
	require.Equal(t, eventID, ev.ID())
	require.NotNil(t, ev.UserID())
	require.Equal(t, userID, *ev.UserID())
	require.Equal(t, entities.AuthEventKindFailed, ev.Kind())
	require.Equal(t, entities.AuthEventSourceWhatsApp, ev.Source())
	require.NotNil(t, ev.Reason())
	require.Equal(t, entities.AuthEventReason("invalid_signature"), *ev.Reason())
}

func TestParseAuthEvent_WithResolvePath(t *testing.T) {
	t.Parallel()

	eventID := uuid.New()
	userID := uuid.New()
	occurredAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	uidStr := userID.String()
	pathStr := "legacy"
	raw, err := json.Marshal(authEventPayload{
		EventID:     eventID.String(),
		UserID:      &uidStr,
		Kind:        "principal_established",
		Source:      "whatsapp",
		ResolvePath: &pathStr,
		OccurredAt:  occurredAt.Format(time.RFC3339),
	})
	require.NoError(t, err)

	ev, err := parseAuthEvent(raw)
	require.NoError(t, err)
	require.NotNil(t, ev.ResolvePath())
	require.Equal(t, domain.AuthResolvePathLegacy, *ev.ResolvePath())
}

func TestParseAuthEvent_InvalidResolvePath(t *testing.T) {
	t.Parallel()

	eventID := uuid.New()
	occurredAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	bad := "unknown"
	raw, err := json.Marshal(authEventPayload{
		EventID:     eventID.String(),
		Kind:        "principal_established",
		Source:      "whatsapp",
		ResolvePath: &bad,
		OccurredAt:  occurredAt.Format(time.RFC3339),
	})
	require.NoError(t, err)

	_, err = parseAuthEvent(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse resolve_path")
}

func TestParseAuthEvent_ErrorPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{
			name:    "payload invalido",
			raw:     []byte("not-json"),
			wantSub: "decode",
		},
		{
			name: "event_id invalido",
			raw: mustMarshal(t, authEventPayload{
				EventID:    "not-a-uuid",
				Kind:       "principal_established",
				Source:     "whatsapp",
				OccurredAt: time.Now().UTC().Format(time.RFC3339),
			}),
			wantSub: "parse event_id",
		},
		{
			name: "occurred_at invalido",
			raw: mustMarshal(t, authEventPayload{
				EventID:    uuid.New().String(),
				Kind:       "principal_established",
				Source:     "whatsapp",
				OccurredAt: "not-a-time",
			}),
			wantSub: "parse occurred_at",
		},
		{
			name: "user_id invalido",
			raw: mustMarshal(t, func() authEventPayload {
				bad := "not-a-uuid"
				return authEventPayload{
					EventID:    uuid.New().String(),
					UserID:     &bad,
					Kind:       "principal_established",
					Source:     "whatsapp",
					OccurredAt: time.Now().UTC().Format(time.RFC3339),
				}
			}()),
			wantSub: "parse user_id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseAuthEvent(tc.raw)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantSub)
		})
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
