package kiwify_test

import (
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/http/client/kiwify"
)

// FuzzPayloadMapperParse verifica que PayloadMapper.Parse nunca entra em pânico
// com qualquer entrada arbitrária. Corpus seed cobre payloads válidos e inválidos.
func FuzzPayloadMapperParse(f *testing.F) {
	registry := kiwify.NewBillingPlansRegistryFromMap(map[string]valueobjects.PlanCode{
		"prod-monthly": valueobjects.PlanCodeMonthly,
	})
	mapper := kiwify.NewPayloadMapper(registry, nil)

	// seed: payload válido
	f.Add([]byte(`{
		"id":"event-001",
		"webhook_event_type":"compra_aprovada",
		"updated_at":"2024-01-01T00:00:00Z",
		"customer":{"mobile":"11999990000","email":"a@b.com"},
		"product":{"id":"prod-monthly"},
		"subscription":{"id":"sub-001","current_period_start":"2024-01-01T00:00:00Z","current_period_end":"2024-01-31T00:00:00Z"},
		"refund":{"amount_cents":0},
		"tracking":{"src":"token123"}
	}`))
	// seed: JSON inválido
	f.Add([]byte(`{invalid`))
	// seed: vazio
	f.Add([]byte(``))
	// seed: null
	f.Add([]byte(`null`))
	// seed: array
	f.Add([]byte(`[]`))
	// seed: payload sem campos obrigatórios
	f.Add([]byte(`{}`))
	// seed: tipos errados nos campos
	f.Add([]byte(`{"id":123,"webhook_event_type":true,"customer":{"mobile":null}}`))
	// seed: strings muito longas
	f.Add([]byte(`{"webhook_event_type":"` + string(make([]byte, 10000)) + `"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// nunca deve entrar em pânico independentemente da entrada
		_, _ = mapper.Parse(data)
	})
}
