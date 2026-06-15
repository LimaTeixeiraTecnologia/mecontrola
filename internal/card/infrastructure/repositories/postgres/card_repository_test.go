package postgres

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
	carddomain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
)

func TestEncodeDecode_Cursor(t *testing.T) {
	scenarios := []struct {
		name      string
		createdAt time.Time
		id        string
		wantErr   bool
	}{
		{
			name:      "round-trip cursor válido",
			createdAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			id:        "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "cursor com nanoseconds preservados",
			createdAt: time.Date(2024, 6, 1, 0, 0, 0, 123456789, time.UTC),
			id:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			encoded, err := pagination.C.Encode(sc.createdAt, sc.id)
			require.NoError(t, err)
			assert.NotEmpty(t, encoded)

			decoded, err := pagination.C.Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, sc.id, decoded.ID)
			assert.True(t, sc.createdAt.UTC().Equal(decoded.CreatedAt.UTC()))
		})
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	scenarios := []struct {
		name    string
		cursor  string
		wantErr bool
	}{
		{
			name:    "string inválida base64",
			cursor:  "!!!invalid!!!",
			wantErr: true,
		},
		{
			name:    "base64 válido mas json inválido",
			cursor:  base64.URLEncoding.EncodeToString([]byte("not json")),
			wantErr: true,
		},
		{
			name: "json válido mas campos vazios",
			cursor: func() string {
				b, _ := json.Marshal(pagination.Cursor{})
				return base64.URLEncoding.EncodeToString(b)
			}(),
			wantErr: true,
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			_, err := pagination.C.Decode(sc.cursor)
			if sc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrorMapping_NicknameConflict(t *testing.T) {
	wrapped := fmt.Errorf("card.repository.pg: %w", carddomain.ErrNicknameConflict)
	assert.True(t, errors.Is(wrapped, carddomain.ErrNicknameConflict))
}

func TestErrorMapping_CardNotFound(t *testing.T) {
	wrapped := fmt.Errorf("card.repository.pg: %w", carddomain.ErrCardNotFound)
	assert.True(t, errors.Is(wrapped, carddomain.ErrCardNotFound))
}
