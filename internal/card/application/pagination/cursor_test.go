package pagination_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/pagination"
)

type CursorSuite struct {
	suite.Suite
}

func TestCursor(t *testing.T) {
	suite.Run(t, new(CursorSuite))
}

func (s *CursorSuite) TestEncode_NormalizesToUTC() {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	s.Require().NoError(err)
	local := time.Date(2026, 1, 10, 9, 0, 0, 0, loc)

	encoded, err := pagination.Encode(local, "abc")
	s.Require().NoError(err)
	s.NotEmpty(encoded)

	decoded, err := pagination.Decode(encoded)
	s.Require().NoError(err)
	s.True(decoded.CreatedAt.Equal(local.UTC()))
	s.Equal(time.UTC, decoded.CreatedAt.Location())
}

func (s *CursorSuite) TestEncodeDecode_RoundTripPreservesValues() {
	scenarios := []struct {
		name      string
		createdAt time.Time
		id        string
	}{
		{
			name:      "uuid padrão",
			createdAt: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			id:        "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:      "nanoseconds preservados",
			createdAt: time.Date(2026, 6, 1, 0, 0, 0, 123456789, time.UTC),
			id:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			encoded, err := pagination.Encode(sc.createdAt, sc.id)
			s.Require().NoError(err)

			decoded, err := pagination.Decode(encoded)
			s.Require().NoError(err)
			s.Equal(sc.id, decoded.ID)
			s.True(sc.createdAt.UTC().Equal(decoded.CreatedAt))
		})
	}
}

func (s *CursorSuite) TestDecode_InvalidBase64ReturnsSentinel() {
	_, err := pagination.Decode("!!!nope!!!")
	s.Require().Error(err)
	s.True(errors.Is(err, pagination.ErrInvalidCursor))
}

func (s *CursorSuite) TestDecode_InvalidJSONReturnsSentinel() {
	encoded := base64.URLEncoding.EncodeToString([]byte("not json"))
	_, err := pagination.Decode(encoded)
	s.Require().Error(err)
	s.True(errors.Is(err, pagination.ErrInvalidCursor))
}

func (s *CursorSuite) TestDecode_EmptyFieldsReturnSentinel() {
	scenarios := []struct {
		name   string
		cursor pagination.Cursor
	}{
		{name: "tudo zero", cursor: pagination.Cursor{}},
		{name: "id vazio", cursor: pagination.Cursor{CreatedAt: time.Now().UTC()}},
		{name: "created_at zero", cursor: pagination.Cursor{ID: "abc"}},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			raw, err := json.Marshal(sc.cursor)
			s.Require().NoError(err)
			encoded := base64.URLEncoding.EncodeToString(raw)

			_, decErr := pagination.Decode(encoded)
			s.Require().Error(decErr)
			s.True(errors.Is(decErr, pagination.ErrInvalidCursor))
		})
	}
}
