package outbox_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type StoragePostgresSuite struct {
	suite.Suite
}

func TestStoragePostgres(t *testing.T) {
	suite.Run(t, new(StoragePostgresSuite))
}

func (s *StoragePostgresSuite) SetupTest() {}

var emptyRowsDriverOnce sync.Once

type emptyRowsDriver struct{}

func (emptyRowsDriver) Open(string) (driver.Conn, error) { return emptyRowsConn{}, nil }

type emptyRowsConn struct{}

func (emptyRowsConn) Prepare(query string) (driver.Stmt, error) { return emptyRowsStmt{}, nil }
func (emptyRowsConn) Close() error                              { return nil }
func (emptyRowsConn) Begin() (driver.Tx, error)                 { return nil, errors.New("not supported") }

type emptyRowsStmt struct{}

func (emptyRowsStmt) Close() error  { return nil }
func (emptyRowsStmt) NumInput() int { return -1 }
func (emptyRowsStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("not supported")
}
func (emptyRowsStmt) Query(args []driver.Value) (driver.Rows, error) { return emptyDriverRows{}, nil }

type emptyDriverRows struct{}

func (emptyDriverRows) Columns() []string              { return []string{"value"} }
func (emptyDriverRows) Close() error                   { return nil }
func (emptyDriverRows) Next(dest []driver.Value) error { return io.EOF }

func (s *StoragePostgresSuite) emptyRows() *sql.Rows {
	emptyRowsDriverOnce.Do(func() {
		sql.Register("outbox_empty_rows", emptyRowsDriver{})
	})
	db, err := sql.Open("outbox_empty_rows", "")
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = db.Close() })
	rows, err := db.QueryContext(context.Background(), "SELECT value")
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = rows.Close() })
	return rows
}

func (s *StoragePostgresSuite) newEvent() outbox.Event {
	event, err := outbox.NewEvent(outbox.EventInput{
		Type:          "test.event",
		AggregateType: "TestAggregate",
		AggregateID:   "agg-1",
		Payload:       []byte(`{"x":1}`),
	})
	s.Require().NoError(err)
	return event
}

func (s *StoragePostgresSuite) TestInsert() {
	type args struct {
		ctx context.Context
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args) (*dbmocks.MockDBTX, outbox.OutboxRepository, outbox.Event)
		expect func(error)
	}{
		{
			name: "deve inserir evento com sucesso",
			args: args{ctx: context.Background()},
			setup: func(input args) (*dbmocks.MockDBTX, outbox.OutboxRepository, outbox.Event) {
				dbtx := dbmocks.NewMockDBTX(s.T())
				event := s.newEvent()
				meta, err := json.Marshal(event.Metadata)
				s.Require().NoError(err)
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
					event.ID, event.Type, event.AggregateType, event.AggregateID, nil,
					event.Payload, meta,
					int(outbox.StatusPending), 15, event.OccurredAt,
				).Return(dbmocks.NewMockResult(s.T()), nil).Once()
				return dbtx, outbox.NewPostgresStorage(dbtx), event
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve propagar erro do exec",
			args: args{ctx: context.Background()},
			setup: func(input args) (*dbmocks.MockDBTX, outbox.OutboxRepository, outbox.Event) {
				dbtx := dbmocks.NewMockDBTX(s.T())
				event := s.newEvent()
				dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"), mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				).Return(nil, errors.New("db error")).Once()
				return dbtx, outbox.NewPostgresStorage(dbtx), event
			},
			expect: func(err error) { s.Error(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			_, storage, event := scenario.setup(scenario.args)
			err := storage.Insert(scenario.args.ctx, event, 15)
			scenario.expect(err)
		})
	}
}

func (s *StoragePostgresSuite) TestStorageMutations() {
	type args struct {
		ctx       context.Context
		id        string
		lastError string
		next      time.Time
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(args) outbox.OutboxRepository
		act    func(outbox.OutboxRepository, args) (int64, error)
		expect func(int64, error)
	}{
		{
			name: "deve retornar nil ao claim batch vazio",
			args: args{ctx: context.Background()},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				rows := s.emptyRows()
				dbtx.EXPECT().QueryContext(input.ctx, mock.AnythingOfType("string"), "inst-1", 50).Return(rows, nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				rows, err := storage.ClaimBatch(input.ctx, "inst-1", 50)
				s.Nil(rows)
				return 0, err
			},
			expect: func(_ int64, err error) { s.NoError(err) },
		},
		{
			name: "deve marcar evento como publicado",
			args: args{ctx: context.Background(), id: uuid.NewString()},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusPublished), input.id).Return(dbmocks.NewMockResult(s.T()), nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				return 0, storage.MarkPublished(input.ctx, input.id)
			},
			expect: func(_ int64, err error) { s.NoError(err) },
		},
		{
			name: "deve marcar evento como falho",
			args: args{ctx: context.Background(), id: uuid.NewString(), lastError: "handler failed"},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusFailed), input.lastError, input.id).Return(dbmocks.NewMockResult(s.T()), nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				return 0, storage.MarkFailed(input.ctx, input.id, input.lastError)
			},
			expect: func(_ int64, err error) { s.NoError(err) },
		},
		{
			name: "deve marcar retry pendente",
			args: args{ctx: context.Background(), id: uuid.NewString(), lastError: "transient", next: time.Now().UTC().Add(5 * time.Second)},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				dbtx.EXPECT().ExecContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusPending), input.lastError, input.next, input.id).Return(dbmocks.NewMockResult(s.T()), nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				return 0, storage.MarkPendingRetry(input.ctx, input.id, input.lastError, input.next)
			},
			expect: func(_ int64, err error) { s.NoError(err) },
		},
		{
			name: "deve resetar stuck",
			args: args{ctx: context.Background()},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				result.EXPECT().RowsAffected().Return(int64(3), nil).Once()
				dbtx.EXPECT().ExecContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusPending), int(outbox.StatusProcessing), (5*time.Minute).Microseconds()).Return(result, nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				return storage.ResetStuck(input.ctx, 5*time.Minute)
			},
			expect: func(result int64, err error) {
				s.NoError(err)
				s.Equal(int64(3), result)
			},
		},
		{
			name: "deve deletar lote publicado",
			args: args{ctx: context.Background()},
			setup: func(input args) outbox.OutboxRepository {
				dbtx := dbmocks.NewMockDBTX(s.T())
				result := dbmocks.NewMockResult(s.T())
				result.EXPECT().RowsAffected().Return(int64(1000), nil).Once()
				dbtx.EXPECT().ExecContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusPublished), (90*24*time.Hour).Microseconds(), 1000).Return(result, nil).Once()
				return outbox.NewPostgresStorage(dbtx)
			},
			act: func(storage outbox.OutboxRepository, input args) (int64, error) {
				return storage.DeletePublishedBatch(input.ctx, 90*24*time.Hour, 1000)
			},
			expect: func(result int64, err error) {
				s.NoError(err)
				s.Equal(int64(1000), result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			storage := scenario.setup(scenario.args)
			result, err := scenario.act(storage, scenario.args)
			scenario.expect(result, err)
		})
	}
}

