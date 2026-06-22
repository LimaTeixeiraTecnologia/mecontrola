package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	identityapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type LinkChannelToUserSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	factoryMock *interfacesmocks.MockRepositoryFactory
	repoMock    *interfacesmocks.MockUserIdentityRepository
	uowMock     *usecasemocks.UnitOfWorkGeneric[LinkChannelResult]
}

func TestLinkChannelToUser(t *testing.T) {
	suite.Run(t, new(LinkChannelToUserSuite))
}

func (s *LinkChannelToUserSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factoryMock = interfacesmocks.NewMockRepositoryFactory(s.T())
	s.repoMock = interfacesmocks.NewMockUserIdentityRepository(s.T())
	s.uowMock = usecasemocks.NewUnitOfWorkGeneric[LinkChannelResult](s.T())
}

func (s *LinkChannelToUserSuite) buildExisting(userID uuid.UUID, channel valueobjects.Channel, externalID valueobjects.ExternalID) entities.UserIdentity {
	identity, err := entities.NewUserIdentity(uuid.New(), userID, channel, externalID, time.Now().UTC())
	s.Require().NoError(err)
	return identity
}

func (s *LinkChannelToUserSuite) TestExecute_FreshLink_Inserts() {
	channel := valueobjects.ChannelTelegram()
	externalID, err := valueobjects.NewExternalID(channel, "12345")
	s.Require().NoError(err)

	s.factoryMock.EXPECT().UserIdentityRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(entities.UserIdentity{}, false, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()

	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	res, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     uuid.New(),
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().NoError(execErr)
	s.False(res.AlreadyLinked)
	s.NotEqual(uuid.Nil, res.IdentityID)
}

func (s *LinkChannelToUserSuite) TestExecute_AlreadyLinkedSameUser_Idempotent() {
	channel := valueobjects.ChannelTelegram()
	externalID, err := valueobjects.NewExternalID(channel, "12345")
	s.Require().NoError(err)

	userID := uuid.New()
	existing := s.buildExisting(userID, channel, externalID)

	s.factoryMock.EXPECT().UserIdentityRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(existing, true, nil).Once()

	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	res, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     userID,
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().NoError(execErr)
	s.True(res.AlreadyLinked, "mesmo userID ja vinculado deve ser idempotente")
	s.Equal(existing.ID(), res.IdentityID)
}

func (s *LinkChannelToUserSuite) TestExecute_AlreadyLinkedOtherUser_ReturnsSecurityError() {
	channel := valueobjects.ChannelTelegram()
	externalID, err := valueobjects.NewExternalID(channel, "12345")
	s.Require().NoError(err)

	otherUserID := uuid.New()
	existing := s.buildExisting(otherUserID, channel, externalID)

	s.factoryMock.EXPECT().UserIdentityRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(existing, true, nil).Once()

	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	_, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     uuid.New(),
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().Error(execErr)
	s.True(errors.Is(execErr, identityapp.ErrUserIdentityAlreadyLinked))
}

func (s *LinkChannelToUserSuite) TestExecute_RaceConflict_SameUser_ReturnsIdempotent() {
	channel := valueobjects.ChannelTelegram()
	externalID, err := valueobjects.NewExternalID(channel, "12345")
	s.Require().NoError(err)

	userID := uuid.New()
	winner := s.buildExisting(userID, channel, externalID)

	s.factoryMock.EXPECT().UserIdentityRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(entities.UserIdentity{}, false, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.Anything).Return(identityapp.ErrUserIdentityAlreadyLinked).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(winner, true, nil).Once()

	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	res, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     userID,
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().NoError(execErr, "race resolvido com mesmo userID deve ser idempotente")
	s.True(res.AlreadyLinked)
}

func (s *LinkChannelToUserSuite) TestExecute_RaceConflict_OtherUser_ReturnsSecurityError() {
	channel := valueobjects.ChannelTelegram()
	externalID, err := valueobjects.NewExternalID(channel, "12345")
	s.Require().NoError(err)

	otherUserID := uuid.New()
	winner := s.buildExisting(otherUserID, channel, externalID)

	s.factoryMock.EXPECT().UserIdentityRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(entities.UserIdentity{}, false, nil).Once()
	s.repoMock.EXPECT().Insert(mock.Anything, mock.Anything).Return(identityapp.ErrUserIdentityAlreadyLinked).Once()
	s.repoMock.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(winner, true, nil).Once()

	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	_, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     uuid.New(),
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().Error(execErr)
	s.True(errors.Is(execErr, identityapp.ErrUserIdentityAlreadyLinked))
}

func (s *LinkChannelToUserSuite) TestExecute_InvalidChannel_Rejects() {
	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	_, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     uuid.New(),
		Channel:    "sms",
		ExternalID: "12345",
	})

	s.Require().Error(execErr)
}

func (s *LinkChannelToUserSuite) TestExecute_NilUserID_Rejects() {
	sut := NewLinkChannelToUser(s.uowMock, s.factoryMock, s.obs)
	_, execErr := sut.Execute(s.ctx, input.LinkChannelToUser{
		UserID:     uuid.Nil,
		Channel:    "telegram",
		ExternalID: "12345",
	})

	s.Require().Error(execErr)
	s.True(errors.Is(execErr, input.ErrUserIDRequired))
}
