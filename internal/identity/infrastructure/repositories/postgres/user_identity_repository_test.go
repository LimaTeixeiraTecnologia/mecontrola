package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
	repopostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type UserIdentityRepositorySuite struct {
	suite.Suite
	ctx context.Context
}

func TestUserIdentityRepositorySuite(t *testing.T) {
	suite.Run(t, new(UserIdentityRepositorySuite))
}

func (s *UserIdentityRepositorySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UserIdentityRepositorySuite) buildIdentity() entities.UserIdentity {
	channel := valueobjects.ChannelWhatsApp()
	externalID, err := valueobjects.NewExternalID(channel, "+5511900000001")
	s.Require().NoError(err)
	identity, err := entities.NewUserIdentity(uuid.New(), uuid.New(), channel, externalID, time.Now().UTC())
	s.Require().NoError(err)
	return identity
}

func (s *UserIdentityRepositorySuite) TestInsertIfAbsent() {
	scenarios := []struct {
		name   string
		setup  func() *dbmocks.MockDBTX
		expect func(bool, error)
	}{
		{
			name: "deve criar vinculo e liberar savepoint quando insert bem-sucedido",
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.MatchedBy(func(q string) bool {
					return q == "SAVEPOINT identity_link_savepoint"
				})).Return(result, nil).Once()
				dbtx.EXPECT().ExecContext(mock.Anything, mock.MatchedBy(func(q string) bool {
					return strings.Contains(q, "INSERT INTO mecontrola.user_identities")
				}),
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				).Return(result, nil).Once()
				dbtx.EXPECT().ExecContext(mock.Anything, mock.MatchedBy(func(q string) bool {
					return q == "RELEASE SAVEPOINT identity_link_savepoint"
				})).Return(result, nil).Once()
				return dbtx
			},
			expect: func(created bool, err error) {
				s.Require().NoError(err)
				s.True(created)
			},
		},
		{
			name: "deve retornar erro quando SAVEPOINT falha",
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.MatchedBy(func(q string) bool {
					return q == "SAVEPOINT identity_link_savepoint"
				})).Return(nil, errors.New("savepoint failed")).Once()
				return dbtx
			},
			expect: func(created bool, err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "savepoint")
				s.False(created)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			dbtx := scenario.setup()
			repo := repopostgres.NewUserIdentityRepository(noop.NewProvider(), dbtx)
			created, err := repo.InsertIfAbsent(s.ctx, s.buildIdentity())
			scenario.expect(created, err)
		})
	}
}
