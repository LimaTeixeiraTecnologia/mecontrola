package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

func TestNewUserID_AcceptsV4(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "canonical lowercase",
			input: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:  "canonical uppercase",
			input: "550E8400-E29B-41D4-A716-446655440000",
		},
		{
			name:  "another v4",
			input: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := entities.NewUserID(tc.input)
			require.NoError(t, err)
			assert.NotEmpty(t, id.String())
		})
	}
}

func TestNewUserID_RejectsV3(t *testing.T) {
	// UUID v3 (version byte = 3)
	v3 := "6ba7b810-9dad-31d1-80b4-00c04fd430c8"
	_, err := entities.NewUserID(v3)
	assert.ErrorIs(t, err, entities.ErrInvalidUserID)
}

func TestNewUserID_RejectsV5(t *testing.T) {
	// UUID v5 (version byte = 5)
	v5 := "886313e1-3b8a-5372-9b90-0c9aee199e5d"
	_, err := entities.NewUserID(v5)
	assert.ErrorIs(t, err, entities.ErrInvalidUserID)
}

func TestNewUserID_RejectsEmpty(t *testing.T) {
	_, err := entities.NewUserID("")
	assert.ErrorIs(t, err, entities.ErrInvalidUserID)
}

func TestNewUserID_RejectsInvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "not a uuid", input: "not-a-uuid"},
		{name: "too short", input: "550e8400-e29b-41d4"},
		{name: "random string", input: "hello world"},
		{name: "all zeros", input: "00000000-0000-0000-0000-000000000000"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entities.NewUserID(tc.input)
			assert.ErrorIs(t, err, entities.ErrInvalidUserID)
		})
	}
}
