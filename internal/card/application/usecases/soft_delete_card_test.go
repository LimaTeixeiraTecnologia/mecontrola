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

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	ifacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/usecases/mocks"
	domain "github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency"
	idemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/idempotency/mocks"
)

type SoftDeleteCardSuite struct {
	suite.Suite
	obs         observability.Observability
	uowMock     *ucmocks.UnitOfWorkVoid
	factoryMock *ifacemocks.RepositoryFactory
	repoMock    *ifacemocks.CardRepository
	idemMock    *idemocks.Storage
}

func TestSoftDeleteCard(t *testing.T) {
	suite.Run(t, new(SoftDeleteCardSuite))
}

func (s *SoftDeleteCardSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.uowMock = ucmocks.NewUnitOfWorkVoid(s.T())
	s.factoryMock = ifacemocks.NewRepositoryFactory(s.T())
	s.repoMock = ifacemocks.NewCardRepository(s.T())
	s.idemMock = idemocks.NewStorage(s.T())
}

func (s *SoftDeleteCardSuite) TestExecute_HappyPath() {
	ctx := context.Background()
	cardID := uuid.New()
	userID := uuid.New()
	in := input.SoftDeleteCard{ID: cardID, UserID: userID}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().SoftDeleteByIDForUser(mock.Anything, cardID.String(), userID.String(), mock.AnythingOfType("time.Time")).Return(nil).Once()

	sut := NewSoftDeleteCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	err := sut.Execute(ctx, in)

	s.Require().NoError(err)
}

func (s *SoftDeleteCardSuite) TestExecute_CardNotFound() {
	ctx := context.Background()
	cardID := uuid.New()
	userID := uuid.New()
	in := input.SoftDeleteCard{ID: cardID, UserID: userID}

	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().SoftDeleteByIDForUser(mock.Anything, cardID.String(), userID.String(), mock.AnythingOfType("time.Time")).Return(domain.ErrCardNotFound).Once()

	sut := NewSoftDeleteCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Require().ErrorIs(err, domain.ErrCardNotFound)
}

func (s *SoftDeleteCardSuite) TestExecute_RINT05_IdempotencyPutRollback() {
	ctx := context.Background()
	cardID := uuid.New()
	userID := uuid.New()
	in := input.SoftDeleteCard{ID: cardID, UserID: userID}

	ic := idempotency.IdempotencyContext{
		Scope:       "card",
		Key:         "key-delete",
		UserID:      userID.String(),
		RequestHash: "hash-delete",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	ctx = idempotency.WithContext(ctx, ic)

	deleteCount := 0
	s.factoryMock.EXPECT().CardRepository(mock.Anything).Return(s.repoMock).Once()
	s.repoMock.EXPECT().SoftDeleteByIDForUser(mock.Anything, cardID.String(), userID.String(), mock.AnythingOfType("time.Time")).
		RunAndReturn(func(ctx context.Context, cID, uID string, now time.Time) error {
			deleteCount++
			return nil
		}).Once()
	idemErr := errors.New("idempotency storage down")
	s.idemMock.EXPECT().Put(mock.Anything, mock.AnythingOfType("idempotency.Record")).Return(idemErr).Once()

	sut := NewSoftDeleteCard(s.uowMock, s.factoryMock, s.idemMock, s.obs)
	err := sut.Execute(ctx, in)

	s.Require().Error(err)
	s.Contains(err.Error(), "idempotency storage down")
	s.Equal(1, deleteCount)
}
