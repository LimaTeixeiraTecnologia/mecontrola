package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

type CleanupKiwifyEventsSuite struct {
	suite.Suite
	ctx      context.Context
	repoMock *mocks.KiwifyEventRepository
}

func TestCleanupKiwifyEvents(t *testing.T) {
	suite.Run(t, new(CleanupKiwifyEventsSuite))
}

func (s *CleanupKiwifyEventsSuite) SetupTest() {
	s.ctx = context.Background()
	s.repoMock = mocks.NewKiwifyEventRepository(s.T())
}

func (s *CleanupKiwifyEventsSuite) newSUT(cfg configs.BillingConfig) *usecases.CleanupKiwifyEvents {
	return usecases.NewCleanupKiwifyEvents(s.repoMock, cfg, noop.NewProvider())
}

func (s *CleanupKiwifyEventsSuite) TestExecute() {
	errDB := errors.New("db error")

	scenarios := []struct {
		name   string
		cfg    configs.BillingConfig
		setup  func()
		expect func(error)
	}{
		{
			name: "deve deletar eventos antigos com sucesso",
			cfg: configs.BillingConfig{
				KiwifyEventsRetentionDays:     30,
				KiwifyEventsHousekeepingBatch: 100,
			},
			setup: func() {
				s.repoMock.EXPECT().
					DeleteOlderThan(s.ctx, mock.AnythingOfType("time.Time"), 100).
					Return(int64(10), nil).
					Once()
				s.repoMock.EXPECT().
					DeleteOlderThan(s.ctx, mock.AnythingOfType("time.Time"), 100).
					Return(int64(0), nil).
					Once()
			},
			expect: func(err error) {
				s.Require().NoError(err)
			},
		},
		{
			name: "deve retornar erro quando repositorio falhar",
			cfg: configs.BillingConfig{
				KiwifyEventsRetentionDays:     30,
				KiwifyEventsHousekeepingBatch: 100,
			},
			setup: func() {
				s.repoMock.EXPECT().
					DeleteOlderThan(s.ctx, mock.AnythingOfType("time.Time"), 100).
					Return(int64(0), errDB).
					Once()
			},
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().True(errors.Is(err, errDB))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			s.SetupTest()
			sut := s.newSUT(scenario.cfg)
			scenario.setup()

			err := sut.Execute(s.ctx)

			scenario.expect(err)
		})
	}
}
