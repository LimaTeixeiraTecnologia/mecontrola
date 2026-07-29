package usecases

import (
	"testing"
	"time"
)

func TestResolveEntryDate(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	yesterday := now.Add(-24 * time.Hour).Format("2006-01-02")

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "vazio cai para hoje", raw: "", want: today},
		{name: "hoje literal resolve para ISO", raw: "hoje", want: today},
		{name: "hoje com espacos resolve para ISO", raw: "  hoje  ", want: today},
		{name: "ontem literal resolve para ISO", raw: "ontem", want: yesterday},
		{name: "ISO passa direto", raw: "2026-07-15", want: "2026-07-15"},
		{name: "data curta dd/mm resolve no ano corrente", raw: "15/07", want: time.Date(now.Year(), 7, 15, 0, 0, 0, 0, loc).Format("2006-01-02")},
		{name: "lixo nao parseavel cai para hoje", raw: "qualquer-coisa", want: today},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveEntryDate(tc.raw); got != tc.want {
				t.Fatalf("resolveEntryDate(%q) = %q; want %q", tc.raw, got, tc.want)
			}
		})
	}
}
