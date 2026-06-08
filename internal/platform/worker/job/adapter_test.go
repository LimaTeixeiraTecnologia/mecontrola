package job_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/job"
)

type AdapterSuite struct {
	suite.Suite
}

func TestAdapterSuite(t *testing.T) {
	suite.Run(t, new(AdapterSuite))
}

func (s *AdapterSuite) SetupTest() {}

func (s *AdapterSuite) TestAdapter() {
	type args struct {
		name     string
		schedule string
	}

	sentinel := errors.New("erro sentinel")

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		build  func(args) *job.Adapter
		act    func(*job.Adapter) error
		expect func(*job.Adapter, error)
	}{
		{
			name:  "deve usar politica skip por padrao",
			args:  args{name: "meu-job", schedule: "@hourly"},
			setup: func() {},
			build: func(input args) *job.Adapter {
				return job.NewAdapter(input.name, input.schedule, func(context.Context) error { return nil })
			},
			act: func(*job.Adapter) error { return nil },
			expect: func(adapter *job.Adapter, err error) {
				s.NoError(err)
				s.Equal("meu-job", adapter.Name())
				s.Equal("@hourly", adapter.Schedule())
				s.Equal(job.OverlapSkip, adapter.OverlapPolicy())
			},
		},
		{
			name:  "deve usar politica allow quando informada",
			args:  args{name: "meu-job", schedule: "@daily"},
			setup: func() {},
			build: func(input args) *job.Adapter {
				return job.NewAdapterWithPolicy(input.name, input.schedule, func(context.Context) error { return nil }, job.OverlapAllow)
			},
			act: func(*job.Adapter) error { return nil },
			expect: func(adapter *job.Adapter, err error) {
				s.NoError(err)
				s.Equal(job.OverlapAllow, adapter.OverlapPolicy())
			},
		},
		{
			name:  "deve delegar execucao da funcao",
			args:  args{name: "test", schedule: "@hourly"},
			setup: func() {},
			build: func(input args) *job.Adapter {
				return job.NewAdapter(input.name, input.schedule, func(context.Context) error { return sentinel })
			},
			act: func(adapter *job.Adapter) error {
				return adapter.Run(context.Background())
			},
			expect: func(_ *job.Adapter, err error) {
				s.ErrorIs(err, sentinel)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := scenario.build(scenario.args)
			err := scenario.act(sut)

			scenario.expect(sut, err)
		})
	}
}
