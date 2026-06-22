package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
)

type CountCardsSuite struct {
	suite.Suite
	obs      observability.Observability
	repoMock *ifacemocks.CardRepository
}

func TestCountCards(t *testing.T) {
	suite.Run(t, new(CountCardsSuite))
}

func (s *CountCardsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.repoMock = ifacemocks.NewCardRepository(s.T())
}

func (s *CountCardsSuite) TestExecute_ReturnsTotal() {
	userID := uuid.New()
	s.repoMock.EXPECT().CountActiveByUser(mock.Anything, userID.String()).Return(int64(12), nil).Once()

	sut := NewCountCards(s.repoMock, fake.NewProvider())
	out, err := sut.Execute(context.Background(), input.CountCards{UserID: userID})

	s.Require().NoError(err)
	s.Equal(int64(12), out.Total)
}

func (s *CountCardsSuite) TestExecute_ZeroWhenNoCards() {
	userID := uuid.New()
	s.repoMock.EXPECT().CountActiveByUser(mock.Anything, userID.String()).Return(int64(0), nil).Once()

	sut := NewCountCards(s.repoMock, fake.NewProvider())
	out, err := sut.Execute(context.Background(), input.CountCards{UserID: userID})

	s.Require().NoError(err)
	s.Zero(out.Total)
}

func (s *CountCardsSuite) TestExecute_PropagatesRepositoryError() {
	userID := uuid.New()
	repoErr := errors.New("db down")
	s.repoMock.EXPECT().CountActiveByUser(mock.Anything, userID.String()).Return(int64(0), repoErr).Once()

	sut := NewCountCards(s.repoMock, fake.NewProvider())
	_, err := sut.Execute(context.Background(), input.CountCards{UserID: userID})

	s.Require().Error(err)
	s.ErrorIs(err, repoErr)
}
