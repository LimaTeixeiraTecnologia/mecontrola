package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codecState struct {
	Name  string   `json:"name"`
	Value int      `json:"value"`
	Tags  []string `json:"tags"`
}

func TestCodec_RoundTrip(t *testing.T) {
	codec := NewCodec[codecState]()

	scenarios := []struct {
		name  string
		state codecState
	}{
		{
			name:  "simple state",
			state: codecState{Name: "test", Value: 42, Tags: []string{"a", "b"}},
		},
		{
			name:  "empty state",
			state: codecState{},
		},
		{
			name:  "state with nil tags",
			state: codecState{Name: "no-tags", Value: 1},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			encoded, err := codec.Encode(s.state)
			require.NoError(t, err)
			assert.NotEmpty(t, encoded)

			decoded, err := codec.Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, s.state.Name, decoded.Name)
			assert.Equal(t, s.state.Value, decoded.Value)
		})
	}
}

func TestCodec_DecodeInvalidJSON(t *testing.T) {
	codec := NewCodec[codecState]()
	_, err := codec.Decode([]byte("not-json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow: codec: decode")
}

func TestCodec_EncodeProducesValidJSON(t *testing.T) {
	codec := NewCodec[codecState]()
	state := codecState{Name: "hello", Value: 99}

	encoded, err := codec.Encode(state)
	require.NoError(t, err)

	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	assert.Equal(t, state, decoded)
}
