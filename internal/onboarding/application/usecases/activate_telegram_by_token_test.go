package usecases_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	identityapp "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	identityinterfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	identityentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	identityvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	appinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/entities"
	domainservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/domain/valueobjects"
)

type stubBinder struct {
	called   bool
	returned entities.MagicToken
	err      error
}

func (b *stubBinder) BindAndConsume(
	_ context.Context,
	_ appinterfaces.MagicTokenRepository,
	magicToken entities.MagicToken,
	_ string,
	_ valueobjects.ActivationPath,
	_ time.Time,
) (entities.MagicToken, error) {
	b.called = true
	if b.err != nil {
		return entities.MagicToken{}, b.err
	}
	if b.returned.ID() == "" {
		return magicToken, nil
	}
	return b.returned, nil
}

var validTokenClear = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte("a"), 32))

type passthroughUoW struct{}

func (passthroughUoW) DBTX() database.DBTX { return nil }

func (passthroughUoW) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type ActivateTelegramByTokenSuite struct {
	suite.Suite
	ctx context.Context
}

func TestActivateTelegramByToken(t *testing.T) {
	suite.Run(t, new(ActivateTelegramByTokenSuite))
}

func (s *ActivateTelegramByTokenSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ActivateTelegramByTokenSuite) buildToken(status valueobjects.TokenStatus, consumedByUserID string, expiresAt time.Time) entities.MagicToken {
	tok, err := valueobjects.TokenFromClear(validTokenClear)
	s.Require().NoError(err)
	now := time.Now().UTC()
	if expiresAt.IsZero() {
		expiresAt = now.Add(24 * time.Hour)
	}
	return entities.HydrateMagicToken(
		uuid.New().String(),
		tok.Hash(),
		status,
		"plan-x",
		expiresAt,
		now,
		time.Time{}, time.Time{}, time.Time{},
		"",
		"sub-1",
		"+5511987654321",
		"customer@example.com",
		"sale-1",
		consumedByUserID,
		"+5511987654321",
		valueobjects.ActivationPathDirect,
		"",
	)
}

func (s *ActivateTelegramByTokenSuite) buildSut() (*usecases.ActivateTelegramByToken, *interfacesmocks.RepositoryFactory, *interfacesmocks.MagicTokenRepository, *identityinterfacesmocks.MockRepositoryFactory, *identityinterfacesmocks.MockUserIdentityRepository) {
	sut, factory, tokenRepo, identityFactory, identityRepo, _ := s.buildSutWith(false)
	return sut, factory, tokenRepo, identityFactory, identityRepo
}

func (s *ActivateTelegramByTokenSuite) buildSutWith(directEnabled bool) (
	*usecases.ActivateTelegramByToken,
	*interfacesmocks.RepositoryFactory,
	*interfacesmocks.MagicTokenRepository,
	*identityinterfacesmocks.MockRepositoryFactory,
	*identityinterfacesmocks.MockUserIdentityRepository,
	*stubBinder,
) {
	factory := interfacesmocks.NewRepositoryFactory(s.T())
	tokenRepo := interfacesmocks.NewMagicTokenRepository(s.T())
	identityFactory := identityinterfacesmocks.NewMockRepositoryFactory(s.T())
	identityRepo := identityinterfacesmocks.NewMockUserIdentityRepository(s.T())
	binder := &stubBinder{}

	u := passthroughUoW{}
	sut := usecases.NewActivateTelegramByToken(
		factory,
		identityFactory,
		u,
		domainservices.NewDirectTelegramActivationWorkflow(),
		binder,
		directEnabled,
		noop.NewProvider(),
	)
	return sut, factory, tokenRepo, identityFactory, identityRepo, binder
}

