# ADR-013 — PII redactor em-process com parse-modify-marshal manual

## Metadados

- **Título:** Implementação do redactor JSONB de PII para `webhook_events`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de plataforma + segurança
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-49, CA-12), `techspec.md` §Schema Postgres, ADR-007

## Contexto

RF-49 do PRD define lista de caminhos JSONB a redactar:
- `customer.cpf`, `customer.cnpj`
- `customer.email`
- `customer.mobile`
- `customer.address.*` (todos os subcampos)
- `card.*`
- `payment.*.card.*`

`customer.address.*` e `card.*` são wildcards (remover todos os filhos do objeto). Substituir por string literal `"[REDACTED]"`.

Confronto com `go.mod`: não há lib de jsonpath. Opções:
1. Parse-modify-marshal manual via `map[string]any`.
2. `tidwall/sjson` + `tidwall/gjson` (~10k stars, estável) — nova dependência.
3. SQL puro com `jsonb_set` chain.

## Decisão

Implementação manual em `internal/billing/application/redactor.go` (ou local em `infrastructure` se preferir, conforme convenção do projeto):

```go
package redactor

import (
    "encoding/json"
    "fmt"
)

const redactedPlaceholder = "[REDACTED]"

type PIIRedactor struct {
    scalarPaths []scalarPath  // e.g., {"customer", "cpf"}
    wildcardPaths []wildcardPath  // e.g., {"customer", "address"} (redact all children)
}

type scalarPath struct{ segments []string }
type wildcardPath struct{ segments []string }

func New() *PIIRedactor {
    return &PIIRedactor{
        scalarPaths: []scalarPath{
            {segments: []string{"customer", "cpf"}},
            {segments: []string{"customer", "cnpj"}},
            {segments: []string{"customer", "email"}},
            {segments: []string{"customer", "mobile"}},
        },
        wildcardPaths: []wildcardPath{
            {segments: []string{"customer", "address"}},
            {segments: []string{"card"}},
            {segments: []string{"payment", "card"}},
        },
    }
}

func (r *PIIRedactor) Strip(raw json.RawMessage) (json.RawMessage, error) {
    var doc map[string]any
    if err := json.Unmarshal(raw, &doc); err != nil {
        return nil, fmt.Errorf("redactor: unmarshal: %w", err)
    }
    for _, p := range r.scalarPaths {
        r.redactScalar(doc, p.segments)
    }
    for _, p := range r.wildcardPaths {
        r.redactWildcard(doc, p.segments)
    }
    return json.Marshal(doc)
}

func (r *PIIRedactor) redactScalar(doc map[string]any, segments []string) {
    if len(segments) == 0 || doc == nil {
        return
    }
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

func (r *PIIRedactor) redactWildcard(doc map[string]any, segments []string) {
    if len(segments) == 0 || doc == nil {
        return
    }
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
```

Implementação ~80 linhas com testes. Idempotente: aplicar 2x sobre o mesmo doc não muda o resultado (campo já é `"[REDACTED]"`). Tolerante: caminho ausente é no-op silencioso.

`payment.*.card.*` é tratado iterando o map intermediário (`payment.<any>.card.*` = para cada chave em `payment`, redact `card`). Adicionar método auxiliar:

```go
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
```

`payment.*.card.*` chama `redactStarMap(doc, "payment", "card")`.

## Alternativas Consideradas

### `tidwall/sjson` + `tidwall/gjson`

- Vantagem: API minimal (`sjson.DeleteBytes(json, "customer.cpf")`); wildcards verdadeiros.
- Desvantagem: 1 nova dependência direta (~10k stars, estável); duas libs (`gjson` para queries, `sjson` para modificação); overhead de manter dep.
- Rejeitada porque a complexidade real do redactor é baixa (80 linhas) e zero deps é preferível.

### SQL `jsonb_set` chain

- Vantagem: zero código Go novo.
- Desvantagem: SQL imenso (~10 statements `jsonb_set`); difícil testar; `customer.address.*` exigiria função PL/pgSQL ou iteração JSONB; UPDATE com 10 funções nested é frágil.
- Rejeitada por dificultar manutenção.

### Library `tidwall/sjson` apenas para o redactor (sem gjson)

- Vantagem: 1 dep só.
- Desvantagem: ainda exige importar 5k linhas Go para fazer trabalho que se resolve em 80.
- Rejeitada por trade-off não favorecer.

## Consequências

### Benefícios Esperados

- Zero novas dependências.
- Código self-contained e fácil de testar (table-driven com payloads sintéticos).
- Lista de paths é dado em código — review obrigatório ao adicionar campos PII novos.
- Comportamento determinístico.

### Trade-offs e Custos

- ~80 linhas para manter (vs ~20 linhas com sjson).
- Performance: ~5ms por payload de 10KB (parse JSON + walk + marshal). Anonimização roda em batch de 500 → ~2.5s/batch. Aceitável para job diário.

### Riscos e Mitigações

- **Risco:** novo campo PII surge no payload Kiwify e fica esquecido. **Mitigação:** PR de schema evolution Kiwify obriga revisão da lista; integration test (CA-12) usa payload sintético com todos os campos esperados.
- **Risco:** payload com encoding incomum (Unicode escape, números muito grandes) quebra unmarshal. **Mitigação:** retorno de erro propaga → linha não é anonimizada (continua íntegra); métrica `billing_webhook_events_anonymization_errors_total` alerta.
- **Risco:** strings literais hard-coded de caminhos divergem do payload real (e.g., Kiwify usa `Customer.Cpf` CamelCase). **Mitigação:** validação empírica via integration test com payload-exemplo real (uma vez disponível).

## Plano de Implementação

1. Criar `internal/billing/application/services/pii_redactor.go` (domain service stateless? ou application service? Decisão: application service — depende de detalhes de PII storage, não de regra de domínio).
2. Implementar `PIIRedactor` com `Strip(json.RawMessage) (json.RawMessage, error)`.
3. `AnonymizeWebhookEventsUseCase` recebe `*PIIRedactor` via DI e aplica.
4. Test unit: tabela com payloads sintéticos cobrindo cada path + idempotência + path ausente + payload malformado.
5. Test integração (CA-12) cobrindo retenção two-tier (ADR-007).

## Monitoramento e Validação

- Métrica `billing_webhook_events_anonymized_total` (counter) em sucesso.
- Métrica `billing_webhook_events_anonymization_errors_total` (counter) em falha de unmarshal/marshal.
- Span OTel `billing.anonymization.tick` com atributo `events_redacted`.

## Impacto em Documentação e Operação

- AGENTS.md billing documenta a lista de paths.
- Runbook LGPD: como adicionar novo path ao redactor (PR + migration de re-anonimização opcional para linhas já anonimizadas que ganham novo path).

## Revisão Futura

- Se número de paths > 20, considerar gerador a partir de schema YAML.
- Se Kiwify expor schema OpenAPI/JSON Schema, derivar lista automaticamente.
