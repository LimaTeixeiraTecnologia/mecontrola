package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type UoWSuite struct {
	suite.Suite
	ctx context.Context
}

func TestUoW(t *testing.T) {
	suite.Run(t, new(UoWSuite))
}

func (s *UoWSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *UoWSuite) TestDeveAplicarTimeoutPadraoDeCincoSegundos() {
	_, hasDeadline := s.ctx.Deadline()
	s.False(hasDeadline)

	timedCtx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	deadline, ok := timedCtx.Deadline()
	s.True(ok)
	s.WithinDuration(time.Now().Add(5*time.Second), deadline, 100*time.Millisecond)
}

func (s *UoWSuite) TestDeveResponderDeadlineDoCallerMaisRestrito() {
	callerDeadline := time.Now().Add(1 * time.Second)
	ctx, cancel := context.WithDeadline(s.ctx, callerDeadline)
	defer cancel()

	deadline, ok := ctx.Deadline()
	s.True(ok)
	s.Equal(callerDeadline.Truncate(time.Millisecond), deadline.Truncate(time.Millisecond))
}