func (s *ActivateTelegramByTokenSuite) TestExecute_InvalidTelegramID() {
	sut, _, _, _, _ := s.buildSut()
	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 0})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeNotFound, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_InvalidToken() {
	sut, _, _, _, _ := s.buildSut()
	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: "x", TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeNotFound, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenNotFoundInRepo() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, domain.ErrTokenNotFound).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeNotFound, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenExpiredViaTime() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	expired := time.Now().UTC().Add(-1 * time.Hour)
	mt := s.buildToken(valueobjects.TokenStatusPaid, "", expired)

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeExpired, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenPending() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	mt := s.buildToken(valueobjects.TokenStatusPending, "", time.Time{})

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeNotYetPaid, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenPaidRequiresWhatsAppFirst() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	mt := s.buildToken(valueobjects.TokenStatusPaid, "", time.Time{})

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenConsumed_LinksFresh() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	userID := uuid.New()
	mt := s.buildToken(valueobjects.TokenStatusConsumed, userID.String(), time.Time{})

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, mock.Anything, mock.Anything).Return(identityentities.UserIdentity{}, false, nil).Once()
	identityRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeLinked, res.Outcome)
	s.Equal(userID, res.UserID)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenConsumed_AlreadyLinkedSameUser() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	userID := uuid.New()
	mt := s.buildToken(valueobjects.TokenStatusConsumed, userID.String(), time.Time{})

	channel := identityvo.ChannelTelegram()
	externalID, errExt := identityvo.NewExternalID(channel, "12345")
	s.Require().NoError(errExt)
	existing, errID := identityentities.NewUserIdentity(uuid.New(), userID, channel, externalID, time.Now().UTC())
	s.Require().NoError(errID)

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(existing, true, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeAlreadyLinked, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenConsumed_ReusedOtherAccount() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	tokenOwnerUserID := uuid.New()
	otherUserID := uuid.New()
	mt := s.buildToken(valueobjects.TokenStatusConsumed, tokenOwnerUserID.String(), time.Time{})

	channel := identityvo.ChannelTelegram()
	externalID, errExt := identityvo.NewExternalID(channel, "12345")
	s.Require().NoError(errExt)
	existing, errID := identityentities.NewUserIdentity(uuid.New(), otherUserID, channel, externalID, time.Now().UTC())
	s.Require().NoError(errID)

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(existing, true, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeReusedOtherAccount, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenConsumed_RaceResolvedSameUser() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	userID := uuid.New()
	mt := s.buildToken(valueobjects.TokenStatusConsumed, userID.String(), time.Time{})

	channel := identityvo.ChannelTelegram()
	externalID, errExt := identityvo.NewExternalID(channel, "12345")
	s.Require().NoError(errExt)
	winner, errID := identityentities.NewUserIdentity(uuid.New(), userID, channel, externalID, time.Now().UTC())
	s.Require().NoError(errID)

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(identityentities.UserIdentity{}, false, nil).Once()
	identityRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(identityapp.ErrUserIdentityAlreadyLinked).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, channel, externalID).Return(winner, true, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeAlreadyLinked, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenConsumed_InvalidUserIDInToken() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()
	mt := s.buildToken(valueobjects.TokenStatusConsumed, "not-a-uuid", time.Time{})

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation, res.Outcome)
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenPaid_DirectFlagDisabled_PreservesWhatsAppRule() {
	sut, factory, tokenRepo, identityFactory, identityRepo, binder := s.buildSutWith(false)
	mt := s.buildToken(valueobjects.TokenStatusPaid, "", time.Time{})

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation, res.Outcome)
	s.False(binder.called, "binder nao deve ser chamado quando flag desabilitada")
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenPaid_DirectFlagEnabled_LinksTelegram() {
	sut, factory, tokenRepo, identityFactory, identityRepo, binder := s.buildSutWith(true)
	userID := uuid.New()
	mt := s.buildToken(valueobjects.TokenStatusPaid, "", time.Time{})
	consumed, errConsume := mt.MarkConsumed(userID.String(), "+5511987654321", valueobjects.ActivationPathDirect, time.Now().UTC())
	s.Require().NoError(errConsume)
	binder.returned = consumed

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()
	tokenRepo.EXPECT().UpdateTelegramExternalID(mock.Anything, mt.ID(), "12345").Return(nil).Once()
	identityRepo.EXPECT().TryFindActive(mock.Anything, mock.Anything, mock.Anything).Return(identityentities.UserIdentity{}, false, nil).Once()
	identityRepo.EXPECT().Insert(mock.Anything, mock.Anything).Return(nil).Once()

	res, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(err)
	s.Equal(usecases.ActivateTelegramOutcomeLinked, res.Outcome)
	s.Equal(userID, res.UserID)
	s.True(binder.called, "binder deve ser chamado quando flag habilitada e dados completos")
}

func (s *ActivateTelegramByTokenSuite) TestExecute_TokenPaid_DirectFlagEnabled_MissingDataBlocksDirect() {
	sut, factory, tokenRepo, identityFactory, identityRepo, binder := s.buildSutWith(true)
	tok, err := valueobjects.TokenFromClear(validTokenClear)
	s.Require().NoError(err)
	now := time.Now().UTC()
	mt := entities.HydrateMagicToken(
		uuid.New().String(),
		tok.Hash(),
		valueobjects.TokenStatusPaid,
		"plan-x",
		now.Add(24*time.Hour),
		now,
		time.Time{}, time.Time{}, time.Time{},
		"",
		"sub-1",
		"",
		"",
		"sale-1",
		"",
		"",
		valueobjects.ActivationPathDirect,
		"",
	)

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(mt, nil).Once()

	res, errExec := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().NoError(errExec)
	s.Equal(usecases.ActivateTelegramOutcomeRequiresWhatsAppActivation, res.Outcome)
	s.False(binder.called, "binder nao deve ser chamado quando dados estao ausentes")
}

func (s *ActivateTelegramByTokenSuite) TestExecute_FindByHashError_Propagates() {
	sut, factory, tokenRepo, identityFactory, identityRepo := s.buildSut()

	factory.EXPECT().MagicTokenRepository(mock.Anything).Return(tokenRepo).Once()
	identityFactory.EXPECT().UserIdentityRepository(mock.Anything).Return(identityRepo).Once()
	tokenRepo.EXPECT().FindByHash(mock.Anything, mock.Anything).Return(entities.MagicToken{}, errors.New("db down")).Once()

	_, err := sut.Execute(s.ctx, usecases.ActivateTelegramByTokenInput{Token: validTokenClear, TelegramUserID: 12345})
	s.Require().Error(err)
}
