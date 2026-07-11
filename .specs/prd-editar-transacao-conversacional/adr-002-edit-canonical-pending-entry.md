# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Caminho canônico de edição no `pending-entry`; alargar `edit_entry` e governá-la como write-tool
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da techspec, aprovação do solicitante (múltipla escolha, recomendação aceita)
- **Relacionados:** PRD (D-04, RF-05..08, RF-31), techspec.md, `.claude/rules/agent-workflows-tools.md`, `.claude/rules/go-adapters.md`

## Contexto

Existem dois caminhos de edição no código: (a) o **vivo** — `edit_entry` tool → `RegisterAttempt.EditEntry` → `PendingEntryState{OperationKind: PendingOpEditEntry}` → pending-entry workflow com confirmação em `AwaitingSlotConfirmation` e `callLedger` roteando `PendingOpEditEntry → ledger.UpdateTransaction(buildRawUpdate(state))` (`pending_entry_workflow.go:617-626`); e (b) um **legado/alternativo** — `destructive_confirm.OpEditEntry` com `executeEditEntry` (`destructive_confirm_workflow.go:313-329`), aparentemente não acionado pelo wiring atual. O schema da tool `edit_entry` só expõe `entryId/amountCents/description/occurredAt` (`edit_entry.go:15-20`) e `RegisterAttempt.EditEntry` copia os demais campos do lançamento atual — logo não há como editar categoria, forma de pagamento, cartão, parcelas ou direção por conversa. Além disso, `edit_entry` não está em `WithWriteToolSet` (`module.go:235`), ficando fora da malha de write-guard/scorer.

## Decisão

1. Declarar o **pending-entry** (`PendingOpEditEntry`) como o único caminho canônico de edição conversacional. O `destructive_confirm.OpEditEntry` permanece intocado, marcado como não-canônico para edição (dívida técnica); nenhum roteamento novo passa por ele.
2. Alargar o schema de `edit_entry` para expor os campos de paridade (`paymentMethod`, `cardId`/apelido, `installments`, `categoryId`/`subcategoryId` ou termo de categoria, `direction`, além de `version`), mantendo a tool como adapter fino (validação de schema + mapeamento para `EditEntryCommand`, sem regra/SQL/branching de domínio).
3. Propagar os campos por `EditEntryCommand → PendingEntryState → buildRawUpdate`; disparar re-resolução de categoria via `classify_category` quando categoria ou direção mudam (kind compatível).
4. Incluir `edit_entry` em `WithWriteToolSet`, submetendo a edição à mesma política de write-tool, guard e contabilização de idempotência/scorer que `register_expense`/`register_income`.

## Alternativas Consideradas

- **Novo workflow dedicado de edição:** duplicaria o slot-filling/confirmação já provados no pending-entry, contra o princípio de reuso e a economia de contexto. Rejeitada.
- **Manter `edit_entry` fora do `WithWriteToolSet`:** deixaria a edição fora do write-guard/scorer (ponto cego de governança). Rejeitada.
- **Adotar o caminho `destructive_confirm.OpEditEntry`:** exigiria migrar o fluxo vivo para um caminho legado e duplicar slot-filling. Rejeitada.

## Consequências

### Benefícios Esperados

- Paridade total de campos reusando o workflow vivo; sem novo padrão estrutural (selector = reject).
- Edição coberta por write-guard, idempotência e scorer `write_persistence_accuracy`.

### Trade-offs e Custos

- `destructive_confirm.OpEditEntry` remanescente como dead path (dívida técnica documentada).
- Schema da tool maior; instruções do agente ajustadas para os campos novos.

### Riscos e Mitigações

- Risco: LLM preencher campos inaplicáveis (ex.: pagamento em receita). Mitigação: `RegisterAttempt.EditEntry`/`DecideUpdate` ignoram/rejeitam por invariante; golden de receita.
- Rollback: reverter o alargamento do schema volta ao comportamento atual (3 campos).

## Plano de Implementação

1. Alargar `EditEntryInput`/schema + `exec`.
2. `EditEntryCommand`/`PendingEntryState`/`buildRawUpdate` + re-resolução de categoria.
3. Incluir `edit_entry` em `WithWriteToolSet`.
4. Golden real-LLM dos campos novos.

## Monitoramento e Validação

- Scorer `write_persistence_accuracy` verde para `operation=edit_entry` (`Routed`/`Reconciled`).
- Gate grep: sem `switch case intent.Kind`; sem SQL/comentário nas tools; estados fechados.
- Sucesso: editar cada campo por conversa resulta em `UpdateTransaction` correto e confirmado.

## Impacto em Documentação e Operação

- Atualizar instruções do agente (`mecontrola_agent.go`) para os campos de edição; documentar o dead path.

## Revisão Futura

- Remover `destructive_confirm.OpEditEntry` quando houver janela para limpeza, com teste de regressão confirmando ausência de acionamento.
