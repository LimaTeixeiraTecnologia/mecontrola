package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

func TestDirectionString_Zero(t *testing.T) {
	var d valueobjects.Direction
	assert.Equal(t, "", d.String())
}

func TestFrequencyString_Zero(t *testing.T) {
	var f valueobjects.Frequency
	assert.Equal(t, "", f.String())
}
