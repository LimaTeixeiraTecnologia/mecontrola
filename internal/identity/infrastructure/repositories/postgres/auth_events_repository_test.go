package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	repopostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type AuthEventsRepositorySuite struct {
	suite.Suite
	ctx context.Context
}

func TestAuthEventsRepositorySuite(t *testing.T) {
	suite.Run(t, new(AuthEventsRepositorySuite))
}

func (s *AuthEventsRepositorySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthEventsRepositorySuite) TestConstructor() {
	scenarios := []struct {
		name   string
		expect func(interfaces.AuthEventsRepository)
	}{
		{
			name: "deve criar repositorio nao nulo com db nil",
			expect: func(repo interfaces.AuthEventsRepository) {
				s.NotNil(repo)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := repopostgres.NewAuthEventsRepository(noop.NewProvider(), nil)
			scenario.expect(repo)
		})
	}
}

func (s *AuthEventsRepositorySuite) TestInsert() {
	type args struct {
		event entities.AuthEvent
	}

	uid := uuid.New()
	reason := entities.AuthEventReasonInvalidSignature
	sampleEvent := entities.HydrateAuthEvent(
		uuid.New(),
		time.Now().UTC(),
		&uid,
		entities.AuthEventKindPrincipalEstablished,
		entities.AuthEventSourceWhatsApp,
		&reason,
		"",
		"",
	)

	scenarios := []struct {
		name   string
		args   args
		setup  func() *dbmocks.MockDBTX
		expect func(error)
	}{
		{
			name: "deve retornar erro quando ExecContext falha",
			args: args{event: sampleEvent},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything,
				).Return(nil, errors.New("db error")).Once()
				return dbtx
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "insert")
			},
		},
		{
			name: "deve inserir evento sem erros",
			args: args{event: sampleEvent},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything,
				).Return(result, nil).Once()
				return dbtx
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			dbtx := scenario.setup()
			repo := repopostgres.NewAuthEventsRepository(noop.NewProvider(), dbtx)
			err := repo.Insert(s.ctx, scenario.args.event)
			scenario.expect(err)
		})
	}
}

func (s *AuthEventsRepositorySuite) TestAnonymizeByUserID() {
	type args struct {
		userID uuid.UUID
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() *dbmocks.MockDBTX
		expect func(error)
	}{
		{
			name: "deve retornar erro quando ExecContext falha",
			args: args{userID: uuid.New()},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("uuid.UUID")).
					Return(nil, errors.New("db error")).Once()
				return dbtx
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "anonymize_by_user_id")
			},
		},
		{
			name: "deve anonimizar sem erros",
			args: args{userID: uuid.New()},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("uuid.UUID")).
					Return(result, nil).Once()
				return dbtx
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			dbtx := scenario.setup()
			repo := repopostgres.NewAuthEventsRepository(noop.NewProvider(), dbtx)
			err := repo.AnonymizeByUserID(s.ctx, scenario.args.userID)
			scenario.expect(err)
		})
	}
}

func (s *AuthEventsRepositorySuite) TestDeleteOlderThan() {
	type args struct {
		cutoff    time.Time
		batchSize int
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func() *dbmocks.MockDBTX
		expect func(int64, error)
	}{
		{
			name: "deve retornar erro quando ExecContext falha",
			args: args{cutoff: time.Now().UTC().Add(-24 * time.Hour), batchSize: 100},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					mock.Anything, mock.Anything,
				).Return(nil, errors.New("db error")).Once()
				return dbtx
			},
			expect: func(n int64, err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "delete_older_than")
				s.Zero(n)
			},
		},
		{
			name: "deve retornar erro quando RowsAffected falha",
			args: args{cutoff: time.Now().UTC().Add(-24 * time.Hour), batchSize: 100},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				result.EXPECT().RowsAffected().Return(int64(0), errors.New("rows affected error")).Once()
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					mock.Anything, mock.Anything,
				).Return(result, nil).Once()
				return dbtx
			},
			expect: func(n int64, err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "delete_older_than")
				s.Zero(n)
			},
		},
		{
			name: "deve retornar count de linhas deletadas",
			args: args{cutoff: time.Now().UTC().Add(-24 * time.Hour), batchSize: 100},
			setup: func() *dbmocks.MockDBTX {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				result.EXPECT().RowsAffected().Return(int64(42), nil).Once()
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					mock.Anything, mock.Anything,
				).Return(result, nil).Once()
				return dbtx
			},
			expect: func(n int64, err error) {
				s.Require().NoError(err)
				s.Equal(int64(42), n)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			dbtx := scenario.setup()
			repo := repopostgres.NewAuthEventsRepository(noop.NewProvider(), dbtx)
			n, err := repo.DeleteOlderThan(s.ctx, scenario.args.cutoff, scenario.args.batchSize)
			scenario.expect(n, err)
		})
	}
}
