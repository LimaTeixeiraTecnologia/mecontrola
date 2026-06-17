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
		{"multiple_dot_thousands", "1.234.567", 123456700, true},
		{"english_decimal", "3500.50", 350050, true},
		{"english_thousands_decimal", "3,500.00", 350000, true},
		{"empty", "", 0, false},
		{"prefix_only", "R$ ", 0, false},
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

	yes := []string{
		"sim", "Sim", "SIM", "s", "claro", "quero", "ok", "yes", "y",
		"pode", "manda", "isso", "confirmo", "bora", "claro que sim",
		"sim cadastrar", "sim, cadastrar", "tá bom", "ta bom", "pode sim",
		"sim!", "pode ser", "beleza",
	}
	for _, in := range yes {
		v, ok := parseYesNo(in)
		require.True(t, ok, "input=%q", in)
		require.True(t, v, "input=%q", in)
	}
	no := []string{"nao", "não", "n", "no", "agora nao", "negativo", "nope"}
	for _, in := range no {
		v, ok := parseYesNo(in)
		require.True(t, ok, "input=%q", in)
		require.False(t, v, "input=%q", in)
	}
	unknown := []string{"maybe", "talvez", "sei la", ""}
	for _, in := range unknown {
		_, ok := parseYesNo(in)
		require.False(t, ok, "input=%q", in)
	}
}

func TestParseCardShortcut(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantOk      bool
		wantName    string
		wantLimit   int64
		wantClosing int
		wantDue     int
	}{
		{"keywords_curtas", "Nubank 10000 fecha 1 vence 1", true, "Nubank", 1000000, 1, 1},
		{"keywords_longas", "Nubank limite 10000 fechamento 1 vencimento 1", true, "Nubank", 1000000, 1, 1},
		{"posicional", "Inter 5000 27 5", true, "Inter", 500000, 27, 5},
		{"nome_composto", "Cartao Inter 5000 27 5", true, "Cartao Inter", 500000, 27, 5},
		{"com_rs", "Nubank R$ 5000 27 5", true, "Nubank", 500000, 27, 5},
		{"so_nome", "Nubank", false, "", 0, 0, 0},
		{"sem_campos_suficientes", "Nubank 5000 27", false, "", 0, 0, 0},
		{"dia_invalido", "Nubank 5000 99 5", false, "", 0, 0, 0},
		{"vazio", "", false, "", 0, 0, 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			card, ok := parseCardShortcut(tc.input)
			require.Equal(t, tc.wantOk, ok, "input=%q", tc.input)
			if tc.wantOk {
				require.Equal(t, tc.wantName, card.Name)
				require.Equal(t, tc.wantLimit, card.LimitCents)
				require.Equal(t, tc.wantClosing, card.ClosingDay)
				require.Equal(t, tc.wantDue, card.DueDay)
			}
		})
	}
}
