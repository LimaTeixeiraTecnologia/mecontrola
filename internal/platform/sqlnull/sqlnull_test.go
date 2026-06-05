package sqlnull_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

func TestStr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{name: "empty string returns nil", input: "", want: nil},
		{name: "non-empty string returns string", input: "alice@example.com", want: "alice@example.com"},
		{name: "single space is preserved (no implicit trim)", input: " ", want: " "},
		{name: "multibyte string preserved", input: "ÁlÍce", want: "ÁlÍce"},
		{name: "long string preserved", input: "+5511988887777", want: "+5511988887777"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sqlnull.Str(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input time.Time
		want  any
	}{
		{name: "zero time returns nil", input: time.Time{}, want: nil},
		{name: "epoch unix is not zero (1970 ≠ Go zero)", input: time.Unix(0, 0).UTC(), want: time.Unix(0, 0).UTC()},
		{name: "concrete UTC time preserved", input: now, want: now},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sqlnull.Time(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestStr_TypeIsExactlyString(t *testing.T) {
	t.Parallel()

	got := sqlnull.Str("v")
	s, ok := got.(string)
	require.True(t, ok, "want string concreto, got %T", got)
	assert.Equal(t, "v", s)
}

func TestTime_TypeIsExactlyTime(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	got := sqlnull.Time(now)
	v, ok := got.(time.Time)
	require.True(t, ok, "want time.Time concreto, got %T", got)
	assert.True(t, v.Equal(now))
}
