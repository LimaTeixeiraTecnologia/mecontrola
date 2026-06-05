package outbox

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type InstanceIDSuite struct {
	suite.Suite
}

func TestInstanceID(t *testing.T) {
	suite.Run(t, new(InstanceIDSuite))
}

func (s *InstanceIDSuite) TestNewInstanceID_ContainsHostAndPID() {
	host, _ := os.Hostname()
	pid := os.Getpid()

	id := newInstanceID()

	s.True(strings.HasPrefix(id, host+"-"), "should start with hostname")
	s.Contains(id, fmt.Sprintf("-%d-", pid), "should contain pid")
}

func (s *InstanceIDSuite) TestNewInstanceID_IsUnique() {
	a := newInstanceID()
	b := newInstanceID()
	s.NotEqual(a, b)
}
