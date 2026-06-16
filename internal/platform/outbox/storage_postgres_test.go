package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	dbmocks "github.com/JailtonJunior94/devkit-go/pkg/database/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type StoragePostgresSuite struct {
	suite.Suite
}

func TestStoragePostgres(t *testing.T) {
	suite.Run(t, new(StoragePostgresSuite))
}

func (s *StoragePostgresSuite) SetupTest() {}

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
				rows := dbmocks.NewMockRows(s.T())
				rows.EXPECT().Next().Return(false).Once()
				rows.EXPECT().Err().Return(nil).Once()
				rows.EXPECT().Close().Return(nil).Once()
				dbtx.EXPECT().QueryContext(input.ctx, mock.AnythingOfType("string"), int(outbox.StatusPending), 50).Return(rows, nil).Once()
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
