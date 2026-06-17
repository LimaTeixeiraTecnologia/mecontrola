package prompting_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
)

func TestRenderPersonaSystem_ContainsCanonicalRules(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderPersonaSystem(prompting.PersonaSystemData{})
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(out))

	require.Contains(t, out, "MeControla")
	require.Contains(t, out, "cartões")
	require.Contains(t, out, "orçamento")
	require.Contains(t, out, "lançamentos")
	require.Contains(t, out, "Conexão")
	require.Contains(t, out, "Atualização Automática")
	require.Contains(t, out, "Segurança")
	require.Contains(t, out, "SQL")
	require.Contains(t, out, "Kiwify")
}

func TestRenderPersonaSystem_DescribesAllModuleAreas(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderPersonaSystem(prompting.PersonaSystemData{})
	require.NoError(t, err)

	require.Contains(t, out, "Categorias")
	require.Contains(t, out, "Cartões de crédito")
	require.Contains(t, out, "Orçamento mensal")
	require.Contains(t, out, "Lançamentos")
	require.Contains(t, out, "Conta e assinatura")
	require.Contains(t, out, "fatura fechada")
}

func TestRenderPersonaSystem_InterpolatesJourneyHint(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderPersonaSystem(prompting.PersonaSystemData{JourneyHint: "Usuário ainda não cadastrou cartões."})
	require.NoError(t, err)
	require.Contains(t, out, "Usuário ainda não cadastrou cartões.")

	without, err := prompting.RenderPersonaSystem(prompting.PersonaSystemData{})
	require.NoError(t, err)
	require.NotContains(t, without, "CONTEXTO ATUAL DO USUÁRIO")
}
