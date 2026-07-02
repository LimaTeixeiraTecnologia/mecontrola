# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estratégia de idempotência/concorrência das novas tools de escrita
- **Data:** 2026-07-02
- **Status:** Aceita (emenda spec-version 3 em 2026-07-02)
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD (RF-15, RF-16, RF-17, RF-18, RF-35, RF-37, RF-40); techspec; ADR-001; ADR-005

## Emenda spec-version 3 (2026-07-02) — identidade/idempotência injetadas server-side [corrige premissa falsa]

A versão original desta ADR assumia que `create_recurrence` receberia `wamid`/`itemSeq` "vindos do
`InboundRequest` (já disponível às tools)" **via schema da tool**. Essa premissa está **incorreta** à
luz da evidência de produção (PRD, seção `Evidência de Produção`, EP-01/EP-05): no código atual esses
campos são **argumentos obrigatórios do LLM** — `internal/agents/application/tools/register_expense.go:52`
lista `wamid`/`itemSeq`/`userId` como `required` com `Strict:true` — e o runtime **nunca os injeta**
(`internal/platform/agent/runtime.go:173-193`, `buildMessages`, não propaga `in.ResourceID`/
`in.MessageID`; `internal/platform/agent/agent.go:198-219`, `invokeToolCall`, marshalla `tc.ArgumentsJSON`
crus). Resultado comprovado: o modelo não fornece esses valores corretamente e a escrita se perde,
com sucesso alucinado.

Correção (RF-37, alinhada à ADR-005): `userId`/`wamid`/`itemSeq` DEVEM ser injetados **server-side** no
ponto de invocação da tool (`invokeToolCall`, `internal/platform/agent`), a partir do `InboundRequest`/
contexto do Run, e **removidos do schema exposto ao LLM**. A idempotência (`IdempotentWrite`,
`internal/agents/application/tools/idempotent_write.go`) permanece com a chave
`(userID, wamid, itemSeq, operation)`, agora **alimentada pelos valores injetados**, não pelo modelo.
Isto vale para toda tool de escrita/leitura por usuário, incluindo `create_recurrence`. Os trechos
abaixo que citam esses campos no schema da tool devem ser lidos sob esta emenda.

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

- `create_recurrence` depende de `wamid`/`itemSeq`/`userId` para a chave de idempotência, mas esses
  valores **não** aparecem no schema da tool: são injetados server-side no `invokeToolCall` a partir do
  `InboundRequest`/contexto do Run (emenda spec-version 3, RF-37, ADR-005). O closure de escrita segue
  independente do DTO do módulo.

### Riscos e Mitigações

- **Risco:** ausência de `wamid`/`itemSeq`/`userId` no contexto do Run. **Mitigação:** a injeção
  server-side (RF-37, ADR-005) garante esses valores a partir do `InboundRequest`; o guard
  anti-simulação (RF-38, ADR-005) impede reportar sucesso quando a escrita não ocorre. Confiar no LLM
  para fornecer esses valores é PROIBIDO.
- **Rollback:** trocar o closure por chamada direta remove a idempotência sem afetar o resto.

## Plano de Implementação

1. Schema de `create_recurrence` expõe ao LLM **apenas** os campos do template; `wamid`/`itemSeq`/
   `userId` são removidos do schema e injetados server-side no `invokeToolCall` (RF-37, ADR-005).
2. Exec usa `IdempotentWrite` com `operation="create_recurrence"`, `resourceKind="recurring_template"`,
   com a chave `(userID, wamid, itemSeq, operation)` alimentada pelos valores injetados.
3. `update/delete_recurrence` e `update_card` passam `version` no `ConfirmState` e efetivam no resume.

## Monitoramento e Validação

- Métrica `agents_write_total{operation="create_recurrence",outcome}` com `outcome=replay` em replays.
- Teste de integração: replay do mesmo `wamid|itemSeq|create_recurrence` não cria segundo template.

## Impacto em Documentação e Operação

- Documentar a chave de idempotência e o comportamento de replay no runbook.

## Revisão Futura

- Revisitar se novas escritas não originadas de mensagem WhatsApp (sem `wamid`) forem introduzidas.
