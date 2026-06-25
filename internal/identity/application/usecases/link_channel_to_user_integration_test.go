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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type LinkChannelToUserIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestLinkChannelToUserIntegration(t *testing.T) {
	suite.Run(t, new(LinkChannelToUserIntegrationSuite))
}

func (s *LinkChannelToUserIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *LinkChannelToUserIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func (s *LinkChannelToUserIntegrationSuite) newSUT() *usecases.LinkChannelToUser {
	factory := repositories.NewRepositoryFactory(s.o11y)
	u := uow.NewUnitOfWork(s.db)
	return usecases.NewLinkChannelToUser(u, factory, s.o11y)
}

func (s *LinkChannelToUserIntegrationSuite) seedUser(waNumber string) uuid.UUID {
	s.T().Helper()
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.UserRepository(s.db)
	waNum, err := valueobjects.NewWhatsAppNumber(waNumber)
	s.Require().NoError(err)
	candidate := entities.New(waNum)
	user, err := repo.UpsertByWhatsAppNumber(s.ctx, candidate, time.Now().UTC())
	s.Require().NoError(err)
	return uuid.MustParse(user.ID())
}

func (s *LinkChannelToUserIntegrationSuite) countActiveLinksForUser(userID uuid.UUID, channel string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM mecontrola.user_identities WHERE user_id = $1 AND channel = $2 AND unlinked_at IS NULL`,
		userID,
		channel,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *LinkChannelToUserIntegrationSuite) TestLinkChannelToUser() {
	type args struct {
		userID     uuid.UUID
		channel    string
		externalID string
	}

	scenarios := []struct {
		name   string
		setup  func() args
		expect func(usecases.LinkChannelResult, error, args)
	}{
		{
			name: "vincular canal novo ao usuario: linha com unlinked_at IS NULL",
			setup: func() args {
				userID := s.seedUser("+5511900000031")
				return args{userID: userID, channel: "whatsapp", externalID: "+5511111111111"}
			},
			expect: func(res usecases.LinkChannelResult, err error, a args) {
				s.Require().NoError(err)
				s.False(res.AlreadyLinked)
				s.NotEqual(uuid.Nil, res.IdentityID)
				s.Equal(1, s.countActiveLinksForUser(a.userID, a.channel))
			},
		},
		{
			name: "vincular mesmo canal+externalID ao mesmo usuario: AlreadyLinked=true COUNT permanece 1",
			setup: func() args {
				userID := s.seedUser("+5511900000032")
				return args{userID: userID, channel: "whatsapp", externalID: "+5511222222222"}
			},
			expect: func(res usecases.LinkChannelResult, err error, a args) {
				s.Require().NoError(err)
				s.Require().False(res.AlreadyLinked)

				sut := s.newSUT()
				res2, err2 := sut.Execute(s.ctx, input.LinkChannelToUser{
					UserID:     a.userID,
					Channel:    a.channel,
					ExternalID: a.externalID,
				})
				s.Require().NoError(err2)
				s.True(res2.AlreadyLinked)
				s.Equal(1, s.countActiveLinksForUser(a.userID, a.channel))
			},
		},
		{
			name: "vincular externalID ja usado por outro usuario ativo: erro ErrUserIdentityAlreadyLinked",
			setup: func() args {
				userIDA := s.seedUser("+5511900000033")
				userIDB := s.seedUser("+5511900000034")

				sut := s.newSUT()
				_, err := sut.Execute(s.ctx, input.LinkChannelToUser{
					UserID:     userIDA,
					Channel:    "whatsapp",
					ExternalID: "+5511333333333",
				})
				s.Require().NoError(err)

				return args{userID: userIDB, channel: "whatsapp", externalID: "+5511333333333"}
			},
			expect: func(res usecases.LinkChannelResult, err error, a args) {
				s.Require().Error(err)
				s.ErrorIs(err, application.ErrUserIdentityAlreadyLinked)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			a := scenario.setup()
			sut := s.newSUT()
			res, err := sut.Execute(s.ctx, input.LinkChannelToUser{
				UserID:     a.userID,
				Channel:    a.channel,
				ExternalID: a.externalID,
			})
			scenario.expect(res, err, a)
		})
	}
}
