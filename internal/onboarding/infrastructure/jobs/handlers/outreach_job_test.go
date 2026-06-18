package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appinterfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type OutreachJobSuite struct {
	suite.Suite
}

func TestOutreachJobSuite(t *testing.T) {
	suite.Run(t, new(OutreachJobSuite))
}

func (s *OutreachJobSuite) SetupTest() {}

func (s *OutreachJobSuite) TestOutreachJob_Scenarios() {
	scenarios := []struct {
		name          string
		enabled       bool
		cancelContext bool
	}{
		{
			name:          "Enabled job runs successfully",
			enabled:       true,
			cancelContext: false,
		},
		{
			name:          "Disabled job skips execution",
			enabled:       false,
			cancelContext: false,
		},
		{
			name:          "Canceled context returns no error",
			enabled:       true,
			cancelContext: true,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			tokenRepo := appinterfacesmocks.NewMagicTokenRepository(s.T())
			waGW := appinterfacesmocks.NewOutreachChannelGateway(s.T())
			cipher := appinterfacesmocks.NewTokenCipher(s.T())

			tokenRepo.EXPECT().FindPaidForOutreach(mock.Anything, mock.Anything, mock.Anything).
				Return(nil, nil).Maybe()

			uc := usecases.NewSendOutreach(
				tokenRepo,
				waGW,
				cipher,
				id.NewUUIDGenerator(),
				"activation_reminder",
				2*time.Hour,
				noop.NewProvider(),
			)

			job := handlers.NewOutreachJob(uc, scenario.enabled)

			s.Equal("onboarding.outreach_job", job.Name())
			s.NotEmpty(job.Schedule())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if scenario.cancelContext {
				cancel()
			}

			err := job.Run(ctx)
			s.NoError(err)
		})
	}
}
