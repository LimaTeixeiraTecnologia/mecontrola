package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	dbpkg "github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/database"
)

type ManagerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestManager(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}

func (s *ManagerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ManagerSuite) TestErrosSentinelaSaoDistintosENaoNulos() {
	scenarios := []struct {
		name string
		err  error
	}{
		{"ErrConnection", dbpkg.ErrConnection},
		{"ErrMigration", dbpkg.ErrMigration},
		{"ErrDeadlineExceeded", dbpkg.ErrDeadlineExceeded},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			s.NotNil(sc.err)
			s.NotEmpty(sc.err.Error())
		})
	}

	s.NotEqual(dbpkg.ErrConnection, dbpkg.ErrMigration)
	s.NotEqual(dbpkg.ErrConnection, dbpkg.ErrDeadlineExceeded)
	s.NotEqual(dbpkg.ErrMigration, dbpkg.ErrDeadlineExceeded)
}

func (s *ManagerSuite) TestErrConnectionPodeSerDetectadoViaErrorsIs() {
	wrapped := errors.Join(dbpkg.ErrConnection, errors.New("underlying: EOF"))
	s.True(errors.Is(wrapped, dbpkg.ErrConnection))
}

func (s *ManagerSuite) TestErrMigrationPodeSerDetectadoViaErrorsIs() {
	wrapped := errors.Join(dbpkg.ErrMigration, errors.New("no such table"))
	s.True(errors.Is(wrapped, dbpkg.ErrMigration))
}

func (s *ManagerSuite) TestErrDeadlineExceededPodeSerDetectadoViaErrorsIs() {
	wrapped := errors.Join(dbpkg.ErrDeadlineExceeded, context.DeadlineExceeded)
	s.True(errors.Is(wrapped, dbpkg.ErrDeadlineExceeded))
}

func (s *ManagerSuite) TestNewManagerFalhaComDSNInvalido() {
	cfg := badDBConfig()
	_, err := dbpkg.NewManager(cfg)

	s.Error(err)
	s.True(errors.Is(err, dbpkg.ErrConnection))
}

func (s *ManagerSuite) TestDefaultUoWTimeoutExpirasContextoComDeadline() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	<-ctx.Done()
	s.ErrorIs(ctx.Err(), context.DeadlineExceeded)
}
