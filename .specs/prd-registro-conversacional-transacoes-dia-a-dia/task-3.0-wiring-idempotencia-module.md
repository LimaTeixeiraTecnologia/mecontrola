# Tarefa 3.0: Wiring de produção da idempotência no `module`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar a idempotência ao caminho produtivo. Construir o `IdempotentWrite` a partir do `writeLedgerRepo` já existente, adaptá-lo à porta `workflows.IdempotentWriter` com um adapter fino, e injetá-lo em `BuildPendingEntryWorkflow`. Hoje `IdempotentWrite` só é instanciado em testes — este passo o ativa em produção.

<requirements>
- RF-19: `IdempotentWrite` acionado no fluxo produtivo (não só no job de retenção).
- RF-20: chave via wamid original preservada de ponta a ponta.
- ADR-001: adapter fino `*usecases.IdempotentWrite → workflows.IdempotentWriter` (desestrutura o result; sem lógica de negócio).
- R-ADAPTER-001: adapter fino, zero comentários.
</requirements>

## Subtarefas

- [ ] 3.1 Criar o adapter `idempotentWriterAdapter` (satisfaz `workflows.IdempotentWriter`) desestruturando `IdempotentWriteResult` em `(ResourceID, Outcome, err)`, com `usecases.WriteFn(write)`.
- [ ] 3.2 No `module.go`, construir `usecases.NewIdempotentWrite(writeLedgerRepo, deps.O11y)` (reusar o `writeLedgerRepo` de module.go:139).
- [ ] 3.3 Passar o adapter para `BuildPendingEntryWorkflow(txLedger, cardManager, categoriesReader, idemAdapter)` (module.go:193).
- [ ] 3.4 Confirmar que `PurgeLedger` continua funcionando (mesma tabela; nenhuma regressão no job de retenção).

## Detalhes de Implementação

Ver `techspec.md` › **Interfaces Chave** (adapter) e **Sequenciamento** (passo 3), e **ADR-001** › Plano de Implementação. Não duplicar.

## Critérios de Sucesso

- Em produção, uma escrita confirmada grava exatamente uma linha em `mecontrola.agents_write_ledger` por chave; reprocessamento não duplica.
- `module.go` compila com a nova assinatura; sem ciclo de import.
- `go build ./...` e `go vet ./internal/agents/...` limpos; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring do runtime agentivo (`module.go`) e da escrita financeira idempotente do stack Mastra Go.

## Testes da Tarefa

- [ ] Testes unitários (adapter desestrutura result corretamente)
- [ ] Testes de integração (fluxo module→workflow com fake ledger; reforçado no harness em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go` — construção do `IdempotentWrite`, adapter e injeção.
- `internal/agents/application/workflows/pending_entry_workflow.go` — consumidor da porta.
- `internal/agents/application/usecases/idempotent_write.go`, `purge_ledger.go` — use case e job de retenção.
