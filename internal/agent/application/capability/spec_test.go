package capability

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapabilityModeString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "read", ModeRead.String())
	require.Equal(t, "write", ModeWrite.String())
	require.Empty(t, CapabilityMode(0).String())
}

func TestCapabilityModeIsValid(t *testing.T) {
	t.Parallel()

	require.True(t, ModeRead.IsValid())
	require.True(t, ModeWrite.IsValid())
	require.False(t, CapabilityMode(0).IsValid())
	require.False(t, CapabilityMode(99).IsValid())
}

func TestParseCapabilityMode(t *testing.T) {
	t.Parallel()

	mode, err := ParseCapabilityMode(" read ")
	require.NoError(t, err)
	require.Equal(t, ModeRead, mode)

	mode, err = ParseCapabilityMode("WRITE")
	require.NoError(t, err)
	require.Equal(t, ModeWrite, mode)

	_, err = ParseCapabilityMode("other")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrCapabilityModeInvalid))
}
