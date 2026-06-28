//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AdvisoryLockSuite struct {
	suite.Suite
	dsn string
}

func TestAdvisoryLockSuite(t *testing.T) {
	suite.Run(t, new(AdvisoryLockSuite))
}

func (s *AdvisoryLockSuite) SetupSuite() {
	_, dsn := testcontainer.Postgres(s.T())
	s.dsn = dsn
}

func (s *AdvisoryLockSuite) openDB() *sql.DB {
	db, err := sqlx.Open("pgx", s.dsn)
	s.Require().NoError(err)
	s.Require().NoError(db.Ping())
	return db.DB
}

func (s *AdvisoryLockSuite) TestAdvisoryLockBlocksConcurrentAcquire() {
	ctx := context.Background()

	db1 := s.openDB()
	defer db1.Close()

	db2 := s.openDB()
	defer db2.Close()

	unlock1, err := acquireMigrationLock(ctx, db1)
	s.Require().NoError(err)
	s.Require().NotNil(unlock1)
	defer unlock1()

	unlock2, err := acquireMigrationLock(ctx, db2)
	s.Require().Error(err)
	s.Require().Contains(err.Error(), "outro processo de migrate esta em execucao")
	s.Require().Nil(unlock2)
}

func (s *AdvisoryLockSuite) TestAdvisoryLockReleasesAfterUnlock() {
	ctx := context.Background()

	db1 := s.openDB()
	defer db1.Close()

	db2 := s.openDB()
	defer db2.Close()

	unlock1, err := acquireMigrationLock(ctx, db1)
	s.Require().NoError(err)
	s.Require().NotNil(unlock1)

	unlock1()

	unlock2, err := acquireMigrationLock(ctx, db2)
	s.Require().NoError(err)
	s.Require().NotNil(unlock2)
	defer unlock2()
}
