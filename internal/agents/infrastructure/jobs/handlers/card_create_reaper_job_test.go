package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type mockCardCreateReaper struct {
	mock.Mock
}

func (m *mockCardCreateReaper) Reap(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

type CardCreateReaperJobSuite struct {
	suite.Suite
	ctx        context.Context
	reaperMock *mockCardCreateReaper
}

func TestCardCreateReaperJobSuite(t *testing.T) {
	suite.Run(t, new(CardCreateReaperJobSuite))
}

func (s *CardCreateReaperJobSuite) SetupTest() {
	s.ctx = context.Background()
	s.reaperMock = &mockCardCreateReaper{}
	s.reaperMock.Test(s.T())
	s.T().Cleanup(func() { s.reaperMock.AssertExpectations(s.T()) })
}

func (s *CardCreateReaperJobSuite) TestRun() {
	type dependencies struct {
		reaper *mockCardCreateReaper
	}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve executar reap com sucesso",
			dependencies: dependencies{
				reaper: func() *mockCardCreateReaper {
					s.reaperMock.On("Reap", s.ctx).Return(int64(3), nil).Once()
					return s.reaperMock
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro quando reap falha",
			dependencies: dependencies{
				reaper: func() *mockCardCreateReaper {
					s.reaperMock.On("Reap", s.ctx).Return(int64(0), errors.New("store indisponivel")).Once()
					return s.reaperMock
				}(),
			},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "store indisponivel")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			job := NewCardCreateReaperJob("agents-card-create-reaper", scenario.dependencies.reaper, "")
			err := job.Run(s.ctx)
			scenario.expect(err)
		})
	}
}

func (s *CardCreateReaperJobSuite) TestNameScheduleTimeout() {
	job := NewCardCreateReaperJob("", s.reaperMock, "")
	s.Equal("agents-card-create-reaper", job.Name())
	s.Equal("*/5 * * * *", job.Schedule())
	s.NotZero(job.Timeout())

	named := NewCardCreateReaperJob("custom-name", s.reaperMock, "0 0 * * *")
	s.Equal("custom-name", named.Name())
	s.Equal("0 0 * * *", named.Schedule())
}
