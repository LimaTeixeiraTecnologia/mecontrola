package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

func validWA(t *testing.T) valueobjects.WhatsAppNumber {
	t.Helper()
	wa, err := valueobjects.NewWhatsAppNumber("+5511987654321")
	require.NoError(t, err)
	return wa
}

func validEmail(t *testing.T) valueobjects.Email {
	t.Helper()
	em, err := valueobjects.NewEmail("test@example.com")
	require.NoError(t, err)
	return em
}

type UserSuite struct {
	suite.Suite
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserSuite))
}

func (s *UserSuite) TestNew_SetsActiveStatusAndNonEmptyID() {
	wa := validWA(s.T())
	u := entities.New(wa)

	s.Equal(entities.StatusActive, u.Status())
	s.NotEmpty(u.ID())
	s.True(u.DeletedAt().IsZero())
	s.True(u.CreatedAt().Equal(u.CreatedAt().UTC()))
}

func (s *UserSuite) TestNew_WithOptions() {
	wa := validWA(s.T())
	em := validEmail(s.T())

	u := entities.New(wa, entities.WithEmail(em), entities.WithDisplayName("Alice"))

	s.Equal(em.String(), u.Email().String())
	s.Equal("Alice", u.DisplayName())
}

func (s *UserSuite) TestNew_UniqueIDs() {
	wa := validWA(s.T())
	u1 := entities.New(wa)
	u2 := entities.New(wa)

	s.NotEqual(u1.ID(), u2.ID())
}

func (s *UserSuite) TestMarkDeleted_SetsStatusAndDeletedAt() {
	wa := validWA(s.T())
	u := entities.New(wa)
	now := time.Now().UTC()

	u.MarkDeleted(now)

	s.Equal(entities.StatusDeleted, u.Status())
	s.Equal(now, u.DeletedAt())
}

func (s *UserSuite) TestMarkDeleted_AlwaysSetsBothFieldsAtomically() {
	wa := validWA(s.T())
	u := entities.New(wa)
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	u.MarkDeleted(now)

	s.Equal(entities.StatusDeleted, u.Status())
	s.Equal(now, u.DeletedAt())
}

func (s *UserSuite) TestReanimate_ClearsPIIAndReactivates() {
	wa := validWA(s.T())
	em := validEmail(s.T())
	u := entities.New(wa, entities.WithEmail(em), entities.WithDisplayName("Alice"))
	deleted := time.Now().UTC().Add(-10 * 24 * time.Hour)
	u.MarkDeleted(deleted)

	now := time.Now().UTC()
	u.Reanimate(now)

	s.Equal(entities.StatusActive, u.Status())
	s.True(u.DeletedAt().IsZero())
	s.Empty(u.Email().String())
	s.Empty(u.DisplayName())
	s.Equal(now, u.UpdatedAt())
}

func (s *UserSuite) TestCanReanimate_BorderCases() {
	wa := validWA(s.T())

	cases := []struct {
		name      string
		elapsed   time.Duration
		deletedAt func() time.Time
		want      bool
	}{
		{
			name:    "exactly 30d",
			elapsed: entities.ReanimationWindow,
			want:    true,
		},
		{
			name:    "30d minus 1ns",
			elapsed: entities.ReanimationWindow - time.Nanosecond,
			want:    true,
		},
		{
			name:    "30d plus 1ns",
			elapsed: entities.ReanimationWindow + time.Nanosecond,
			want:    false,
		},
		{
			name:      "deletedAt zero",
			deletedAt: func() time.Time { return time.Time{} },
			want:      false,
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			u := entities.New(wa)
			base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			u.MarkDeleted(base)

			var now time.Time
			if tc.deletedAt != nil {
				u2 := entities.Hydrate(u.ID(), wa.String(), "", "", string(entities.StatusDeleted),
					u.CreatedAt(), u.UpdatedAt(), tc.deletedAt())
				now = base.Add(entities.ReanimationWindow)
				s.Equal(tc.want, u2.CanReanimate(now))
				return
			}
			now = base.Add(tc.elapsed)
			s.Equal(tc.want, u.CanReanimate(now))
		})
	}
}

func (s *UserSuite) TestSetDisplayNameIfEmpty_PopulatesWhenEmpty() {
	wa := validWA(s.T())
	u := entities.New(wa)

	s.Empty(u.DisplayName())
	u.SetDisplayNameIfEmpty("Alice")
	s.Equal("Alice", u.DisplayName())
}

func (s *UserSuite) TestSetDisplayNameIfEmpty_PreservesWhenPopulated() {
	wa := validWA(s.T())
	u := entities.New(wa, entities.WithDisplayName("Original"))

	u.SetDisplayNameIfEmpty("New Name")

	s.Equal("Original", u.DisplayName())
}

func (s *UserSuite) TestSetEmailIfEmpty_PopulatesWhenEmpty() {
	wa := validWA(s.T())
	u := entities.New(wa)
	em := validEmail(s.T())

	u.SetEmailIfEmpty(em)

	s.Equal(em.String(), u.Email().String())
}

func (s *UserSuite) TestSetEmailIfEmpty_PreservesWhenPopulated() {
	wa := validWA(s.T())
	em := validEmail(s.T())
	u := entities.New(wa, entities.WithEmail(em))

	other, err := valueobjects.NewEmail("other@example.com")
	require.NoError(s.T(), err)

	u.SetEmailIfEmpty(other)

	s.Equal(em.String(), u.Email().String())
}

func (s *UserSuite) TestHydrate_ReconstructsWithoutGeneratingID() {
	wa := validWA(s.T())
	em := validEmail(s.T())
	id := "fixed-uuid-1234"
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	u := entities.Hydrate(id, wa.String(), em.String(), "Alice", string(entities.StatusActive), now, now, time.Time{})

	s.Equal(id, u.ID())
	s.Equal("Alice", u.DisplayName())
	s.Equal(entities.StatusActive, u.Status())
	s.True(u.DeletedAt().IsZero())
}

func (s *UserSuite) TestNew_IDIsUUIDFormat() {
	wa := validWA(s.T())
	u := entities.New(wa)

	id := u.ID()
	s.Len(id, 36)
	assert.Regexp(s.T(), `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}
