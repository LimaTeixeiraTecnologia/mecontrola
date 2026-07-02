# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estratégia de idempotência/concorrência das novas tools de escrita
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD (RF-15, RF-16, RF-17, RF-18, RF-35); techspec; ADR-001

## Contexto

O PRD exige (RF-35) que novas escritas espelhem o padrão já usado no agente: criações via
`IdempotentWrite` (chave `wamid|itemSeq|operation`, `idempotent_write.go:72`), edições/exclusões por
concorrência otimista `version` + gate destrutivo. Diferente de `RawCreateTransaction`, o DTO
`RawCreateRecurringTemplate` **não possui** campos de origem (`OriginWamid`/`OriginItemSeq`), então a
idempotência não pode depender do DTO do módulo.

## Decisão

- **`create_recurrence`**: envolver a escrita em `IdempotentWrite.Execute(ctx, userID, wamid, itemSeq,
  "create_recurrence", "recurring_template", writeClosure)`. O closure chama
  `RecurrenceManager.CreateRecurrence`. A idempotência vive na camada do agente (ledger próprio
  `agents_write_*`), independente do DTO do módulo — o closure não precisa de origin fields.
- **`update_recurrence` / `delete_recurrence` / `update_card`**: concorrência otimista por `version`
  (já presente nos use cases `Update*`/`Delete*`), efetivada apenas após confirmação no gate
  `destructive-confirm` (ADR-001). Sem `IdempotentWrite` (a confirmação humana + `version` garantem
  não-duplicação e detecção de conflito).

## Alternativas Consideradas

- **`IdempotentWrite` para toda escrita, inclusive edições/exclusões.** Desvantagem: diverge do padrão
  vigente (`edit_entry`/`delete_entry` usam `version`+gate) e sobrepõe duas proteções redundantes.
  Rejeitada por inconsistência com o código real.
- **Sem idempotência em `create_recurrence`.** Desvantagem: replay de mensagem duplicaria template
  recorrente (efeito multiplicado a cada mês). Rejeitada por risco.
- **Adicionar origin fields ao DTO do módulo transactions.** Desvantagem: muda contrato de domínio
  fora do escopo (o PRD não cria capacidade nova). Rejeitada.

## Consequências

### Benefícios Esperados

- Consistência total com o padrão já existente e testado no agente.
- Proteção contra replay em criação; detecção de conflito em edição/exclusão via `version`.

### Trade-offs e Custos

- `create_recurrence` precisa de `wamid`/`itemSeq` no schema da tool (origem da mensagem WhatsApp).

### Riscos e Mitigações

- **Risco:** ausência de `wamid` no contexto. **Mitigação:** `wamid`/`itemSeq` vêm do
  `InboundRequest` (já disponível às tools), como nas tools `register_*`.
- **Rollback:** trocar o closure por chamada direta remove a idempotência sem afetar o resto.

## Plano de Implementação

1. Schema de `create_recurrence` inclui `wamid` (string) e `itemSeq` (integer), além dos campos do
   template.
2. Exec usa `IdempotentWrite` com `operation="create_recurrence"`, `resourceKind="recurring_template"`.
3. `update/delete_recurrence` e `update_card` passam `version` no `ConfirmState` e efetivam no resume.

## Monitoramento e Validação

- Métrica `agents_write_total{operation="create_recurrence",outcome}` com `outcome=replay` em replays.
- Teste de integração: replay do mesmo `wamid|itemSeq|create_recurrence` não cria segundo template.

## Impacto em Documentação e Operação

- Documentar a chave de idempotência e o comportamento de replay no runbook.

## Revisão Futura

- Revisitar se novas escritas não originadas de mensagem WhatsApp (sem `wamid`) forem introduzidas.
