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

func TestCodec_MergePatch_PreservesBaseFields(t *testing.T) {
	codec := NewCodec[codecState]()
	base := codecState{Name: "original", Value: 42, Tags: []string{"a", "b"}}

	baseBytes, err := codec.Encode(base)
	require.NoError(t, err)

	patch := []byte(`{"value":99}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	require.NoError(t, err)

	result, err := codec.Decode(merged)
	require.NoError(t, err)
	assert.Equal(t, "original", result.Name)
	assert.Equal(t, 99, result.Value)
	assert.Equal(t, []string{"a", "b"}, result.Tags)
}

func TestCodec_MergePatch_DeltaOverwritesOnlyPresentKeys(t *testing.T) {
	codec := NewCodec[codecState]()
	base := codecState{Name: "base", Value: 10, Tags: []string{"x"}}

	baseBytes, err := codec.Encode(base)
	require.NoError(t, err)

	patch := []byte(`{"name":"patched"}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	require.NoError(t, err)

	result, err := codec.Decode(merged)
	require.NoError(t, err)
	assert.Equal(t, "patched", result.Name)
	assert.Equal(t, 10, result.Value)
	assert.Equal(t, []string{"x"}, result.Tags)
}

func TestCodec_MergePatch_NullRemovesKey(t *testing.T) {
	codec := NewCodec[map[string]any]()
	base := map[string]any{"name": "hello", "extra": "remove-me"}

	baseBytes, err := codec.Encode(base)
	require.NoError(t, err)

	patch := []byte(`{"extra":null}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	require.NoError(t, err)

	result, err := codec.Decode(merged)
	require.NoError(t, err)
	assert.Equal(t, "hello", result["name"])
	_, exists := result["extra"]
	assert.False(t, exists)
}

func TestCodec_MergePatch_EmptyPatch_IsNoOp(t *testing.T) {
	codec := NewCodec[codecState]()
	base := codecState{Name: "noop", Value: 7, Tags: []string{"z"}}

	baseBytes, err := codec.Encode(base)
	require.NoError(t, err)

	patch := []byte(`{}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	require.NoError(t, err)

	result, err := codec.Decode(merged)
	require.NoError(t, err)
	assert.Equal(t, base, result)
}

func TestCodec_MergePatch_InvalidBase_ReturnsError(t *testing.T) {
	codec := NewCodec[codecState]()
	_, err := codec.MergePatch([]byte("not-json"), []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow: codec: merge base")
}

func TestCodec_MergePatch_InvalidPatch_ReturnsError(t *testing.T) {
	codec := NewCodec[codecState]()
	base := codecState{Name: "x", Value: 1}
	baseBytes, _ := codec.Encode(base)
	_, err := codec.MergePatch(baseBytes, []byte("not-json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow: codec: merge patch")
}

func TestCodec_MergePatch_ArrayReplacesPerRFC7386(t *testing.T) {
	codec := NewCodec[codecState]()
	base := codecState{Name: "base", Value: 1, Tags: []string{"a", "b"}}
	baseBytes, err := codec.Encode(base)
	require.NoError(t, err)

	patch := []byte(`{"tags":["c"]}`)
	merged, err := codec.MergePatch(baseBytes, patch)
	require.NoError(t, err)

	result, err := codec.Decode(merged)
	require.NoError(t, err)
	assert.Equal(t, []string{"c"}, result.Tags)
	assert.Equal(t, "base", result.Name)
}
