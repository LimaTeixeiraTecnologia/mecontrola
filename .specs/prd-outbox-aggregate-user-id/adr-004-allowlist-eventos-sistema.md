# ADR-004 — Allowlist explícita de event types de sistema

## Metadados

- **Título:** Eventos legitimamente sem `aggregate_user_id` declarados em allowlist no pacote outbox
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-15; [techspec](techspec.md) seção Allowlist; ADR-001

## Contexto

Nem todo evento tem um usuário dono. Exemplos hipotéticos: housekeeping de sistema, sinalizações de health, eventos de bootstrap. Tratar todos os eventos sem `user_id` como suspeitos (warning + métrica `has_user_id="false"`) gera ruído operacional quando há casos legítimos.

A pergunta é: **como distinguir um caller que esqueceu de popular** vs **um event type legitimamente sem dono**?

## Decisão

Manter **allowlist explícita** no pacote `internal/platform/outbox`:

```go
package outbox

var systemEventTypes = map[string]struct{}{
    // Declarar explicitamente. Inicialmente vazio no MVP.
    // Adicionar entrada exige ADR de superseder ou PR com revisão.
}

func isSystemEvent(eventType string) bool {
    _, ok := systemEventTypes[eventType]
    return ok
}
```

Em `outbox.NewEvent`, quando `AggregateUserID == ""`:
- Se `isSystemEvent(input.Type)` → silent (sem warning, sem métrica `has_user_id="false"`).
- Caso contrário → log warn + métrica.

A allowlist começa **vazia** no MVP. Adicionar entrada requer:
1. Justificativa documentada (ADR ou PR comment) explicando por que o evento legitimamente não tem dono.
2. Revisão humana — não pode ser auto-adicionada por linter.

## Alternativas Consideradas

1. **Tratar todos os eventos sem user_id como suspeitos** (warning sempre) — silencia operador a longo prazo. **Rejeitada**: gera ruído; operador acaba ignorando warnings.
2. **Detecção heurística por nome** (event_type contém "system" → silent) — frágil. **Rejeitada**: regras implícitas violam o princípio de explicitness; futuras decisões ficam difíceis de auditar.
3. **Allowlist em config externa** (env var ou YAML) — mais flexível. **Rejeitada**: estado mutável fora do código quebra reproducibilidade; mudanças deveriam exigir review de PR.
4. **Allowlist em tabela DB** (`system_event_types`) — dinâmica. **Rejeitada**: idem; over-engineering para MVP.

## Consequências

### Benefícios Esperados

- Explicitness máxima: olhar a lista responde "esse evento deveria ter user_id?".
- Adicionar entrada exige revisão humana.
- Gate de lint pode usar a mesma lista como fonte de verdade.

### Trade-offs e Custos

- Lista começar vazia significa que qualquer evento sem user_id loga warning na v1. Aceito: força revisão proativa de eventuais callers de sistema que apareçam.
- Adicionar entrada exige PR (não há atalho operacional). Aceito: alinha com princípio inegociável de "sem falso positivo".

### Riscos e Mitigações

- **R-01**: futuro desenvolvedor adiciona entrada na allowlist para silenciar warning de caller mal escrito. **Mitigação**: revisão de PR + ADR exigida para qualquer entrada nova.
- **R-02**: lista cresce sem controle. **Mitigação**: revisão anual da allowlist; remover entradas obsoletas.

## Plano de Implementação

1. Criar `internal/platform/outbox/system_event_allowlist.go` com map vazio.
2. `outbox.NewEvent` consulta a allowlist antes de logar warning.
3. Gate de lint `lint:outbox-user-id` pode opcionalmente carregar a mesma lista para isentar de validação.
4. Documentar política de adição no comentário do arquivo (única exceção R-ADAPTER-001.1: comentário de cabeçalho descrevendo política).

## Monitoramento e Validação

- Auditar a allowlist a cada 6 meses.
- Métrica `outbox_events_inserted_total{has_user_id="false"}` deve ser próxima de 0 ou refletir apenas tipos na allowlist.

## Impacto em Documentação e Operação

- `internal/platform/outbox/system_event_allowlist.go` próprio é documentação.
- Runbook ganha seção "como adicionar evento à allowlist".

## Revisão Futura

Revisar quando:
- Lista atingir > 5 entradas (sintoma de crescimento descontrolado).
- Surgir necessidade de eventos de sistema com semântica nova.
- Data sugerida: 2026-12-12.
