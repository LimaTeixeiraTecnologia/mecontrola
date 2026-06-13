package valueobjects_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewGatewayTimestamp(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	window := 60 * time.Second

	scenarios := []struct {
		name      string
		raw       string
		now       time.Time
		window    time.Duration
		wantErr   bool
		wantErrIs error
	}{
		{
			name:    "aceita timestamp exato igual a now",
			raw:     fmt.Sprintf("%d", now.Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
		{
			name:    "aceita timestamp 30s no passado",
			raw:     fmt.Sprintf("%d", now.Add(-30*time.Second).Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
		{
			name:    "aceita timestamp 30s no futuro",
			raw:     fmt.Sprintf("%d", now.Add(30*time.Second).Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
		{
			name:    "aceita timestamp exatamente na borda inferior",
			raw:     fmt.Sprintf("%d", now.Add(-60*time.Second).Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
		{
			name:    "aceita timestamp exatamente na borda superior",
			raw:     fmt.Sprintf("%d", now.Add(60*time.Second).Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
		{
			name:      "rejeita timestamp 61s no passado",
			raw:       fmt.Sprintf("%d", now.Add(-61*time.Second).Unix()),
			now:       now,
			window:    window,
			wantErr:   true,
			wantErrIs: valueobjects.ErrGatewayTimestampStale,
		},
		{
			name:      "rejeita timestamp 61s no futuro",
			raw:       fmt.Sprintf("%d", now.Add(61*time.Second).Unix()),
			now:       now,
			window:    window,
			wantErr:   true,
			wantErrIs: valueobjects.ErrGatewayTimestampStale,
		},
		{
			name:      "rejeita formato nao numerico",
			raw:       "abc",
			now:       now,
			window:    window,
			wantErr:   true,
			wantErrIs: valueobjects.ErrGatewayTimestampInvalid,
		},
		{
			name:      "rejeita string vazia",
			raw:       "",
			now:       now,
			window:    window,
			wantErr:   true,
			wantErrIs: valueobjects.ErrGatewayTimestampInvalid,
		},
		{
			name:      "rejeita float",
			raw:       "1234567890.5",
			now:       now,
			window:    window,
			wantErr:   true,
			wantErrIs: valueobjects.ErrGatewayTimestampInvalid,
		},
		{
			name:    "aceita timestamp negativo valido dentro da janela",
			raw:     fmt.Sprintf("%d", now.Unix()),
			now:     now,
			window:  window,
			wantErr: false,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			ts, err := valueobjects.NewGatewayTimestamp(sc.raw, sc.now, sc.window)
			if sc.wantErr {
				assert.Error(t, err)
				if sc.wantErrIs != nil {
					assert.True(t, errors.Is(err, sc.wantErrIs), "expected errors.Is(%v)", sc.wantErrIs)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, sc.raw, ts.Raw())
			assert.False(t, ts.Time().IsZero())
		})
	}
}

func TestGatewayTimestampAccessors(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	raw := fmt.Sprintf("%d", now.Unix())

	ts, err := valueobjects.NewGatewayTimestamp(raw, now, 60*time.Second)
	assert.NoError(t, err)
	assert.Equal(t, raw, ts.Raw())
	assert.Equal(t, now.Unix(), ts.Time().Unix())
}
