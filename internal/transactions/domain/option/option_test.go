package option_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
)

func TestSome(t *testing.T) {
	o := option.Some("hello")
	v, ok := o.Get()
	assert.True(t, ok)
	assert.Equal(t, "hello", v)
	assert.True(t, o.IsPresent())
}

func TestNone(t *testing.T) {
	o := option.None[string]()
	v, ok := o.Get()
	assert.False(t, ok)
	assert.Equal(t, "", v)
	assert.False(t, o.IsPresent())
}

func TestSomeInt(t *testing.T) {
	o := option.Some(42)
	v, ok := o.Get()
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestNoneInt(t *testing.T) {
	o := option.None[int]()
	_, ok := o.Get()
	assert.False(t, ok)
}
