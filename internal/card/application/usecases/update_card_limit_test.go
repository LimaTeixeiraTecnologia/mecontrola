package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain/valueobjects"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type UpdateCardLimitSuite struct {
	suite.Suite
	obs         observability.Observability
	uowMock     *ucmocks.UnitOfWorkCard
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
	idemMock    *idemocks.Storage
}

func TestUpdateCardLimit(t *testing.T) {
	suite.Run(t, new(UpdateCardLimitSuite))
}

func (s *UpdateCardLimitSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.uowMock = ucmocks.NewUnitOfWorkCard(s.T())
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	s.idemMock = idemocks.NewStorage(s.T())
}

func (s *UpdateCardLimitSuite) existingCard(userID uuid.UUID) entities.Card {
	name, _ := valueobjects.NewCardName("Card")
	nick, _ := valueobjects.NewNickname("nick")
	cycle, _ := valueobjects.NewBillingCycle(10, 17)
	return entities.HydrateCard(uuid.New(), userID, name, nick, cycle, 0, time.Now().UTC(), time.Now().UTC(), nil)
}

func (s *UpdateCardLimitSuite) TestExecute_HappyPath() {
	ctx := context.Background()
	userID := uuid.New()
	existing := s.existingCard(userID)

	in := input.UpdateCardLimit{CardID: existing.ID, UserID: userID, LimitCents: 750000}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateLimitByIDForUser(mock.Anything, mock.MatchedBy(func(c entities.Card) bool {
		return c.LimitCents == 750000
	}), existing.Version).Return(existing.UpdateLimit(mustLimit(750000), time.Now().UTC()), nil).Once()

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	out, err := sut.Execute(ctx, in)

	s.Require().NoError(err)
	s.Equal(int64(750000), out.LimitCents)
}

func (s *UpdateCardLimitSuite) TestExecute_ExpectedVersionMatches_Success() {
	ctx := context.Background()
	userID := uuid.New()
	existing := s.existingCard(userID)
	expected := existing.Version

	in := input.UpdateCardLimit{
		CardID:          existing.ID,
		UserID:          userID,
		LimitCents:      900000,
		ExpectedVersion: &expected,
	}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateLimitByIDForUser(mock.Anything, mock.MatchedBy(func(c entities.Card) bool {
		return c.LimitCents == 900000
	}), existing.Version).Return(existing.UpdateLimit(mustLimit(900000), time.Now().UTC()), nil).Once()

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	out, err := sut.Execute(ctx, in)

	s.Require().NoError(err)
	s.Equal(int64(900000), out.LimitCents)
}

func (s *UpdateCardLimitSuite) TestExecute_ExpectedVersionMismatch_ReturnsConflict() {
	ctx := context.Background()
	userID := uuid.New()
	existing := s.existingCard(userID)
	stale := existing.Version + 99

	in := input.UpdateCardLimit{
		CardID:          existing.ID,
		UserID:          userID,
		LimitCents:      500000,
		ExpectedVersion: &stale,
	}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardLimitConflict)
}

func (s *UpdateCardLimitSuite) TestExecute_RepoConflict_ReturnsConflict() {
	ctx := context.Background()
	userID := uuid.New()
	existing := s.existingCard(userID)

	in := input.UpdateCardLimit{CardID: existing.ID, UserID: userID, LimitCents: 600000}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, existing.ID.String(), userID.String()).Return(existing, nil).Once()
	s.repoMock.EXPECT().UpdateLimitByIDForUser(mock.Anything, mock.Anything, existing.Version).Return(entities.Card{}, domain.ErrCardLimitConflict).Once()

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardLimitConflict)
}

func (s *UpdateCardLimitSuite) TestExecute_NegativeLimit_RejectedBeforeUoW() {
	ctx := context.Background()
	in := input.UpdateCardLimit{CardID: uuid.New(), UserID: uuid.New(), LimitCents: -1}

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, input.ErrCardLimitCentsInvalid)
}

func (s *UpdateCardLimitSuite) TestExecute_TooLarge_RejectedBeforeUoW() {
	ctx := context.Background()
	in := input.UpdateCardLimit{CardID: uuid.New(), UserID: uuid.New(), LimitCents: 100_000_000_00 + 1}

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardLimitTooLarge)
}

func (s *UpdateCardLimitSuite) TestExecute_CardNotFound() {
	ctx := context.Background()
	userID := uuid.New()
	in := input.UpdateCardLimit{CardID: uuid.New(), UserID: userID, LimitCents: 500000}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().GetByIDForUser(mock.Anything, in.CardID.String(), userID.String()).Return(entities.Card{}, domain.ErrCardNotFound).Once()

	sut := NewUpdateCardLimit(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	_, err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func mustLimit(cents int64) valueobjects.CardLimit {
	v, err := valueobjects.NewCardLimit(cents)
	if err != nil {
		panic(err)
	}
	return v
}
