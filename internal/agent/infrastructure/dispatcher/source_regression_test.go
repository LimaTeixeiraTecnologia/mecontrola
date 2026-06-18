package dispatcher_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/infrastructure/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	transactionsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/output"
)

func TestTransactionsAdapter_Create_PreservesTelegramSourceFromCtx(t *testing.T) {
	create := &stubCreateTransaction{resp: transactionsoutput.Transaction{Direction: "expense"}}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{}, nil, nil, nil)

	userID := uuid.New()
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: userID,
		Source: auth.SourceTelegram,
	})

	payload := json.RawMessage(`{"amount":10,"type":"expense","category_id":"` + uuid.New().String() + `"}`)
	_, err := sut.Create(ctx, userID, payload)
	require.NoError(t, err)

	assert.Equal(t, auth.SourceTelegram, create.gotPrincipal.Source,
		"Source deveria preservar Telegram quando ctx já tem Principal injetado pelo HandleInboundMessage")
}

func TestTransactionsAdapter_Create_FallsBackToWhatsAppWhenCtxEmpty(t *testing.T) {
	create := &stubCreateTransaction{resp: transactionsoutput.Transaction{Direction: "expense"}}
	sut := dispatcher.NewTransactionsAdapterFull(&stubListTransactionsForCreate{}, create, &stubDeleteTransaction{}, &stubGetTransaction{}, nil, nil, nil)

	payload := json.RawMessage(`{"amount":10,"type":"expense","category_id":"` + uuid.New().String() + `"}`)
	_, err := sut.Create(context.Background(), uuid.New(), payload)
	require.NoError(t, err)

	assert.Equal(t, auth.SourceWhatsApp, create.gotPrincipal.Source,
		"Fallback SourceWhatsApp quando ctx não tem Principal (backward compat)")
}

func TestAuthSourceFromChannel(t *testing.T) {
	cases := []struct {
		channel string
		want    auth.PrincipalSource
		wantErr bool
	}{
		{channel: "whatsapp", want: auth.SourceWhatsApp},
		{channel: "telegram", want: auth.SourceTelegram},
		{channel: "unknown", want: auth.SourceWhatsApp, wantErr: true},
		{channel: "", want: auth.SourceWhatsApp, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			source, err := auth.SourceFromChannel(tc.channel)
			assert.Equal(t, tc.want, source)
			if tc.wantErr {
				assert.Error(t, err, "unknown channel deve sinalizar erro para evitar drift silencioso")
				assert.ErrorIs(t, err, auth.ErrSourceFromChannelUnknown)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
