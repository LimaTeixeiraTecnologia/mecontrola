package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
)

type CleanupNomatchThrottleSuite struct {
	suite.Suite
	ctx context.Context
}

func TestCleanupNomatchThrottleSuite(t *testing.T) {
	suite.Run(t, new(CleanupNomatchThrottleSuite))
}

func (s *CleanupNomatchThrottleSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *CleanupNomatchThrottleSuite) TestExecute() {
	type dependencies struct {
		throttle *mocks.NoMatchThrottle
	}

	scenarios := []struct {
		name         string
		cfg          configs.OnboardingConfig
		dependencies dependencies
		expect       func(err error)
	}{
		{
			name: "deve apagar registros e retornar nil",
			cfg:  configs.OnboardingConfig{ActivationNoMatchThrottleRetentionDays: 7, ActivationNoMatchThrottleBatch: 100},
			dependencies: dependencies{
				throttle: func() *mocks.NoMatchThrottle {
					m := mocks.NewNoMatchThrottle(s.T())
					m.EXPECT().
						DeleteBefore(mock.Anything, mock.AnythingOfType("time.Time"), 100).
						Return(int64(5), nil).
						Once()
					m.EXPECT().
						DeleteBefore(mock.Anything, mock.AnythingOfType("time.Time"), 100).
						Return(int64(0), nil).
						Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve usar defaults quando config zerada",
			cfg:  configs.OnboardingConfig{},
			dependencies: dependencies{
				throttle: func() *mocks.NoMatchThrottle {
					m := mocks.NewNoMatchThrottle(s.T())
					m.EXPECT().
						DeleteBefore(mock.Anything, mock.AnythingOfType("time.Time"), nomatchThrottleDefaultBatchSize).
						Return(int64(0), nil).
						Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve retornar erro de infraestrutura",
			cfg:  configs.OnboardingConfig{ActivationNoMatchThrottleRetentionDays: 7, ActivationNoMatchThrottleBatch: 100},
			dependencies: dependencies{
				throttle: func() *mocks.NoMatchThrottle {
					m := mocks.NewNoMatchThrottle(s.T())
					m.EXPECT().
						DeleteBefore(mock.Anything, mock.AnythingOfType("time.Time"), 100).
						Return(int64(0), errors.New("db error")).
						Once()
					return m
				}(),
			},
			expect: func(err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := NewCleanupNomatchThrottle(scenario.dependencies.throttle, scenario.cfg, fake.NewProvider())
			err := uc.Execute(s.ctx)
			scenario.expect(err)
		})
	}
}
