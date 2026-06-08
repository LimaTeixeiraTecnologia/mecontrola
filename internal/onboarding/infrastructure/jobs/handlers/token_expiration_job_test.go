package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/jobs/handlers"
)

type fakeExpireTokensUseCase struct {
	executeErr   error
	executeCalls int
}

func (f *fakeExpireTokensUseCase) Execute(_ context.Context) error {
	f.executeCalls++
	return f.executeErr
}

type TokenExpirationJobSuite struct {
	suite.Suite
	uc  *fakeExpireTokensUseCase
	job *handlers.TokenExpirationJob
}

func TestTokenExpirationJob(t *testing.T) {
	suite.Run(t, new(TokenExpirationJobSuite))
}

func (s *TokenExpirationJobSuite) SetupTest() {
	s.uc = &fakeExpireTokensUseCase{}
	s.job = handlers.NewTokenExpirationJob(s.uc, "0 3 * * *")
}

func (s *TokenExpirationJobSuite) TestName() {
	s.Equal("onboarding-token-expiration", s.job.Name())
}

func (s *TokenExpirationJobSuite) TestSchedule() {
	s.Equal("0 3 * * *", s.job.Schedule())
}

func (s *TokenExpirationJobSuite) TestRunDelegatesUseCase() {
	err := s.job.Run(context.Background())
	s.Require().NoError(err)
	s.Equal(1, s.uc.executeCalls)
}

func (s *TokenExpirationJobSuite) TestRunPropagatesError() {
	s.uc.executeErr = errors.New("expire failed")

	err := s.job.Run(context.Background())

	s.Require().Error(err)
	s.ErrorContains(err, "expire failed")
}
