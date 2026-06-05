package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	dbmocks "github.com/JailtonJunior94/devkit-go/pkg/database/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type StoragePostgresSuite struct {
	suite.Suite
	dbtx    *dbmocks.MockDBTX
	rows    *dbmocks.MockRows
	result  *dbmocks.MockResult
	storage OutboxRepository
}

func TestStoragePostgres(t *testing.T) {
	suite.Run(t, new(StoragePostgresSuite))
}

func (s *StoragePostgresSuite) SetupTest() {
	s.dbtx = dbmocks.NewMockDBTX(s.T())
	s.rows = dbmocks.NewMockRows(s.T())
	s.result = dbmocks.NewMockResult(s.T())
	s.storage = NewPostgresStorage(s.dbtx)
}

func (s *StoragePostgresSuite) TestInsert_Sucesso() {
	ctx := context.Background()
	evt := mustNewEvent(s.T())

	meta, err := marshalMetadata(evt.Metadata)
	s.Require().NoError(err)

	s.dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"),
		evt.ID, evt.Type, evt.AggregateType, evt.AggregateID,
		evt.Payload, meta,
		int(StatusPending), 15, evt.OccurredAt, evt.OccurredAt,
	).Return(s.result, nil)

	s.NoError(s.storage.Insert(ctx, evt, 15))
}

func (s *StoragePostgresSuite) TestInsert_ErroExec() {
	ctx := context.Background()
	evt := mustNewEvent(s.T())
	dbErr := errors.New("db error")

	s.dbtx.EXPECT().ExecContext(mock.Anything, mock.AnythingOfType("string"), mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, dbErr)

	s.Error(s.storage.Insert(ctx, evt, 15))
}

func (s *StoragePostgresSuite) TestClaimBatch_VazioRetornaNil() {
	ctx := context.Background()

	emptyRows := dbmocks.NewMockRows(s.T())
	emptyRows.EXPECT().Next().Return(false)
	emptyRows.EXPECT().Err().Return(nil)
	emptyRows.EXPECT().Close().Return(nil)

	s.dbtx.EXPECT().QueryContext(ctx, mock.AnythingOfType("string"), int(StatusPending), 50).Return(emptyRows, nil)

	result, err := s.storage.ClaimBatch(ctx, "inst-1", 50)
	s.NoError(err)
	s.Nil(result)
}

func (s *StoragePostgresSuite) TestMarkPublished_Sucesso() {
	ctx := context.Background()
	id := uuid.NewString()

	s.dbtx.EXPECT().ExecContext(ctx, mock.AnythingOfType("string"), int(StatusPublished), id).Return(s.result, nil)

	s.NoError(s.storage.MarkPublished(ctx, id))
}

func (s *StoragePostgresSuite) TestMarkFailed_Sucesso() {
	ctx := context.Background()
	id := uuid.NewString()
	lastErr := "handler failed"

	s.dbtx.EXPECT().ExecContext(ctx, mock.AnythingOfType("string"), int(StatusFailed), lastErr, id).Return(s.result, nil)

	s.NoError(s.storage.MarkFailed(ctx, id, lastErr))
}

func (s *StoragePostgresSuite) TestMarkPendingRetry_Sucesso() {
	ctx := context.Background()
	id := uuid.NewString()
	lastErr := "transient"
	next := time.Now().UTC().Add(5 * time.Second)

	s.dbtx.EXPECT().ExecContext(ctx, mock.AnythingOfType("string"), int(StatusPending), lastErr, next, id).Return(s.result, nil)

	s.NoError(s.storage.MarkPendingRetry(ctx, id, lastErr, next))
}

func (s *StoragePostgresSuite) TestResetStuck_Sucesso() {
	ctx := context.Background()
	stuckAfter := 5 * time.Minute

	s.result.EXPECT().RowsAffected().Return(int64(3), nil)
	s.dbtx.EXPECT().ExecContext(ctx, mock.AnythingOfType("string"),
		int(StatusPending), int(StatusProcessing), stuckAfter.Microseconds(),
	).Return(s.result, nil)

	n, err := s.storage.ResetStuck(ctx, stuckAfter)
	s.NoError(err)
	s.Equal(int64(3), n)
}

func (s *StoragePostgresSuite) TestDeletePublishedBatch_Sucesso() {
	ctx := context.Background()
	retention := 90 * 24 * time.Hour

	s.result.EXPECT().RowsAffected().Return(int64(1000), nil)
	s.dbtx.EXPECT().ExecContext(ctx, mock.AnythingOfType("string"),
		int(StatusPublished), retention.Microseconds(), 1000,
	).Return(s.result, nil)

	n, err := s.storage.DeletePublishedBatch(ctx, retention, 1000)
	s.NoError(err)
	s.Equal(int64(1000), n)
}

func mustNewEvent(t interface{ Helper() }) Event {
	t.Helper()
	evt, _ := NewEvent(EventInput{
		Type:          "test.event",
		AggregateType: "TestAggregate",
		AggregateID:   "agg-1",
		Payload:       []byte(`{"x":1}`),
	})
	return evt
}
