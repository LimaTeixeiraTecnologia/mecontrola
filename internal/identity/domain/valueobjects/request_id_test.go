package valueobjects_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func TestNewRequestID(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantErr bool
		wantStr string
	}{
		{
			name:    "aceita string valida simples",
			input:   "req-abc-123",
			wantErr: false,
			wantStr: "req-abc-123",
		},
		{
			name:    "aceita string com espacos nas bordas e faz trim",
			input:   "  req-trimmed  ",
			wantErr: false,
			wantStr: "req-trimmed",
		},
		{
			name:    "aceita string com exatamente 128 chars",
			input:   strings.Repeat("a", 128),
			wantErr: false,
			wantStr: strings.Repeat("a", 128),
		},
		{
			name:    "rejeita string vazia",
			input:   "",
			wantErr: true,
		},
		{
			name:    "rejeita string apenas espacos",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "rejeita string com 129 chars",
			input:   strings.Repeat("b", 129),
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			rid, err := valueobjects.NewRequestID(sc.input)
			if sc.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, valueobjects.ErrRequestIDInvalid))
				assert.True(t, rid.IsZero())
				return
			}
			assert.NoError(t, err)
			assert.False(t, rid.IsZero())
			assert.Equal(t, sc.wantStr, rid.String())
		})
	}
}

func TestRequestIDIsZero(t *testing.T) {
	var zero valueobjects.RequestID
	assert.True(t, zero.IsZero())
	assert.Equal(t, "", zero.String())
}
