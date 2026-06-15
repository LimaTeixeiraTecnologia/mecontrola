package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewChannel(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantErr    error
		wantString string
		wantWA     bool
		wantTG     bool
	}{
		{name: "whatsapp lower", raw: "whatsapp", wantString: "whatsapp", wantWA: true},
		{name: "whatsapp mixed case trimmed", raw: " WhatsApp ", wantString: "whatsapp", wantWA: true},
		{name: "telegram lower", raw: "telegram", wantString: "telegram", wantTG: true},
		{name: "empty", raw: "", wantErr: valueobjects.ErrChannelEmpty},
		{name: "blank", raw: "   ", wantErr: valueobjects.ErrChannelEmpty},
		{name: "unknown", raw: "sms", wantErr: valueobjects.ErrChannelUnknown},
		{name: "instagram", raw: "instagram", wantErr: valueobjects.ErrChannelUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			channel, err := valueobjects.NewChannel(tc.raw)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr), "expected error %v, got %v", tc.wantErr, err)
				assert.True(t, channel.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantString, channel.String())
			assert.Equal(t, tc.wantWA, channel.IsWhatsApp())
			assert.Equal(t, tc.wantTG, channel.IsTelegram())
			assert.False(t, channel.IsZero())
		})
	}
}

func TestChannel_FactoryHelpers(t *testing.T) {
	wa := valueobjects.ChannelWhatsApp()
	tg := valueobjects.ChannelTelegram()
	assert.Equal(t, "whatsapp", wa.String())
	assert.Equal(t, "telegram", tg.String())
	assert.True(t, wa.Equal(valueobjects.ChannelWhatsApp()))
	assert.False(t, wa.Equal(tg))
}

func TestChannel_ZeroValue(t *testing.T) {
	var zero valueobjects.Channel
	assert.True(t, zero.IsZero())
	assert.False(t, zero.IsWhatsApp())
	assert.False(t, zero.IsTelegram())
}
