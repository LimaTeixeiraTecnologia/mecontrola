package valueobjects_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewExternalID_WhatsApp(t *testing.T) {
	channel := valueobjects.ChannelWhatsApp()

	cases := []struct {
		name       string
		raw        string
		wantString string
		wantErr    bool
	}{
		{name: "valid br cell", raw: "+5511987654321", wantString: "+5511987654321"},
		{name: "valid br cell without plus", raw: "5511987654321", wantString: "+5511987654321"},
		{name: "valid br cell with formatting", raw: "(11) 98765-4321", wantString: "+5511987654321"},
		{name: "empty", raw: "", wantErr: true},
		{name: "non-br landline", raw: "+551133334444", wantErr: true},
		{name: "letters", raw: "+55abc", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, err := valueobjects.NewExternalID(channel, tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, ext.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantString, ext.String())
			assert.True(t, ext.Channel().Equal(channel))
			assert.False(t, ext.IsZero())
		})
	}
}

func TestNewExternalID_Telegram(t *testing.T) {
	channel := valueobjects.ChannelTelegram()

	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid small", raw: "123"},
		{name: "valid large within int64", raw: "9223372036854775807"},
		{name: "trimmed", raw: "  12345  "},
		{name: "empty", raw: "", wantErr: true},
		{name: "zero prefix", raw: "0123", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "letters", raw: "abc", wantErr: true},
		{name: "overflow int64", raw: "99999999999999999999", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext, err := valueobjects.NewExternalID(channel, tc.raw)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.True(t, ext.Channel().Equal(channel))
			assert.False(t, ext.IsZero())
		})
	}
}

func TestNewExternalID_ChannelZero(t *testing.T) {
	var zero valueobjects.Channel
	_, err := valueobjects.NewExternalID(zero, "anything")
	require.Error(t, err)
	assert.True(t, errors.Is(err, valueobjects.ErrExternalIDChannelRequired))
}

func TestExternalID_Equal(t *testing.T) {
	a, err := valueobjects.NewExternalID(valueobjects.ChannelTelegram(), "123")
	require.NoError(t, err)
	b, err := valueobjects.NewExternalID(valueobjects.ChannelTelegram(), "123")
	require.NoError(t, err)
	c, err := valueobjects.NewExternalID(valueobjects.ChannelTelegram(), "456")
	require.NoError(t, err)

	assert.True(t, a.Equal(b))
	assert.False(t, a.Equal(c))
}

func TestExternalID_Masked_WhatsApp(t *testing.T) {
	ext, err := valueobjects.NewExternalID(valueobjects.ChannelWhatsApp(), "+5511987654321")
	require.NoError(t, err)
	masked := ext.Masked()
	assert.NotEmpty(t, masked)
	assert.NotEqual(t, ext.String(), masked)
}

func TestExternalID_Masked_Telegram(t *testing.T) {
	ext, err := valueobjects.NewExternalID(valueobjects.ChannelTelegram(), "1234567")
	require.NoError(t, err)
	masked := ext.Masked()
	assert.Equal(t, "***4567", masked)
}
