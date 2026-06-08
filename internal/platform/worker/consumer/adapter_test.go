package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
	consumermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer/mocks"
)

type AdapterSuite struct {
	suite.Suite
}

func TestAdapterSuite(t *testing.T) {
	suite.Run(t, new(AdapterSuite))
}

func (s *AdapterSuite) SetupTest() {}

func (s *AdapterSuite) TestMetadata() {
	type args struct {
		name       string
		technology string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumermocks.Runner)
		expect func(*consumer.Adapter)
	}{
		{
			name:  "deve expor nome e tecnologia configurados",
			args:  args{name: "billing", technology: "kafka"},
			setup: func(*consumermocks.Runner) {},
			expect: func(adapter *consumer.Adapter) {
				s.Equal("billing", adapter.Name())
				s.Equal("kafka", adapter.Technology())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			runner := consumermocks.NewRunner(s.T())
			scenario.setup(runner)

			sut := consumer.NewAdapter(scenario.args.name, scenario.args.technology, runner)

			scenario.expect(sut)
		})
	}
}

func (s *AdapterSuite) TestDelegation() {
	type args struct {
		ctx context.Context
	}

	sentinel := errors.New("stop err")

	scenarios := []struct {
		name   string
		args   args
		setup  func(*consumermocks.Runner)
		expect func(*consumer.Adapter, error)
		act    func(*consumer.Adapter, context.Context) error
	}{
		{
			name: "deve delegar start para runner",
			args: args{ctx: context.Background()},
			setup: func(runner *consumermocks.Runner) {
				runner.EXPECT().Start(context.Background()).Return(nil).Once()
			},
			act: func(adapter *consumer.Adapter, ctx context.Context) error {
				return adapter.Start(ctx)
			},
			expect: func(_ *consumer.Adapter, err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do stop do runner",
			args: args{ctx: context.Background()},
			setup: func(runner *consumermocks.Runner) {
				runner.EXPECT().Stop(context.Background()).Return(sentinel).Once()
			},
			act: func(adapter *consumer.Adapter, ctx context.Context) error {
				return adapter.Stop(ctx)
			},
			expect: func(_ *consumer.Adapter, err error) {
				s.ErrorIs(err, sentinel)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			runner := consumermocks.NewRunner(s.T())
			scenario.setup(runner)

			sut := consumer.NewAdapter("test", "fake", runner)
			err := scenario.act(sut, scenario.args.ctx)

			scenario.expect(sut, err)
		})
	}
}
