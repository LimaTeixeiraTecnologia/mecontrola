package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type LoggingRegressionSuite struct {
	suite.Suite
	ctx       context.Context
	fakeO11y  *fake.Provider
	factory   *interfacesmocks.MockRepositoryFactory
	repo      *interfacesmocks.MockUserRepository
	idRepo    *interfacesmocks.MockUserIdentityRepository
	publisher *outboxmocks.Publisher
	uow       *usecasemocks.UnitOfWorkGeneric[EstablishResult]
}

func TestLoggingRegression(t *testing.T) {
	suite.Run(t, new(LoggingRegressionSuite))
}

func (s *LoggingRegressionSuite) SetupTest() {
	s.ctx = context.Background()
	s.fakeO11y = fake.NewProvider()
	s.factory = interfacesmocks.NewMockRepositoryFactory(s.T())
	s.repo = interfacesmocks.NewMockUserRepository(s.T())
	s.idRepo = interfacesmocks.NewMockUserIdentityRepository(s.T())
	s.publisher = outboxmocks.NewPublisher(s.T())
	s.uow = usecasemocks.NewUnitOfWorkGeneric[EstablishResult](s.T())
}

func (s *LoggingRegressionSuite) TestNoSensitiveFieldsOnSuccess() {
	const waRaw = "+5511987654321"
	wa, err := valueobjects.NewWhatsAppNumber(waRaw)
	s.Require().NoError(err)

	userID := uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001")
	user, err := entities.Hydrate(
		userID.String(),
		wa.String(),
		"", "", "ACTIVE",
		time.Time{}, time.Time{}, time.Time{},
	)
	s.Require().NoError(err)

	channelWA := valueobjects.ChannelWhatsApp()
	externalIDWA, errExt := valueobjects.NewExternalID(channelWA, wa.String())
	s.Require().NoError(errExt)

	s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
	s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
	s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
	s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).Return(user, true, nil).Once()
	s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
	s.idRepo.EXPECT().InsertIfAbsent(mock.Anything, mock.AnythingOfType("entities.UserIdentity")).Return(true, nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	sut := NewEstablishPrincipal(s.uow, s.factory, s.publisher, s.fakeO11y)
	_, _ = sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: waRaw})

	s.assertNoSensitiveFieldsInLogs()
}

func (s *LoggingRegressionSuite) TestNoSensitiveFieldsOnDBError() {
	const waRaw = "+5511987654321"
	wa, err := valueobjects.NewWhatsAppNumber(waRaw)
	s.Require().NoError(err)

	channelWA := valueobjects.ChannelWhatsApp()
	externalIDWA, errExt := valueobjects.NewExternalID(channelWA, wa.String())
	s.Require().NoError(errExt)

	s.factory.EXPECT().UserIdentityRepository(mock.Anything).Return(s.idRepo).Once()
	s.idRepo.EXPECT().TryFindActive(mock.Anything, channelWA, externalIDWA).Return(entities.UserIdentity{}, false, nil).Once()
	s.factory.EXPECT().UserRepository(mock.Anything).Return(s.repo).Once()
	s.repo.EXPECT().TryFindActiveByWhatsApp(mock.Anything, wa).
		Return(entities.User{}, false, errors.New("db_unavailable")).Once()

	sut := NewEstablishPrincipal(s.uow, s.factory, s.publisher, s.fakeO11y)
	_, _ = sut.Execute(s.ctx, input.EstablishPrincipalInput{WhatsAppNumber: waRaw})

	s.assertNoSensitiveFieldsInLogs()
}

func (s *LoggingRegressionSuite) assertNoSensitiveFieldsInLogs() {
	logger, ok := s.fakeO11y.Logger().(*fake.FakeLogger)
	s.Require().True(ok, "logger deve ser *fake.FakeLogger")

	entries := logger.GetEntries()
	forbidden := []string{"+5511987654321", "Authorization", "META_APP_SECRET"}

	for _, entry := range entries {
		for _, f := range forbidden {
			s.NotContainsf(entry.Message, f,
				"campo sensivel '%s' encontrado na mensagem de log: %s", f, entry.Message)
		}
		for _, field := range entry.Fields {
			for _, f := range forbidden {
				s.NotContainsf(field.Key, f,
					"campo sensivel '%s' encontrado na chave do campo: %s", f, field.Key)
				s.NotContainsf(field.StringValue(), f,
					"campo sensivel '%s' encontrado no valor do campo '%s': %s", f, field.Key, field.StringValue())
			}
		}
	}
}
