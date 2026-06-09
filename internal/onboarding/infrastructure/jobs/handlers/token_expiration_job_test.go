package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
	handlersjobmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers/mocks"
)

type TokenExpirationJobSuite struct {
	suite.Suite
}

func TestTokenExpirationJobSuite(t *testing.T) {
	suite.Run(t, new(TokenExpirationJobSuite))
}

func (s *TokenExpirationJobSuite) SetupTest() {}

func (s *TokenExpirationJobSuite) TestJobMetadata() {
	scenarios := []struct {
		name   string
		expect func(*handlers.TokenExpirationJob)
	}{
		{
			name: "deve expor nome configurado",
			expect: func(job *handlers.TokenExpirationJob) {
				s.Equal("onboarding-token-expiration", job.Name())
			},
		},
		{
			name: "deve expor cron configurado",
			expect: func(job *handlers.TokenExpirationJob) {
				s.Equal("0 3 * * *", job.Schedule())
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := handlersjobmocks.NewExpireTokensUseCase(s.T())
			job := handlers.NewTokenExpirationJob(useCase, "0 3 * * *")
			scenario.expect(job)
		})
	}
}

func (s *TokenExpirationJobSuite) TestRun() {
	scenarios := []struct {
		name   string
		setup  func(*handlersjobmocks.ExpireTokensUseCase)
		expect func(error)
	}{
		{
			name: "deve delegar para o use case",
			setup: func(useCase *handlersjobmocks.ExpireTokensUseCase) {
				useCase.EXPECT().Execute(mock.Anything).Return(nil).Once()
			},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve propagar erro do use case",
			setup: func(useCase *handlersjobmocks.ExpireTokensUseCase) {
				useCase.EXPECT().Execute(mock.Anything).Return(errors.New("expire failed")).Once()
			},
			expect: func(err error) {
				s.ErrorContains(err, "expire failed")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			useCase := handlersjobmocks.NewExpireTokensUseCase(s.T())
			scenario.setup(useCase)
			job := handlers.NewTokenExpirationJob(useCase, "0 3 * * *")
			scenario.expect(job.Run(context.Background()))
		})
	}
}
