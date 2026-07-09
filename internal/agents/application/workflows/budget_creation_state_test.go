package workflows

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BudgetCreationStateSuite struct {
	suite.Suite
}

func TestBudgetCreationStateSuite(t *testing.T) {
	suite.Run(t, new(BudgetCreationStateSuite))
}

func (s *BudgetCreationStateSuite) TestBudgetAwaitingSlotRoundTrip() {
	type args struct {
		slot BudgetAwaitingSlot
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(str string, parsed BudgetAwaitingSlot, err error)
	}{
		{
			name: "total",
			args: args{slot: AwaitingBudgetTotal},
			expect: func(str string, parsed BudgetAwaitingSlot, err error) {
				s.Equal("total", str)
				s.NoError(err)
				s.Equal(AwaitingBudgetTotal, parsed)
				s.True(AwaitingBudgetTotal.IsValid())
			},
		},
		{
			name: "distribution",
			args: args{slot: AwaitingBudgetDistribution},
			expect: func(str string, parsed BudgetAwaitingSlot, err error) {
				s.Equal("distribution", str)
				s.NoError(err)
				s.Equal(AwaitingBudgetDistribution, parsed)
				s.True(AwaitingBudgetDistribution.IsValid())
			},
		},
		{
			name: "confirm",
			args: args{slot: AwaitingBudgetConfirm},
			expect: func(str string, parsed BudgetAwaitingSlot, err error) {
				s.Equal("confirm", str)
				s.NoError(err)
				s.Equal(AwaitingBudgetConfirm, parsed)
				s.True(AwaitingBudgetConfirm.IsValid())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			str := scenario.args.slot.String()
			parsed, err := ParseBudgetAwaitingSlot(str)
			scenario.expect(str, parsed, err)
		})
	}
}

func (s *BudgetCreationStateSuite) TestBudgetAwaitingSlotInvalid() {
	invalid := BudgetAwaitingSlot(0)
	s.False(invalid.IsValid())
	s.Equal("unknown", invalid.String())

	parsed, err := ParseBudgetAwaitingSlot("xpto")
	s.Error(err)
	s.Equal(BudgetAwaitingSlot(0), parsed)
	s.ErrorIs(err, errInvalidBudgetAwaitingSlot)
}

func (s *BudgetCreationStateSuite) TestBudgetCreationStatusRoundTrip() {
	type args struct {
		status BudgetCreationStatus
	}
	scenarios := []struct {
		name   string
		args   args
		expect func(str string, parsed BudgetCreationStatus, err error)
	}{
		{
			name: "active",
			args: args{status: BudgetCreationActive},
			expect: func(str string, parsed BudgetCreationStatus, err error) {
				s.Equal("active", str)
				s.NoError(err)
				s.Equal(BudgetCreationActive, parsed)
				s.True(BudgetCreationActive.IsValid())
			},
		},
		{
			name: "completed",
			args: args{status: BudgetCreationCompleted},
			expect: func(str string, parsed BudgetCreationStatus, err error) {
				s.Equal("completed", str)
				s.NoError(err)
				s.Equal(BudgetCreationCompleted, parsed)
				s.True(BudgetCreationCompleted.IsValid())
			},
		},
		{
			name: "cancelled",
			args: args{status: BudgetCreationCancelled},
			expect: func(str string, parsed BudgetCreationStatus, err error) {
				s.Equal("cancelled", str)
				s.NoError(err)
				s.Equal(BudgetCreationCancelled, parsed)
				s.True(BudgetCreationCancelled.IsValid())
			},
		},
		{
			name: "expired",
			args: args{status: BudgetCreationExpired},
			expect: func(str string, parsed BudgetCreationStatus, err error) {
				s.Equal("expired", str)
				s.NoError(err)
				s.Equal(BudgetCreationExpired, parsed)
				s.True(BudgetCreationExpired.IsValid())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			str := scenario.args.status.String()
			parsed, err := ParseBudgetCreationStatus(str)
			scenario.expect(str, parsed, err)
		})
	}
}

func (s *BudgetCreationStateSuite) TestBudgetCreationStatusInvalid() {
	invalid := BudgetCreationStatus(0)
	s.False(invalid.IsValid())
	s.Equal("unknown", invalid.String())

	parsed, err := ParseBudgetCreationStatus("xpto")
	s.Error(err)
	s.Equal(BudgetCreationStatus(0), parsed)
	s.ErrorIs(err, errInvalidBudgetCreationStatus)
}
