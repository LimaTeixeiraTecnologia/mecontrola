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
	require.Contains(t, out, "parcelada")
	require.Contains(t, out, "recorrentes")
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

func TestRenderBudgetsPersona_ContainsCapabilities(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderBudgetsPersona(prompting.BudgetsPersonaData{})
	require.NoError(t, err)
	require.NotEmpty(t, strings.TrimSpace(out))

	require.Contains(t, out, "Custo Fixo")
	require.Contains(t, out, "Metas")
	require.Contains(t, out, "Liberdade Financeira")
	require.Contains(t, out, "80%")
	require.Contains(t, out, "50%")
	require.Contains(t, out, "Recorrência")
	require.Contains(t, out, "rascunho")
}

func TestRenderBudgetsPersona_ContainsConstraints(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderBudgetsPersona(prompting.BudgetsPersonaData{})
	require.NoError(t, err)

	require.Contains(t, out, "lançamentos")
	require.Contains(t, out, "imutável")
	require.Contains(t, out, "catálogo de categorias é fixo")
}

func TestRenderBudgetsPersona_InterpolatesJourneyHint(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderBudgetsPersona(prompting.BudgetsPersonaData{JourneyHint: "Orçamento de junho ainda não ativado."})
	require.NoError(t, err)
	require.Contains(t, out, "Orçamento de junho ainda não ativado.")

	without, err := prompting.RenderBudgetsPersona(prompting.BudgetsPersonaData{})
	require.NoError(t, err)
	require.NotContains(t, without, "CONTEXTO ATUAL DO USUÁRIO")
}

func TestRenderBudgetsPersona_WithoutHintNoContextSection(t *testing.T) {
	t.Parallel()
	out, err := prompting.RenderBudgetsPersona(prompting.BudgetsPersonaData{})
	require.NoError(t, err)
	require.NotContains(t, out, "CONTEXTO ATUAL DO USUÁRIO")
}
