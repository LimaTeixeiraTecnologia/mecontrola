package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMonetary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     string
		wantCents int64
		wantOk    bool
	}{
		{"plain_int", "3500", 350000, true},
		{"with_prefix", "R$ 3500", 350000, true},
		{"prefix_no_space", "R$3500", 350000, true},
		{"lowercase_prefix", "r$ 3500", 350000, true},
		{"rs_prefix", "RS 3500", 350000, true},
		{"comma_decimal", "3500,00", 350000, true},
		{"comma_decimal_with_prefix", "R$ 3.500,00", 350000, true},
		{"dot_thousand", "3.500", 350000, true},
		{"english_decimal", "3500.50", 350050, true},
		{"english_thousands_decimal", "3,500.00", 350000, true},
		{"empty", "", 0, false},
		{"letters", "abc", 0, false},
		{"mixed", "1a2", 0, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cents, ok := parseMonetary(tc.input)
			require.Equal(t, tc.wantOk, ok, "input=%q got %d", tc.input, cents)
			if tc.wantOk {
				require.Equal(t, tc.wantCents, cents, "input=%q", tc.input)
			}
		})
	}
}

func TestParseDay(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		wantN  int
		wantOk bool
	}{
		{"5", 5, true},
		{"27", 27, true},
		{"dia 27", 27, true},
		{"DIA 5", 5, true},
		{" 10 ", 10, true},
		{"", 0, false},
		{"abc", 0, false},
		{"dia abc", 0, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			n, ok := parseDay(tc.input)
			require.Equal(t, tc.wantOk, ok)
			if tc.wantOk {
				require.Equal(t, tc.wantN, n)
			}
		})
	}
}

func TestParseYesNo(t *testing.T) {
	t.Parallel()

	yes := []string{"sim", "Sim", "SIM", "s", "claro", "quero", "ok", "yes", "y"}
	for _, in := range yes {
		v, ok := parseYesNo(in)
		require.True(t, ok, "input=%q", in)
		require.True(t, v, "input=%q", in)
	}
	no := []string{"nao", "não", "n", "no"}
	for _, in := range no {
		v, ok := parseYesNo(in)
		require.True(t, ok, "input=%q", in)
		require.False(t, v, "input=%q", in)
	}
	_, ok := parseYesNo("maybe")
	require.False(t, ok)
}
