//go:build integration

package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ResolvePrincipalByIdentityIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestResolvePrincipalByIdentityIntegration(t *testing.T) {
	suite.Run(t, new(ResolvePrincipalByIdentityIntegrationSuite))
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) newSUT() *usecases.ResolvePrincipalByIdentity {
	factory := repositories.NewRepositoryFactory(s.o11y)
	u := uow.NewUnitOfWork(s.db)
	return usecases.NewResolvePrincipalByIdentity(u, factory, s.o11y)
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) seedUserWithWhatsAppIdentity(waNumber, externalNumber string) entities.User {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.db)
	waNum, err := valueobjects.NewWhatsAppNumber(waNumber)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	userUID := uuid.MustParse(user.ID())
	identityRepo := factory.UserIdentityRepository(s.db)
	ch := valueobjects.ChannelWhatsApp()
	extID, err := valueobjects.NewExternalID(ch, externalNumber)
	s.Require().NoError(err)
	identityID, err := uuid.NewV7()
	s.Require().NoError(err)
	identity, err := entities.NewUserIdentity(identityID, userUID, ch, extID, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(identityRepo.Insert(s.ctx, identity))
	return user
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) seedUserWithUnlinkedWhatsAppIdentity(waNumber, externalNumber string) {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.db)
	waNum, err := valueobjects.NewWhatsAppNumber(waNumber)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)

	userUID := uuid.MustParse(user.ID())
	identityRepo := factory.UserIdentityRepository(s.db)
	ch := valueobjects.ChannelWhatsApp()
	extID, err := valueobjects.NewExternalID(ch, externalNumber)
	s.Require().NoError(err)
	identityID, err := uuid.NewV7()
	s.Require().NoError(err)
	identity, err := entities.NewUserIdentity(identityID, userUID, ch, extID, time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NoError(identityRepo.Insert(s.ctx, identity))
	s.Require().NoError(identityRepo.Unlink(s.ctx, identityID, time.Now().UTC()))
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) countActiveIdentitiesByChannelAndExtID(channel, externalID string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM mecontrola.user_identities WHERE channel = $1 AND external_id = $2 AND unlinked_at IS NULL`,
		channel,
		externalID,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *ResolvePrincipalByIdentityIntegrationSuite) TestResolvePrincipalByIdentity() {
	type args struct {
		channel    string
		externalID string
	}

	scenarios := []struct {
		name   string
		setup  func() args
		expect func(auth.Principal, error)
	}{
		{
			name: "canal whatsapp ativo: retorna Principal com UserID correto",
			setup: func() args {
				const externalNumber = "+5511900000011"
				user := s.seedUserWithWhatsAppIdentity("+5511900000010", externalNumber)
				s.NotEmpty(user.ID())
				return args{channel: "whatsapp", externalID: externalNumber}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().NoError(err)
				s.False(p.IsZero())
				s.Equal(auth.SourceWhatsApp, p.Source)
				s.Equal(1, s.countActiveIdentitiesByChannelAndExtID("whatsapp", "+5511900000011"))
			},
		},
		{
			name: "canal sem registro: retorna ErrUnknownUser",
			setup: func() args {
				return args{channel: "whatsapp", externalID: "+5519999999999"}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
			},
		},
		{
			name: "canal existe mas unlinked_at IS NOT NULL: retorna ErrUnknownUser",
			setup: func() args {
				const externalNumber = "+5511900000022"
				s.seedUserWithUnlinkedWhatsAppIdentity("+5511900000021", externalNumber)
				return args{channel: "whatsapp", externalID: externalNumber}
			},
			expect: func(p auth.Principal, err error) {
				s.Require().ErrorIs(err, application.ErrUnknownUser)
				s.True(p.IsZero())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.setup()
			sut := s.newSUT()
			p, err := sut.Execute(s.ctx, input.ResolvePrincipalByIdentity{
				Channel:    a.channel,
				ExternalID: a.externalID,
			})
			scenario.expect(p, err)
		})
	}
}
