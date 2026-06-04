package postgres

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/suite"
)

type MapperSuite struct {
	suite.Suite
	mapper rowMapper
}

func TestMapperSuite(t *testing.T) {
	suite.Run(t, new(MapperSuite))
}

func (s *MapperSuite) SetupTest() {
	s.mapper = rowMapper{}
}

func (s *MapperSuite) TestIsNoRowsRecognizesSQLAndPgxSentinels() {
	scenarios := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "sql ErrNoRows deve ser reconhecido", err: sql.ErrNoRows, expected: true},
		{name: "pgx ErrNoRows deve ser reconhecido", err: pgx.ErrNoRows, expected: true},
		{name: "erro arbitrario nao deve ser reconhecido", err: errors.New("erro arbitrario"), expected: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expected, isNoRows(scenario.err))
		})
	}
}

func (s *MapperSuite) TestHydrateUser_Valid() {
	now := time.Now().UTC()
	row := userRow{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		WhatsAppNumber: "+5511987654321",
		DisplayName:    sql.NullString{String: "Test User", Valid: true},
		Email:          sql.NullString{String: "test@example.com", Valid: true},
		IsAdmin:        false,
		Status:         "ACTIVE",
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      sql.NullTime{},
	}

	user, err := s.mapper.HydrateUser(row)

	s.NoError(err)
	s.NotNil(user)
	s.Equal("550e8400-e29b-41d4-a716-446655440000", user.ID().String())
	s.Equal("+5511987654321", user.WhatsAppNumber().String())
	s.NotNil(user.Email())
	s.Equal("test@example.com", user.Email().String())
}

func (s *MapperSuite) TestHydrateUser_CorruptedID() {
	now := time.Now().UTC()
	row := userRow{
		ID:             "not-a-valid-uuid",
		WhatsAppNumber: "+5511987654321",
		DisplayName:    sql.NullString{},
		Email:          sql.NullString{},
		IsAdmin:        false,
		Status:         "ACTIVE",
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      sql.NullTime{},
	}

	user, err := s.mapper.HydrateUser(row)

	s.Nil(user)
	s.Error(err)
	s.Contains(err.Error(), "postgres user mapper: id corrompido")
}

func (s *MapperSuite) TestHydrateUser_CorruptedWhatsApp() {
	now := time.Now().UTC()
	row := userRow{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		WhatsAppNumber: "invalid-number",
		DisplayName:    sql.NullString{},
		Email:          sql.NullString{},
		IsAdmin:        false,
		Status:         "ACTIVE",
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      sql.NullTime{},
	}

	user, err := s.mapper.HydrateUser(row)

	s.Nil(user)
	s.Error(err)
	s.Contains(err.Error(), "postgres user mapper: whatsapp corrompido")
}

func (s *MapperSuite) TestHydrateUser_NullEmail() {
	now := time.Now().UTC()
	row := userRow{
		ID:             "550e8400-e29b-41d4-a716-446655440000",
		WhatsAppNumber: "+5511987654321",
		DisplayName:    sql.NullString{},
		Email:          sql.NullString{Valid: false},
		IsAdmin:        false,
		Status:         "ACTIVE",
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      sql.NullTime{},
	}

	user, err := s.mapper.HydrateUser(row)

	s.NoError(err)
	s.NotNil(user)
	s.Nil(user.Email())
}
