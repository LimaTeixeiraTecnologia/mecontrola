package outbox_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type InstanceIDSuite struct {
	suite.Suite
}

func TestInstanceID(t *testing.T) {
	suite.Run(t, new(InstanceIDSuite))
}

func (s *InstanceIDSuite) SetupTest() {}

func (s *InstanceIDSuite) TestNewEventGeneratesUniqueIDs() {
	type args struct {
		aggregateID string
	}

	scenarios := []struct {
		name   string
		args   []args
		setup  func()
		expect func([]outbox.Event, []error)
	}{
		{
			name:  "deve gerar ids validos e unicos",
			args:  []args{{aggregateID: "1"}, {aggregateID: "2"}},
			setup: func() {},
			expect: func(events []outbox.Event, errs []error) {
				s.Require().NoError(errs[0])
				s.Require().NoError(errs[1])
				s.NotEqual(events[0].ID, events[1].ID)
				_, firstErr := uuid.Parse(events[0].ID)
				_, secondErr := uuid.Parse(events[1].ID)
				s.NoError(firstErr)
				s.NoError(secondErr)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			results := make([]outbox.Event, 0, len(scenario.args))
			errs := make([]error, 0, len(scenario.args))
			for _, arg := range scenario.args {
				sut := outbox.NewEvent
				event, err := sut(outbox.EventInput{
					Type:          "test.event",
					AggregateType: "aggregate",
					AggregateID:   arg.aggregateID,
					Payload:       []byte(`{}`),
				})
				results = append(results, event)
				errs = append(errs, err)
			}

			scenario.expect(results, errs)
		})
	}
}
