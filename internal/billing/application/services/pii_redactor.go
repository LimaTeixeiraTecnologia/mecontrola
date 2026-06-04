// Package services contém serviços de aplicação stateless do módulo billing.
package services

import (
	"encoding/json"
	"fmt"
)

const redactedPlaceholder = "[REDACTED]"

// PIIRedactor redacta caminhos canônicos de PII em payloads JSONB de webhook_events.
// Implementação em-process via parse-modify-marshal (ADR-013, RF-49).
// Operação irreversível por design — uma vez redactado, o campo original é perdido (RF-52).
// Idempotente: aplicar Strip 2x no mesmo documento produz o mesmo resultado.
type PIIRedactor struct {
	scalarPaths  [][]string
	wildcardKeys [][]string
	starMapPaths []starMapPath
}

type starMapPath struct {
	parent string
	target string
}

// NewPIIRedactor cria um PIIRedactor com a lista canônica de caminhos PII (ADR-013, RF-49).
// Paths cobridos:
//   - Escalares: customer.cpf, customer.cnpj, customer.email, customer.mobile
//   - Wildcards (todos os filhos do objeto): customer.address.*, card.*
//   - Star-map (filhos de chave arbitrária): payment.*.card.*
func NewPIIRedactor() *PIIRedactor {
	return &PIIRedactor{
		scalarPaths: [][]string{
			{"customer", "cpf"},
			{"customer", "cnpj"},
			{"customer", "email"},
			{"customer", "mobile"},
		},
		wildcardKeys: [][]string{
			{"customer", "address"},
			{"card"},
		},
		starMapPaths: []starMapPath{
			{parent: "payment", target: "card"},
		},
	}
}

// Strip redacta todos os caminhos PII canônicos no payload JSON fornecido.
// Retorna o documento modificado com campos PII substituídos por "[REDACTED]".
// Caminhos ausentes são no-op silencioso.
// Retorna erro apenas se o payload não for JSON válido ou o marshal falhar.
func (r *PIIRedactor) Strip(raw json.RawMessage) (json.RawMessage, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("pii redactor: unmarshal: %w", err)
	}
	for _, segments := range r.scalarPaths {
		r.redactScalar(doc, segments)
	}
	for _, segments := range r.wildcardKeys {
		r.redactWildcard(doc, segments)
	}
	for _, path := range r.starMapPaths {
		r.redactStarMap(doc, path.parent, path.target)
	}
	result, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("pii redactor: marshal: %w", err)
	}
	return result, nil
}

// redactScalar substitui o valor escalar em segments pelo placeholder.
// Percorre o caminho sem criar estrutura ausente (no-op silencioso).
func (r *PIIRedactor) redactScalar(doc map[string]any, segments []string) {
	if len(segments) == 1 {
		if _, ok := doc[segments[0]]; ok {
			doc[segments[0]] = redactedPlaceholder
		}
		return
	}
	next, ok := doc[segments[0]].(map[string]any)
	if !ok {
		return
	}
	r.redactScalar(next, segments[1:])
}

// redactWildcard substitui todos os filhos do objeto alvo pelo placeholder.
// Exemplo: segments=["customer","address"] redacta customer.address.street, .city, etc.
func (r *PIIRedactor) redactWildcard(doc map[string]any, segments []string) {
	if len(segments) == 1 {
		target, ok := doc[segments[0]].(map[string]any)
		if !ok {
			return
		}
		for k := range target {
			target[k] = redactedPlaceholder
		}
		return
	}
	next, ok := doc[segments[0]].(map[string]any)
	if !ok {
		return
	}
	r.redactWildcard(next, segments[1:])
}

// redactStarMap redacta o objeto target dentro de cada entrada do map parent.
// Exemplo: parent="payment", target="card" redacta payment.<qualquer_chave>.card.*.
func (r *PIIRedactor) redactStarMap(doc map[string]any, parent, target string) {
	parentMap, ok := doc[parent].(map[string]any)
	if !ok {
		return
	}
	for _, childAny := range parentMap {
		childMap, ok := childAny.(map[string]any)
		if !ok {
			continue
		}
		targetMap, ok := childMap[target].(map[string]any)
		if !ok {
			continue
		}
		for k := range targetMap {
			targetMap[k] = redactedPlaceholder
		}
	}
}
