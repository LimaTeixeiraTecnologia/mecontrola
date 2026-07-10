package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/status"
)

type DeliveryStateSuite struct {
	suite.Suite
}

func TestDeliveryStateSuite(t *testing.T) {
	suite.Run(t, new(DeliveryStateSuite))
}

func (s *DeliveryStateSuite) TestDecideDeliveryState() {
	scenarios := []struct {
		name   string
		total  int
		failed int
		expect status.MessageDeliveryState
	}{
		{name: "sem status recebido", total: 0, failed: 0, expect: status.DeliveryStateNotReceived},
		{name: "total negativo trata como nao recebido", total: -1, failed: 0, expect: status.DeliveryStateNotReceived},
		{name: "com falha", total: 2, failed: 1, expect: status.DeliveryStateFailed},
		{name: "somente falha", total: 1, failed: 1, expect: status.DeliveryStateFailed},
		{name: "entregue sem falha", total: 3, failed: 0, expect: status.DeliveryStateDelivered},
	}
	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.Equal(scenario.expect, status.DecideDeliveryState(scenario.total, scenario.failed))
		})
	}
}

func (s *DeliveryStateSuite) TestIsValid() {
	s.True(status.DeliveryStateNotReceived.IsValid())
	s.True(status.DeliveryStateFailed.IsValid())
	s.True(status.DeliveryStateDelivered.IsValid())
	s.False(status.MessageDeliveryState("bogus").IsValid())
}

type fakeDeliveryReader struct {
	counts status.DeliveryCounts
	err    error
}

func (f *fakeDeliveryReader) DeliveryCounts(_ context.Context, _ string) (status.DeliveryCounts, error) {
	return f.counts, f.err
}

func (s *DeliveryStateSuite) TestLookupDeliveryState_NotReceived() {
	uc := status.NewLookupDeliveryState(&fakeDeliveryReader{counts: status.DeliveryCounts{Total: 0}}, noop.NewProvider())
	state, err := uc.Execute(context.Background(), "wamid-1")
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateNotReceived, state)
}

func (s *DeliveryStateSuite) TestLookupDeliveryState_Failed() {
	uc := status.NewLookupDeliveryState(&fakeDeliveryReader{counts: status.DeliveryCounts{Total: 2, Failed: 1}}, noop.NewProvider())
	state, err := uc.Execute(context.Background(), "wamid-2")
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateFailed, state)
}

func (s *DeliveryStateSuite) TestLookupDeliveryState_Delivered() {
	uc := status.NewLookupDeliveryState(&fakeDeliveryReader{counts: status.DeliveryCounts{Total: 1}}, noop.NewProvider())
	state, err := uc.Execute(context.Background(), "wamid-3")
	s.Require().NoError(err)
	s.Equal(status.DeliveryStateDelivered, state)
}

func (s *DeliveryStateSuite) TestLookupDeliveryState_EmptyMessageID() {
	uc := status.NewLookupDeliveryState(&fakeDeliveryReader{}, noop.NewProvider())
	state, err := uc.Execute(context.Background(), "")
	s.Require().Error(err)
	s.ErrorIs(err, status.ErrEmptyMessageID)
	s.Equal(status.DeliveryStateNotReceived, state)
}

func (s *DeliveryStateSuite) TestLookupDeliveryState_ReaderError() {
	uc := status.NewLookupDeliveryState(&fakeDeliveryReader{err: errors.New("pg down")}, noop.NewProvider())
	state, err := uc.Execute(context.Background(), "wamid-4")
	s.Require().Error(err)
	s.Contains(err.Error(), "pg down")
	s.Equal(status.DeliveryStateNotReceived, state)
}
