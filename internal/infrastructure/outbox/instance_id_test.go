package outbox_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox"
)

type InstanceIDSuite struct {
	suite.Suite
}

func TestInstanceID(t *testing.T) {
	suite.Run(t, new(InstanceIDSuite))
}

func (s *InstanceIDSuite) TestNewInstanceID_NaoVazio() {
	id := outbox.NewInstanceID()
	s.NotEmpty(id, "InstanceID nao pode ser vazio")
}

func (s *InstanceIDSuite) TestNewInstanceID_ContemPIDCorrente() {
	id := outbox.NewInstanceID()
	pid := os.Getpid()
	s.Contains(id, fmt.Sprintf("%d", pid), "InstanceID deve conter o pid corrente")
}

func (s *InstanceIDSuite) TestNewInstanceID_FormatoHostnamePID() {
	id := outbox.NewInstanceID()
	// Deve conter pelo menos um hífen separando hostname do pid
	s.Truef(strings.Contains(id, "-"), "InstanceID deve ter formato hostname-pid, got %q", id)
	// Último segmento deve ser o pid
	parts := strings.Split(id, "-")
	pid := fmt.Sprintf("%d", os.Getpid())
	s.Equal(pid, parts[len(parts)-1], "ultimo segmento deve ser o pid")
}
