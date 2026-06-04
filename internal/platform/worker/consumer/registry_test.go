package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/worker/consumer"
)

type RegistrySuite struct {
	suite.Suite
	registry consumer.Registry
}

func (s *RegistrySuite) SetupTest() {
	s.registry = consumer.NewRegistry()
}

func TestRegistrySuite(t *testing.T) {
	suite.Run(t, new(RegistrySuite))
}

func (s *RegistrySuite) TestRegistrar_Sucesso() {
	h := consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return nil })
	err := s.registry.Register(consumer.Registration{Name: "test", EventType: "order.created", Handler: h})
	s.NoError(err)
}

func (s *RegistrySuite) TestRegistrar_HandlerNil() {
	err := s.registry.Register(consumer.Registration{Name: "test", EventType: "order.created", Handler: nil})
	s.Error(err)
}

func (s *RegistrySuite) TestRegistrar_EventTypeDuplicado() {
	h := consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return nil })
	reg := consumer.Registration{Name: "test", EventType: "order.created", Handler: h}
	s.NoError(s.registry.Register(reg))
	err := s.registry.Register(reg)
	s.Error(err)
}

func (s *RegistrySuite) TestDespachar_Sucesso_ParamsBodyCorretos() {
	var gotParams map[string]string
	var gotBody []byte
	h := consumer.HandlerFunc(func(_ context.Context, params map[string]string, body []byte) error {
		gotParams = params
		gotBody = body
		return nil
	})
	s.NoError(s.registry.Register(consumer.Registration{Name: "t", EventType: "evt", Handler: h}))

	err := s.registry.Dispatch(context.Background(), "evt", map[string]string{"k": "v"}, []byte("payload"))
	s.NoError(err)
	s.Equal(map[string]string{"k": "v"}, gotParams)
	s.Equal([]byte("payload"), gotBody)
}

func (s *RegistrySuite) TestDespachar_EventTypeDesconhecido() {
	err := s.registry.Dispatch(context.Background(), "nao-existe", nil, nil)
	s.Error(err)
}

func (s *RegistrySuite) TestDespachar_PropagaErroDoHandler() {
	sentinel := errors.New("handler error")
	h := consumer.HandlerFunc(func(_ context.Context, _ map[string]string, _ []byte) error { return sentinel })
	s.NoError(s.registry.Register(consumer.Registration{Name: "t", EventType: "evt", Handler: h}))

	err := s.registry.Dispatch(context.Background(), "evt", nil, nil)
	s.ErrorIs(err, sentinel)
}
