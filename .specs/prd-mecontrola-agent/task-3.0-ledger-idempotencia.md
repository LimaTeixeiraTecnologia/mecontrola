# Tarefa 3.0: Ledger de idempotência agent-owned (`agents_write_ledger` + `IdempotentWrite`)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Garantir **escrita exatamente-uma-vez por intenção** (RF-38): retries do agente, loops de tool e reentregas do canal não podem duplicar lançamento. Como o middleware de idempotência atual só persiste 4xx (`internal/platform/idempotency/middleware.go:136-154`), o agente mantém um ledger próprio chaveado por `(wamid, item_seq, operation)`.

<requirements>
- ADR-004: ledger agent-owned com unique `(wamid, item_seq, operation)`; helper `IdempotentWrite`; replay → `ToolOutcomeReplay`.
- Persistência própria do agente (não compartilha transação com outro módulo).
- Cobre: RF-38.
</requirements>

## Subtarefas

- [ ] 3.1 Migration `agents_write_ledger(id, user_id, wamid, item_seq, operation, resource_id, resource_kind, created_at)` com unique `(wamid, item_seq, operation)`.
- [ ] 3.2 Repositório Postgres do ledger (SQL apenas no adapter postgres; `defer func(){_=rows.Close()}()`).
- [ ] 3.3 Helper `IdempotentWrite`: consulta por chave → se existe, retorna `resource_id` como replay; senão executa a escrita e grava o `resource_id`.
- [ ] 3.4 Job de retenção do ledger (análogo ao dedup do WhatsApp), opcional/configurável.

## Detalhes de Implementação

Ver techspec.md → "Modelos de Dados" (`agents_write_ledger`) e ADR-004. `operation` distingue create_transaction/create_card_purchase/edit/delete; `item_seq` distingue múltiplos lançamentos da mesma mensagem (D-22).

## Critérios de Sucesso

- Unique constraint efetiva: dupla execução com mesma chave cria **um único** recurso (teste de concorrência).
- `ToolOutcome` permanece tipo fechado; replay mapeado para `ToolOutcomeReplay` (state-as-type, DMMF).
- Zero comentários em `.go` de produção; SQL só no adapter postgres (R-ADAPTER-001).
- go-implementation R0–R7; build/gofmt verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — idempotência da escrita das tools do agente (caminho Tool→write), persistência agent-owned do substrato.

## Testes da Tarefa

- [ ] Testes unitários: `IdempotentWrite` (miss→executa+grava; hit→replay sem segunda mutação).
- [ ] Testes de integração (testcontainers Postgres): unique sob concorrência; replay reconcilia por `(wamid,item_seq,operation)`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/` (nova `agents_write_ledger`)
- `internal/agents/infrastructure/persistence/` (repositório + `IdempotentWrite`)
- Referência do risco: `internal/platform/idempotency/middleware.go:136-154`
- techspec.md (Modelos de Dados), ADR-004
