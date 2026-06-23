package entities_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
)

type ThreadSuite struct {
	suite.Suite
}

func TestThreadSuite(t *testing.T) {
	suite.Run(t, new(ThreadSuite))
}

func (s *ThreadSuite) TestNewThreadValidation() {
	cases := []struct {
		name    string
		userID  uuid.UUID
		channel string
		wantErr error
	}{
		{name: "valido", userID: uuid.New(), channel: "whatsapp", wantErr: nil},
		{name: "user nil", userID: uuid.Nil, channel: "whatsapp", wantErr: entities.ErrThreadUserRequired},
		{name: "channel vazio", userID: uuid.New(), channel: "  ", wantErr: entities.ErrThreadChannelRequired},
		{name: "channel longo", userID: uuid.New(), channel: strings.Repeat("a", 33), wantErr: entities.ErrThreadChannelTooLong},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			thread, err := entities.NewThread(tc.userID, tc.channel)
			if tc.wantErr != nil {
				s.Require().ErrorIs(err, tc.wantErr)
				return
			}
			s.Require().NoError(err)
			s.NotEqual(uuid.Nil, thread.ID())
			s.Equal(tc.userID, thread.UserID())
			s.Equal("whatsapp", thread.Channel())
			s.False(thread.CreatedAt().IsZero())
			s.False(thread.UpdatedAt().IsZero())
		})
	}
}

func (s *ThreadSuite) TestNewThreadTrimsChannel() {
	thread, err := entities.NewThread(uuid.New(), "  whatsapp  ")
	s.Require().NoError(err)
	s.Equal("whatsapp", thread.Channel())
}

func (s *ThreadSuite) TestRestoreThread() {
	id := uuid.New()
	userID := uuid.New()
	created := time.Now().UTC().Add(-time.Hour)
	updated := time.Now().UTC()

	thread, err := entities.RestoreThread(entities.ThreadParams{
		ID:        id,
		UserID:    userID,
		Channel:   "whatsapp",
		CreatedAt: created,
		UpdatedAt: updated,
	})
	s.Require().NoError(err)
	s.Equal(id, thread.ID())
	s.Equal(userID, thread.UserID())
	s.WithinDuration(created, thread.CreatedAt(), time.Second)
	s.WithinDuration(updated, thread.UpdatedAt(), time.Second)
}

func (s *ThreadSuite) TestRestoreThreadValidation() {
	cases := []struct {
		name    string
		params  entities.ThreadParams
		wantErr error
	}{
		{name: "id nil", params: entities.ThreadParams{ID: uuid.Nil, UserID: uuid.New(), Channel: "whatsapp"}, wantErr: entities.ErrThreadIDRequired},
		{name: "user nil", params: entities.ThreadParams{ID: uuid.New(), UserID: uuid.Nil, Channel: "whatsapp"}, wantErr: entities.ErrThreadUserRequired},
		{name: "channel vazio", params: entities.ThreadParams{ID: uuid.New(), UserID: uuid.New(), Channel: ""}, wantErr: entities.ErrThreadChannelRequired},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := entities.RestoreThread(tc.params)
			s.Require().ErrorIs(err, tc.wantErr)
		})
	}
}
