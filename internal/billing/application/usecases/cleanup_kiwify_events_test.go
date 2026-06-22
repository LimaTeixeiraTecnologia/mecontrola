package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases/mocks"
)

type CleanupKiwifyEventsSuite struct {
	suite.Suite
	ctx      context.Context
	obs      *fake.Provider
	repoMock *mocks.KiwifyEventRepository
}

func TestCleanupKiwifyEvents(t *testing.T) {
	suite.Run(t, new(CleanupKiwifyEventsSuite))
}

func (s *CleanupKiwifyEventsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = mocks.NewKiwifyEventRepository(s.T())
}

func (s *CleanupKiwifyEventsSuite) TestExecute() {
	errDB := errors.New("db error")

	type dependencies struct {
		repoMock *mocks.KiwifyEventRepository
	}

	scenarios := []struct {
		name         string
		cfg          configs.BillingConfig
		dependencies dependencies
		expect       func(error)
	}{
		{
			name: "deve deletar eventos antigos com sucesso",
			cfg: configs.BillingConfig{
				KiwifyEventsRetentionDays:     30,
				KiwifyEventsHousekeepingBatch: 100,
			},
			dependencies: func() dependencies {
				s.repoMock.EXPECT().
					DeleteOlderThan(mock.Anything, mock.AnythingOfType("time.Time"), 100).
					Return(int64(10), nil).
					Once()
				s.repoMock.EXPECT().
					DeleteOlderThan(mock.Anything, mock.AnythingOfType("time.Time"), 100).
					Return(int64(0), nil).
					Once()
				return dependencies{repoMock: s.repoMock}
			}(),
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
			dependencies: func() dependencies {
				s.repoMock.EXPECT().
					DeleteOlderThan(mock.Anything, mock.AnythingOfType("time.Time"), 100).
					Return(int64(0), errDB).
					Once()
				return dependencies{repoMock: s.repoMock}
			}(),
			expect: func(err error) {
				s.Require().Error(err)
				s.Require().True(errors.Is(err, errDB))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewCleanupKiwifyEvents(scenario.dependencies.repoMock, scenario.cfg, s.obs)
			err := sut.Execute(s.ctx)
			scenario.expect(err)
		})
	}
}
