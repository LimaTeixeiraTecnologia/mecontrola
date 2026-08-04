package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

func TestBuildO11yConfigRegistersGlobal(t *testing.T) {
	cfg := &configs.Config{}
	o11yConfig := buildO11yConfig(cfg, "test-host")
	assert.True(t, o11yConfig.RegisterGlobal)
}