func (s *StoragePostgresSuite) TestCountPending() {
	db, mockDB, err := sqlmock.New()
	s.Require().NoError(err)
	defer func() { _ = db.Close() }()

	mockDB.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(int(outbox.StatusPending)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	storage := outbox.NewPostgresStorage(db)
	count, err := storage.CountPending(context.Background())

	s.NoError(err)
	s.NoError(mockDB.ExpectationsWereMet())
	s.Equal(int64(42), count)
}

func (s *StoragePostgresSuite) TestClaimBatchPartitioned() {
	claimColumns := []string{
		"id", "event_type", "aggregate_type", "aggregate_id", "aggregate_user_id",
		"payload", "metadata", "attempts", "max_attempts", "occurred_at",
	}

	userID := uuid.NewString()
	userID2 := uuid.NewString()
	eventID := uuid.NewString()
	eventID2 := uuid.NewString()
	occurred := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	occurred2 := time.Date(2026, 7, 1, 10, 1, 0, 0, time.UTC)
	meta := []byte(`{"traceparent":"00-abc"}`)

	type scenario struct {
		name   string
		setup  func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func())
		expect func([]outbox.Row, error)
	}

	scenarios := []scenario{
		{
			name: "deve adiar e retornar nil em colisao 23505",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.NoError(err)
				s.Nil(rows)
			},
		},
		{
			name: "deve retornar nil para batch vazio via sqlmock",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnRows(sqlmock.NewRows(claimColumns))
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.NoError(err)
				s.Nil(rows)
			},
		},
		{
			name: "deve parsear evento com aggregate_user_id",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
						eventID, "test.event", "Aggregate", "agg-1", userID,
						[]byte(`{"k":"v"}`), meta, 0, 5, occurred,
					))
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.NoError(err)
				s.Require().Len(rows, 1)
				r := rows[0]
				s.Equal(eventID, r.ID)
				s.Equal("test.event", r.Type)
				s.Equal(userID, r.AggregateUserID)
				s.Equal(occurred, r.OccurredAt)
				s.Equal("00-abc", r.Metadata["traceparent"])
			},
		},
		{
			name: "deve parsear evento sistemico sem aggregate_user_id",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnRows(sqlmock.NewRows(claimColumns).AddRow(
						eventID, "system.event", "System", "sys-1", nil,
						[]byte(`{}`), []byte(`{}`), 0, 3, occurred,
					))
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.NoError(err)
				s.Require().Len(rows, 1)
				s.Empty(rows[0].AggregateUserID)
			},
		},
		{
			name: "deve respeitar ordenacao por occurred_at com usuarios distintos",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnRows(sqlmock.NewRows(claimColumns).
						AddRow(eventID, "test.event", "Agg", "agg-1", userID, []byte(`{}`), []byte(`{}`), 0, 5, occurred).
						AddRow(eventID2, "test.event", "Agg", "agg-2", userID2, []byte(`{}`), []byte(`{}`), 0, 5, occurred2),
					)
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.NoError(err)
				s.Require().Len(rows, 2)
				s.Equal(eventID, rows[0].ID)
				s.Equal(eventID2, rows[1].ID)
				s.True(rows[0].OccurredAt.Before(rows[1].OccurredAt))
			},
		},
		{
			name: "deve propagar erro nao 23505",
			setup: func() (outbox.OutboxRepository, *sqlmock.Sqlmock, func()) {
				db, mockDB, err := sqlmock.New()
				s.Require().NoError(err)
				mockDB.ExpectQuery("WITH claimable").
					WithArgs("worker-1", 10).
					WillReturnError(errors.New("connection reset"))
				return outbox.NewPostgresStorage(db), &mockDB, func() { _ = db.Close() }
			},
			expect: func(rows []outbox.Row, err error) {
				s.Error(err)
				s.Nil(rows)
			},
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			storage, mockDB, cleanup := sc.setup()
			defer cleanup()
			rows, err := storage.ClaimBatch(context.Background(), "worker-1", 10)
			sc.expect(rows, err)
			if mockDB != nil {
				s.NoError((*mockDB).ExpectationsWereMet())
			}
		})
	}
}
