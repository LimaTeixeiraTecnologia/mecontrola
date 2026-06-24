package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
)

type OnboardingSessionDriftSuite struct {
	suite.Suite
}

func TestOnboardingSessionDriftSuite(t *testing.T) {
	suite.Run(t, new(OnboardingSessionDriftSuite))
}

type driftRowDriver struct {
	columns []string
	row     []driver.Value
}

func (d driftRowDriver) Open(string) (driver.Conn, error) {
	return driftRowConn(d), nil
}

type driftRowConn struct {
	columns []string
	row     []driver.Value
}

func (c driftRowConn) Prepare(_ string) (driver.Stmt, error) {
	return driftRowStmt(c), nil
}
func (c driftRowConn) Close() error              { return nil }
func (c driftRowConn) Begin() (driver.Tx, error) { return nil, io.EOF }

type driftRowStmt struct {
	columns []string
	row     []driver.Value
}

func (s driftRowStmt) Close() error  { return nil }
func (s driftRowStmt) NumInput() int { return -1 }
func (s driftRowStmt) Exec(_ []driver.Value) (driver.Result, error) {
	return nil, io.EOF
}
func (s driftRowStmt) Query(_ []driver.Value) (driver.Rows, error) {
	return &driftDriverRows{columns: s.columns, row: s.row}, nil
}

type driftDriverRows struct {
	columns []string
	row     []driver.Value
	read    bool
}

func (r *driftDriverRows) Columns() []string { return r.columns }
func (r *driftDriverRows) Close() error      { return nil }
func (r *driftDriverRows) Next(dest []driver.Value) error {
	if r.read {
		return io.EOF
	}
	r.read = true
	copy(dest, r.row)
	return nil
}

func buildDriftDB(t *testing.T, name string, row []driver.Value) *sql.DB {
	cols := []string{"user_id", "channel", "state", "payload", "updated_at"}
	sql.Register(name, driftRowDriver{columns: cols, row: row})
	db, err := sql.Open(name, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func (s *OnboardingSessionDriftSuite) TestFind_DriftDetected_ActiveWithoutCompletedAt() {
	userID := uuid.New()
	payload, err := json.Marshal(onboardingSessionPayloadJSON{
		IncomeCents: 100000,
	})
	s.Require().NoError(err)

	row := []driver.Value{
		[]byte(userID.String()),
		"whatsapp",
		"active",
		payload,
		time.Now().UTC(),
	}
	db := buildDriftDB(s.T(), "onboarding_drift_active_no_completed_at", row)

	obs := fake.NewProvider()
	repo := NewOnboardingSessionRepository(obs, db)

	got, findErr := repo.Find(context.Background(), userID)
	s.Require().NoError(findErr)
	s.False(got.IsActive())
	s.Nil(got.Payload().CompletedAt)

	metrics := obs.Metrics().(*fake.FakeMetrics)
	counter := metrics.GetCounter("onboarding_state_drift_total")
	s.Require().NotNil(counter, "counter onboarding_state_drift_total deve existir")
	values := counter.GetValues()
	s.Require().Len(values, 1, "counter deve ter sido incrementado uma vez")
	s.Equal(int64(1), values[0].Value)

	logs := obs.Logger().(*fake.FakeLogger).GetEntries()
	var warnFound bool
	for _, e := range logs {
		if e.Message == "onboarding.repository.state_drift" {
			warnFound = true
			break
		}
	}
	s.True(warnFound, "warn onboarding.repository.state_drift deve ter sido emitido")
}

func (s *OnboardingSessionDriftSuite) TestFind_NoDrift_ActiveWithCompletedAt() {
	userID := uuid.New()
	now := time.Now().UTC()
	payload, err := json.Marshal(onboardingSessionPayloadJSON{
		IncomeCents: 100000,
		CompletedAt: &now,
	})
	s.Require().NoError(err)

	row := []driver.Value{
		[]byte(userID.String()),
		"whatsapp",
		"active",
		payload,
		time.Now().UTC(),
	}
	db := buildDriftDB(s.T(), "onboarding_drift_active_with_completed_at", row)

	obs := fake.NewProvider()
	repo := NewOnboardingSessionRepository(obs, db)

	_, findErr := repo.Find(context.Background(), userID)
	s.Require().NoError(findErr)

	metrics := obs.Metrics().(*fake.FakeMetrics)
	counter := metrics.GetCounter("onboarding_state_drift_total")
	if counter != nil {
		s.Empty(counter.GetValues(), "counter nao deve ser incrementado quando completed_at presente")
	}
}
