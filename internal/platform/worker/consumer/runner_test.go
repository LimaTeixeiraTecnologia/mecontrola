package consumer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
)

type RunnerSuite struct {
	suite.Suite
	logger *slog.Logger
}

func (s *RunnerSuite) SetupTest() {
	s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunnerSuite(t *testing.T) {
	suite.Run(t, new(RunnerSuite))
}

func (s *RunnerSuite) TestStart_PropagaErroDoSource() {
	sentinel := errors.New("source error")
	src := &fakeSource{startErr: sentinel}
	reg := consumer.NewRegistry()
	r := consumer.NewRunner(src, reg, s.logger)

	err := r.Start(context.Background())
	s.ErrorIs(err, sentinel)
}

func (s *RunnerSuite) TestStart_DespachaMensagens() {
	var dispatched []string
	src := &fakeSource{
		messages: []consumer.Message{
			{EventType: "evt1", Params: nil, Body: nil},
			{EventType: "evt2", Params: nil, Body: nil},
		},
	}
	reg := consumer.NewRegistry()
	for _, evt := range []string{"evt1", "evt2"} {
		evt := evt
		h := consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error {
			dispatched = append(dispatched, evt)
			return nil
		})
		s.NoError(reg.Register(consumer.Registration{Name: evt, EventType: evt, Handler: h}))
	}

	r := consumer.NewRunner(src, reg, s.logger)
	err := r.Start(context.Background())
	s.NoError(err)
	s.Equal([]string{"evt1", "evt2"}, dispatched)
}

func (s *RunnerSuite) TestStop_ChamaSourceStop() {
	src := &fakeSource{}
	reg := consumer.NewRegistry()
	r := consumer.NewRunner(src, reg, s.logger)

	err := r.Stop(context.Background())
	s.NoError(err)
	s.True(src.stopCalled)
}

func (s *RunnerSuite) TestStop_PropagaErroDoSourceStop() {
	sentinel := errors.New("stop error")
	src := &fakeSource{stopErr: sentinel}
	reg := consumer.NewRegistry()
	r := consumer.NewRunner(src, reg, s.logger)

	err := r.Stop(context.Background())
	s.ErrorIs(err, sentinel)
}

type fakeSource struct {
	startErr   error
	stopErr    error
	stopCalled bool
	messages   []consumer.Message
}

func (s *fakeSource) Start(_ context.Context, deliver func(context.Context, consumer.Message) error) error {
	if s.startErr != nil {
		return s.startErr
	}
	for _, msg := range s.messages {
		if err := deliver(context.Background(), msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakeSource) Stop(_ context.Context) error {
	s.stopCalled = true
	return s.stopErr
}
