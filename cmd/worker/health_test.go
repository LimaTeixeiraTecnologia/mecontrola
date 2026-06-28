package worker

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
)

type fakeDriver struct {
	failPing bool
}

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	return &fakeConn{failPing: d.failPing}, nil
}

type fakeConn struct {
	failPing bool
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, nil }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return nil, nil }
func (c *fakeConn) Ping(context.Context) error {
	if c.failPing {
		return errors.New("db unavailable")
	}
	return nil
}

type HealthServerSuite struct {
	suite.Suite
}

func TestHealthServerSuite(t *testing.T) {
	suite.Run(t, new(HealthServerSuite))
}

func (s *HealthServerSuite) openDB(failPing bool) *sql.DB {
	name := fmt.Sprintf("fake-driver-%d-%d", rand.Int(), rand.Int())
	sql.Register(name, &fakeDriver{failPing: failPing})

	db, err := sql.Open(name, "")
	s.Require().NoError(err)

	return db
}

func (s *HealthServerSuite) TestLivez() {
	h := newHealthServer(nil, nil, "")
	srv := httptest.NewServer(h.server.Handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/livez")
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
}

func (s *HealthServerSuite) TestReadyz_Success() {
	db := sqlx.NewDb(s.openDB(false), "fake")
	defer func() { _ = db.Close() }()

	h := newHealthServer(db, nil, "")
	srv := httptest.NewServer(h.server.Handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	s.Require().Equal(http.StatusOK, resp.StatusCode)
}

func (s *HealthServerSuite) TestReadyz_DBFailure() {
	db := sqlx.NewDb(s.openDB(true), "fake")
	defer func() { _ = db.Close() }()

	h := newHealthServer(db, nil, "")
	srv := httptest.NewServer(h.server.Handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/readyz")
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	s.Require().Equal(http.StatusServiceUnavailable, resp.StatusCode)
}

func (s *HealthServerSuite) TestShutdown() {
	h := newHealthServer(nil, nil, "127.0.0.1:0")
	s.Require().NoError(h.start(context.Background()))

	baseURL := fmt.Sprintf("http://%s", h.ln.Addr().String())
	resp, err := http.Get(baseURL + "/livez")
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	s.Require().NoError(resp.Body.Close())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Require().NoError(h.shutdown(shutdownCtx))

	_, err = http.Get(baseURL + "/livez")
	s.Require().Error(err)
}
