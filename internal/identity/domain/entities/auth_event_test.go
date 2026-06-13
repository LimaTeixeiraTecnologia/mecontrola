package entities_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

func TestNewPrincipalEstablished(t *testing.T) {
	t.Parallel()

	t.Run("deve criar evento com requestID e clientIP", func(t *testing.T) {
		t.Parallel()
		uid := uuid.New()
		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "req-123", "10.0.0.1")
		require.NoError(t, err)
		require.Equal(t, entities.AuthEventKindPrincipalEstablished, ev.Kind())
		require.Equal(t, entities.AuthEventSourceWhatsApp, ev.Source())
		require.Equal(t, "req-123", ev.RequestID())
		require.Equal(t, "10.0.0.1", ev.ClientIP())
		require.NotNil(t, ev.UserID())
		require.Equal(t, uid, *ev.UserID())
		require.Nil(t, ev.Reason())
	})

	t.Run("deve criar evento com campos forenses vazios", func(t *testing.T) {
		t.Parallel()
		uid := uuid.New()
		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "")
		require.NoError(t, err)
		require.Equal(t, "", ev.RequestID())
		require.Equal(t, "", ev.ClientIP())
	})

	t.Run("deve rejeitar userID zero", func(t *testing.T) {
		t.Parallel()
		_, err := entities.NewPrincipalEstablished(uuid.Nil, entities.AuthEventSourceWhatsApp, "", "")
		require.ErrorIs(t, err, entities.ErrPrincipalEstablishedRequiresUserID)
	})
}

func TestNewAuthFailed(t *testing.T) {
	t.Parallel()

	t.Run("deve criar evento gateway_invalid_signature com campos forenses", func(t *testing.T) {
		t.Parallel()
		ev, err := entities.NewAuthFailed(
			entities.AuthEventReasonGatewayInvalidSignature,
			entities.AuthEventSourceWhatsApp,
			nil,
			"req-abc",
			"192.168.1.1",
		)
		require.NoError(t, err)
		require.Equal(t, entities.AuthEventKindFailed, ev.Kind())
		require.NotNil(t, ev.Reason())
		require.Equal(t, entities.AuthEventReasonGatewayInvalidSignature, *ev.Reason())
		require.Equal(t, "req-abc", ev.RequestID())
		require.Equal(t, "192.168.1.1", ev.ClientIP())
	})

	t.Run("deve criar evento gateway_missing_header", func(t *testing.T) {
		t.Parallel()
		ev, err := entities.NewAuthFailed(
			entities.AuthEventReasonGatewayMissingHeader,
			entities.AuthEventSourceWhatsApp,
			nil,
			"",
			"",
		)
		require.NoError(t, err)
		require.NotNil(t, ev.Reason())
		require.Equal(t, entities.AuthEventReasonGatewayMissingHeader, *ev.Reason())
	})

	t.Run("deve criar evento gateway_invalid_timestamp", func(t *testing.T) {
		t.Parallel()
		ev, err := entities.NewAuthFailed(
			entities.AuthEventReasonGatewayInvalidTimestamp,
			entities.AuthEventSourceWhatsApp,
			nil,
			"",
			"",
		)
		require.NoError(t, err)
		require.Equal(t, entities.AuthEventReasonGatewayInvalidTimestamp, *ev.Reason())
	})

	t.Run("deve criar evento gateway_stale_timestamp", func(t *testing.T) {
		t.Parallel()
		ev, err := entities.NewAuthFailed(
			entities.AuthEventReasonGatewayStaleTimestamp,
			entities.AuthEventSourceWhatsApp,
			nil,
			"",
			"",
		)
		require.NoError(t, err)
		require.Equal(t, entities.AuthEventReasonGatewayStaleTimestamp, *ev.Reason())
	})

	t.Run("deve rejeitar reason vazio", func(t *testing.T) {
		t.Parallel()
		_, err := entities.NewAuthFailed("", entities.AuthEventSourceWhatsApp, nil, "", "")
		require.ErrorIs(t, err, entities.ErrAuthFailedRequiresReason)
	})
}

func TestHydrateAuthEvent(t *testing.T) {
	t.Parallel()

	t.Run("deve hidratar evento com requestID e clientIP", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		now := time.Now().UTC()
		reason := entities.AuthEventReasonGatewayInvalidSignature
		ev := entities.HydrateAuthEvent(
			id,
			now,
			nil,
			entities.AuthEventKindFailed,
			entities.AuthEventSourceWhatsApp,
			&reason,
			"req-hydrate",
			"172.16.0.1",
		)
		require.Equal(t, id, ev.ID())
		require.Equal(t, now, ev.OccurredAt())
		require.Equal(t, entities.AuthEventKindFailed, ev.Kind())
		require.Equal(t, entities.AuthEventSourceWhatsApp, ev.Source())
		require.NotNil(t, ev.Reason())
		require.Equal(t, reason, *ev.Reason())
		require.Equal(t, "req-hydrate", ev.RequestID())
		require.Equal(t, "172.16.0.1", ev.ClientIP())
	})

	t.Run("deve hidratar evento com campos forenses vazios", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		ev := entities.HydrateAuthEvent(
			id,
			time.Now().UTC(),
			nil,
			entities.AuthEventKindUnknownUser,
			entities.AuthEventSourceWhatsApp,
			nil,
			"",
			"",
		)
		require.Equal(t, "", ev.RequestID())
		require.Equal(t, "", ev.ClientIP())
		require.Nil(t, ev.Reason())
	})
}

func TestAuthEventGetters(t *testing.T) {
	t.Parallel()

	t.Run("getter RequestID retorna valor correto", func(t *testing.T) {
		t.Parallel()
		uid := uuid.New()
		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "rid-001", "")
		require.NoError(t, err)
		require.Equal(t, "rid-001", ev.RequestID())
	})

	t.Run("getter ClientIP retorna valor correto", func(t *testing.T) {
		t.Parallel()
		uid := uuid.New()
		ev, err := entities.NewPrincipalEstablished(uid, entities.AuthEventSourceWhatsApp, "", "1.2.3.4")
		require.NoError(t, err)
		require.Equal(t, "1.2.3.4", ev.ClientIP())
	})
}
