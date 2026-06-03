package outbox

import (
	"fmt"
	"maps"
	"slices"
)

// Headers é um value object de primeira classe (OC #4) que encapsula os metadados
// de propagação de contexto de um evento. Chaves canônicas: "traceparent",
// "correlation_id", "causation_id".
type Headers map[string]string

// _canonicalKeys lista as chaves de propagação canônicas reconhecidas pelo Outbox.
var _canonicalKeys = []string{"traceparent", "correlation_id", "causation_id"}

// WithTrace retorna um novo Headers com a chave "traceparent" definida.
// Não muta o receptor — cria cópia.
func (h Headers) WithTrace(traceparent string) Headers {
	out := h.clone()
	out["traceparent"] = traceparent
	return out
}

// Get retorna o valor da chave informada e se ela existe.
func (h Headers) Get(key string) (string, bool) {
	v, ok := h[key]
	return v, ok
}

// Validate verifica que as chaves presentes pertencem ao conjunto canônico.
// Retorna erro para cada chave desconhecida encontrada.
func (h Headers) Validate() error {
	for k := range h {
		if !isCanonicalKey(k) {
			return fmt.Errorf("outbox: headers: chave %q nao e canonica (esperado: traceparent, correlation_id, causation_id)", k)
		}
	}
	return nil
}

// clone retorna uma cópia rasa do mapa.
func (h Headers) clone() Headers {
	out := make(Headers, len(h)+1)
	maps.Copy(out, h)
	return out
}

func isCanonicalKey(k string) bool {
	return slices.Contains(_canonicalKeys, k)
}
