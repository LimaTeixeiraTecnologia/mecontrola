//go:build integration

package e2e_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

func TestParseInbound_RealLLM_Confidence(t *testing.T) {
	if os.Getenv("RUN_REAL_LLM") == "" {
		t.Skip("set RUN_REAL_LLM=1 e exporte OPENROUTER_API_KEY para rodar a validacao real")
	}
	require.NotEmpty(t, os.Getenv("OPENROUTER_API_KEY"), "OPENROUTER_API_KEY ausente")

	parser := realParser(t)

	cases := []struct {
		name      string
		text      string
		wantClear bool
	}{
		{name: "gasto claro", text: "ifood 58 reais", wantClear: true},
		{name: "gasto claro com cartao", text: "gastei 200 no mercado com nubank", wantClear: true},
		{name: "ganho claro", text: "recebi meu salário de 16.400", wantClear: true},
		{name: "consulta clara", text: "qual a fatura do nubank?", wantClear: true},
		{name: "vago e ambiguo", text: "acho que foi uns trocados ali não lembro quanto", wantClear: false},
		{name: "ruido", text: "oi tudo bem?", wantClear: false},
	}

	values := make([]float64, 0, len(cases))
	for _, tc := range cases {
		wantClear := tc.wantClear
		out := parseUntil(t, parser, tc.text, func(o usecases.ParseInboundOutput) bool {
			return !wantClear || o.Intent.Kind() != intent.KindUnknown
		})
		conf := out.Confidence.Value()
		kind := out.Intent.Kind()
		require.GreaterOrEqual(t, conf, 0.0, "confidence deve ser >= 0")
		require.LessOrEqual(t, conf, 1.0, "confidence deve ser <= 1")
		values = append(values, conf)
		t.Logf("[%-18s] %-50q => kind=%s confidence=%.3f", tc.name, tc.text, kind.String(), conf)
		if tc.wantClear {
			require.NotEqual(t, intent.KindUnknown, kind, "caso claro %q caiu em unknown em %d tentativas — o modelo nao esta classificando", tc.text, realLLMMaxAttempts)
			require.Greater(t, conf, 0.0, "caso claro %q deveria ter confidence > 0 vinda do modelo", tc.text)
		}
	}

	allDefault := true
	for _, v := range values {
		if v != 1.0 {
			allDefault = false
			break
		}
	}
	require.False(t, allDefault, "todas as confidences vieram 1.0 — provavel que o modelo NAO esteja populando o campo e estamos caindo no default neutro")
}
