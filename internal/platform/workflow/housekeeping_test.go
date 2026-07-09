package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type noopLogger struct{}

func (noopLogger) Debug(context.Context, string, ...observability.Field) {}
func (noopLogger) Info(context.Context, string, ...observability.Field)  {}
func (noopLogger) Warn(context.Context, string, ...observability.Field)  {}
func (noopLogger) Error(context.Context, string, ...observability.Field) {}
func (noopLogger) With(...observability.Field) observability.Logger      { return noopLogger{} }

type unitOfWorkVoid struct{}

func (u *unitOfWorkVoid) DBTX() database.DBTX { return nil }
func (u *unitOfWorkVoid) Do(ctx context.Context, fn func(context.Context, database.DBTX) error) error {
	return fn(ctx, nil)
}

type deleteCompletedResult struct {
	n   int64
	err error
}

type fakeHousekeepingStore struct {
	results []deleteCompletedResult
	idx     int
}

func (f *fakeHousekeepingStore) Insert(_ context.Context, _ Snapshot) error { return nil }
func (f *fakeHousekeepingStore) Load(_ context.Context, _, _ string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}
func (f *fakeHousekeepingStore) LoadLatest(_ context.Context, _, _ string) (Snapshot, bool, error) {
	return Snapshot{}, false, nil
}
func (f *fakeHousekeepingStore) Save(_ context.Context, _ Snapshot, _ int64) error { return nil }
func (f *fakeHousekeepingStore) AppendStep(_ context.Context, _ StepRecord) error  { return nil }
func (f *fakeHousekeepingStore) ListSuspended(_ context.Context, _ string, _ time.Time, _ int) ([]Snapshot, error) {
	return nil, nil
}
func (f *fakeHousekeepingStore) DeleteCompleted(_ context.Context, _ time.Duration, _ int) (int64, error) {
	if f.idx >= len(f.results) {
		return 0, nil
	}
	r := f.results[f.idx]
	f.idx++
	return r.n, r.err
}

type fakeStoreFactory struct {
	store Store
}

func (f *fakeStoreFactory) Store(_ database.DBTX) Store { return f.store }

type HousekeepingJobSuite struct {
	suite.Suite
	ctx context.Context
	cfg configs.WorkflowKernelConfig
}

func TestHousekeepingJobSuite(t *testing.T) {
	suite.Run(t, new(HousekeepingJobSuite))
}

func (s *HousekeepingJobSuite) SetupTest() {
	s.ctx = context.Background()
	s.cfg = configs.WorkflowKernelConfig{
		HousekeepingRetentionDays: 30,
		HousekeepingSchedule:      "@daily",
		HousekeepingBatchSize:     100,
	}
}

func (s *HousekeepingJobSuite) TestRun() {
	type args struct {
		ctx context.Context
	}
	type dependencies struct {
		factory StoreFactory
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve deletar lotes ate o limite",
			args: args{ctx: context.Background()},
			dependencies: dependencies{
				factory: func() StoreFactory {
					store := &fakeHousekeepingStore{results: []deleteCompletedResult{
						{n: 100, err: nil},
						{n: 50, err: nil},
						{n: 0, err: nil},
					}}
					return &fakeStoreFactory{store: store}
				}(),
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve concluir sem runs completados",
			args: args{ctx: context.Background()},
			dependencies: dependencies{
				factory: func() StoreFactory {
					store := &fakeHousekeepingStore{results: []deleteCompletedResult{
						{n: 0, err: nil},
					}}
					return &fakeStoreFactory{store: store}
				}(),
			},
			expect: func(err error) { s.NoError(err) },
		},
		{
			name: "deve retornar erro ao falhar no delete",
			args: args{ctx: context.Background()},
			dependencies: dependencies{
				factory: func() StoreFactory {
					store := &fakeHousekeepingStore{results: []deleteCompletedResult{
						{n: 0, err: errors.New("db error")},
					}}
					return &fakeStoreFactory{store: store}
				}(),
			},
			expect: func(err error) { s.Error(err) },
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut, newErr := NewHousekeepingJob(&unitOfWorkVoid{}, scenario.dependencies.factory, s.cfg, noopLogger{})
			s.Require().NoError(newErr)
			err := sut.Run(scenario.args.ctx)
			scenario.expect(err)
		})
	}
}

func (s *HousekeepingJobSuite) TestNewHousekeepingJob_InvalidConfig() {
	type args struct {
		cfg configs.WorkflowKernelConfig
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(job *HousekeepingJob, err error)
	}{
		{
			name: "deve rejeitar retention_days zero",
			args: args{cfg: configs.WorkflowKernelConfig{
				HousekeepingRetentionDays: 0,
				HousekeepingBatchSize:     100,
				HousekeepingSchedule:      "@daily",
			}},
			expect: func(job *HousekeepingJob, err error) {
				s.Error(err)
				s.Nil(job)
			},
		},
		{
			name: "deve rejeitar retention_days negativo",
			args: args{cfg: configs.WorkflowKernelConfig{
				HousekeepingRetentionDays: -1,
				HousekeepingBatchSize:     100,
				HousekeepingSchedule:      "@daily",
			}},
			expect: func(job *HousekeepingJob, err error) {
				s.Error(err)
				s.Nil(job)
			},
		},
		{
			name: "deve rejeitar batch_size zero",
			args: args{cfg: configs.WorkflowKernelConfig{
				HousekeepingRetentionDays: 30,
				HousekeepingBatchSize:     0,
				HousekeepingSchedule:      "@daily",
			}},
			expect: func(job *HousekeepingJob, err error) {
				s.Error(err)
				s.Nil(job)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			job, err := NewHousekeepingJob(&unitOfWorkVoid{}, &fakeStoreFactory{store: &fakeHousekeepingStore{}}, scenario.args.cfg, noopLogger{})
			scenario.expect(job, err)
		})
	}
}

func (s *HousekeepingJobSuite) TestMetadata() {
	sut, err := NewHousekeepingJob(&unitOfWorkVoid{}, &fakeStoreFactory{store: &fakeHousekeepingStore{}}, s.cfg, noopLogger{})
	s.Require().NoError(err)
	s.Equal("workflow-kernel-housekeeping", sut.Name())
	s.Equal("@daily", sut.Schedule())
	s.Equal(5*time.Minute, sut.Timeout())
}
