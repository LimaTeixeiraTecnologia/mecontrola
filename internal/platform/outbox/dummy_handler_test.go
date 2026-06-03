package outbox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type DummyHandlerSuite struct {
	suite.Suite
}

func TestDummyHandler(t *testing.T) {
	suite.Run(t, new(DummyHandlerSuite))
}

func (s *DummyHandlerSuite) TestDummyHandlerUsesLogAllowlist() {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	evtID, err := events.NewEventID("01JTEST000000000000000004")
	s.Require().NoError(err)
	evtType, err := events.NewEventName("platform.outbox-dummy")
	s.Require().NoError(err)
	evt, err := outbox.NewEvent(outbox.NewEventParams{
		ID:            evtID,
		EventType:     evtType,
		AggregateType: "sensitive-aggregate",
		AggregateID:   "sensitive-id",
		Payload:       json.RawMessage(`{"secret":"not logged"}`),
	})
	s.Require().NoError(err)

	err = outbox.DummyHandler(context.Background(), evt)
	s.Require().NoError(err)

	output := buf.String()
	s.Contains(output, "event_id")
	s.Contains(output, "event_type")
	s.NotContains(output, "aggregate_type")
	s.NotContains(output, "aggregate_id")
	s.NotContains(output, "sensitive-id")
	s.NotContains(output, "secret")
}
