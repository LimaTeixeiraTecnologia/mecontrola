package workflows

import (
	"testing"
	"time"
)

func fixedNow(t *testing.T, iso string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04", iso, loc)
	if err != nil {
		t.Fatalf("fixedNow parse %q: %v", iso, err)
	}
	return parsed
}

func TestParseInputDate(t *testing.T) {
	scenarios := []struct {
		name string
		text string
		now  string
		want string
	}{
		{name: "hoje", text: "hoje", now: "2026-03-31 10:00", want: "2026-03-31"},
		{name: "ontem", text: "ontem", now: "2026-03-31 10:00", want: "2026-03-30"},
		{name: "anteontem", text: "anteontem", now: "2026-03-31 10:00", want: "2026-03-29"},
		{name: "sexta passada permanece weekday", text: "sexta passada", now: "2026-07-30 10:00", want: "2026-07-17"},
		{name: "segunda weekday atual", text: "segunda", now: "2026-07-30 10:00", want: "2026-07-27"},
		{name: "DD/MM", text: "10/07", now: "2026-07-30 10:00", want: "2026-07-10"},
		{name: "ISO", text: "2026-01-15", now: "2026-07-30 10:00", want: "2026-01-15"},
		{name: "vazio nao resolve", text: "qualquer coisa", now: "2026-07-30 10:00", want: ""},

		{name: "semana passada", text: "semana passada", now: "2026-07-30 10:00", want: "2026-07-23"},
		{name: "semana passada vira mes", text: "semana passada", now: "2026-03-05 10:00", want: "2026-02-26"},

		{name: "mes passado dia comum", text: "mês passado", now: "2026-07-15 10:00", want: "2026-06-15"},
		{name: "mes passado sem acento", text: "mes passado", now: "2026-07-15 10:00", want: "2026-06-15"},
		{name: "mes passado clamp 31 para fev", text: "mês passado", now: "2026-03-31 10:00", want: "2026-02-28"},
		{name: "mes passado clamp 31 para fev bissexto", text: "mês passado", now: "2024-03-31 10:00", want: "2024-02-29"},
		{name: "mes passado vira ano anterior", text: "mês passado", now: "2026-01-10 10:00", want: "2025-12-10"},
		{name: "mes passado clamp 31 para 30", text: "mês passado", now: "2026-07-31 10:00", want: "2026-06-30"},

		{name: "dia N no mes corrente passado", text: "dia 10", now: "2026-07-30 10:00", want: "2026-07-10"},
		{name: "dia N igual a hoje", text: "dia 30", now: "2026-07-30 10:00", want: "2026-07-30"},
		{name: "dia N futuro cai no mes anterior", text: "dia 15", now: "2026-07-10 10:00", want: "2026-06-15"},
		{name: "dia 31 em mes de 30 dias cai no anterior", text: "dia 31", now: "2026-04-15 10:00", want: "2026-03-31"},
		{name: "dia 29 fevereiro nao bissexto varre atras", text: "dia 29", now: "2026-02-10 10:00", want: "2026-01-29"},
		{name: "dia 29 fevereiro bissexto", text: "dia 29", now: "2024-02-29 10:00", want: "2024-02-29"},
		{name: "dia N vira ano anterior", text: "dia 20", now: "2026-01-10 10:00", want: "2025-12-20"},
		{name: "dia 0 invalido", text: "dia 0", now: "2026-07-30 10:00", want: ""},
		{name: "dia 32 invalido", text: "dia 32", now: "2026-07-30 10:00", want: ""},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got := ParseInputDate(scenario.text, fixedNow(t, scenario.now))
			if got != scenario.want {
				t.Fatalf("ParseInputDate(%q) = %q, want %q", scenario.text, got, scenario.want)
			}
		})
	}
}

func TestDaysInMonth(t *testing.T) {
	scenarios := []struct {
		name  string
		year  int
		month time.Month
		want  int
	}{
		{name: "janeiro 31", year: 2026, month: time.January, want: 31},
		{name: "fevereiro nao bissexto 28", year: 2026, month: time.February, want: 28},
		{name: "fevereiro bissexto 29", year: 2024, month: time.February, want: 29},
		{name: "abril 30", year: 2026, month: time.April, want: 30},
		{name: "dezembro 31", year: 2026, month: time.December, want: 31},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if got := daysInMonth(scenario.year, scenario.month); got != scenario.want {
				t.Fatalf("daysInMonth(%d, %v) = %d, want %d", scenario.year, scenario.month, got, scenario.want)
			}
		})
	}
}
