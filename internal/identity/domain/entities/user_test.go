package entities_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UserSuite struct {
	suite.Suite

	validID    entities.UserID
	validEmail valueobjects.Email
	now        time.Time
	number     valueobjects.WhatsAppNumber
}

func TestUserSuite(t *testing.T) {
	suite.Run(t, new(UserSuite))
}

func (s *UserSuite) SetupTest() {
	id, err := entities.NewUserID("550e8400-e29b-41d4-a716-446655440000")
	s.Require().NoError(err)
	s.validID = id

	email, err := valueobjects.NewEmail("user@example.com")
	s.Require().NoError(err)
	s.validEmail = email

	number, err := valueobjects.NewWhatsAppNumber("11987654321")
	s.Require().NoError(err)
	s.number = number

	s.now = time.Now()
}

func (s *UserSuite) TestNewUser_Success() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})

	s.Require().NoError(err)
	s.NotNil(user)
	s.Equal(s.validID, user.ID())
	s.Equal(s.number, user.WhatsAppNumber())
	s.Equal(valueobjects.UserStatusActive, user.Status())
	s.Nil(user.DeletedAt())
	s.False(user.IsDeleted())
}

func (s *UserSuite) TestNewUser_RejectsZeroNumber() {
	_, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    valueobjects.WhatsAppNumber{},
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})

	s.ErrorIs(err, entities.ErrUserRequiresNumber)
}

func (s *UserSuite) TestNewUser_RejectsZeroCreatedAt() {
	_, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: time.Time{},
		UpdatedAt: s.now,
	})

	s.ErrorIs(err, entities.ErrUserRequiresTimestamps)
}

func (s *UserSuite) TestNewUser_RejectsZeroUpdatedAt() {
	_, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: time.Time{},
	})

	s.ErrorIs(err, entities.ErrUserRequiresTimestamps)
}

func (s *UserSuite) TestSoftDelete_Success() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})
	s.Require().NoError(err)

	deleteAt := s.now.Add(time.Hour)
	err = user.SoftDelete(deleteAt)

	s.NoError(err)
	s.True(user.IsDeleted())
	s.NotNil(user.DeletedAt())
	s.Equal(valueobjects.UserStatusDeleted, user.Status())
}

func (s *UserSuite) TestSoftDelete_AlreadyDeleted() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})
	s.Require().NoError(err)

	s.Require().NoError(user.SoftDelete(s.now.Add(time.Hour)))

	err = user.SoftDelete(s.now.Add(2 * time.Hour))

	s.ErrorIs(err, entities.ErrUserAlreadyDeleted)
}

func (s *UserSuite) TestMarkAsAdmin_SetsAdmin() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})
	s.Require().NoError(err)

	s.False(user.IsAdmin())

	updatedAt := s.now.Add(time.Minute)
	user.MarkAsAdmin(updatedAt)

	s.True(user.IsAdmin())
}

func (s *UserSuite) TestRevokeAdmin_ClearsAdmin() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		IsAdmin:   true,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})
	s.Require().NoError(err)

	s.True(user.IsAdmin())

	updatedAt := s.now.Add(time.Minute)
	user.RevokeAdmin(updatedAt)

	s.False(user.IsAdmin())
}

func (s *UserSuite) TestUpdateEmail_SubstitutesEmail() {
	user, err := entities.NewUser(entities.NewUserParams{
		ID:        s.validID,
		Number:    s.number,
		CreatedAt: s.now,
		UpdatedAt: s.now,
	})
	s.Require().NoError(err)

	s.Nil(user.Email())

	updatedAt := s.now.Add(time.Minute)
	user.UpdateEmail(s.validEmail, updatedAt)

	s.NotNil(user.Email())
	s.Equal(s.validEmail.String(), user.Email().String())
}

func (s *UserSuite) TestRehydrateUser_AcceptsArbitraryState() {
	deletedAt := s.now.Add(-time.Hour)

	user := entities.RehydrateUser(entities.RehydrateUserParams{
		ID:        s.validID,
		Number:    s.number,
		IsAdmin:   true,
		Status:    valueobjects.UserStatusDeleted,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
		DeletedAt: &deletedAt,
	})

	s.NotNil(user)
	s.Equal(valueobjects.UserStatusDeleted, user.Status())
	s.True(user.IsAdmin())
	s.NotNil(user.DeletedAt())
}

func (s *UserSuite) TestRehydrateUser_AcceptsZeroTimestamps() {
	user := entities.RehydrateUser(entities.RehydrateUserParams{
		ID:        s.validID,
		Number:    s.number,
		Status:    valueobjects.UserStatusActive,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	})

	s.NotNil(user)
	s.Equal(valueobjects.UserStatusActive, user.Status())
}
