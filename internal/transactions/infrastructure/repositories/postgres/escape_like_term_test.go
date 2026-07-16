package postgres

import "testing"

func TestEscapeLikeTerm(t *testing.T) {
	scenarios := []struct {
		name string
		term string
		want string
	}{
		{name: "sem metacaracteres permanece igual", term: "mercado", want: "mercado"},
		{name: "percent vira literal", term: "50%", want: `50\%`},
		{name: "underscore vira literal", term: "conta_luz", want: `conta\_luz`},
		{name: "backslash vira literal", term: `a\b`, want: `a\\b`},
		{name: "combinacao de metacaracteres", term: `10%_\x`, want: `10\%\_\\x`},
		{name: "vazio permanece vazio", term: "", want: ""},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if got := escapeLikeTerm(scenario.term); got != scenario.want {
				t.Fatalf("escapeLikeTerm(%q) = %q, want %q", scenario.term, got, scenario.want)
			}
		})
	}
}
