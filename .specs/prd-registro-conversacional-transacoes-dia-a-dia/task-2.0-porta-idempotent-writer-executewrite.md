# Tarefa 2.0: Porta `IdempotentWriter` + integração de idempotência em `executeWrite`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o gap crítico: envolver a escrita (`callLedger`) em `IdempotentWrite` dentro de `executeWrite`, usando a chave `(state.MessageID, state.ItemSeq, state.OperationKind.String())` — wamid **original** (RF-20). Para não criar ciclo de import (`usecases` já importa `workflows`), declarar uma porta consumidor-side no pacote `workflows` retornando apenas primitivos + `agent.ToolOutcome`.

<requirements>
- RF-19: toda escrita passa por `IdempotentWrite`; integrar em `executeWrite` antes de `ledger.CreateTransaction`/`CreateRecurringTemplate`.
- RF-20: chave ancorada no wamid original (`state.MessageID`), não na mensagem de confirmação.
- ADR-001: porta `IdempotentWriter` + `IdempotentWriteFn` retornam primitivos; `resourceKind` determinístico; replay → `ToolOutcomeReplay` sem 2º INSERT, completando com o mesmo texto de sucesso.
- Sem alterar o kernel `internal/platform/workflow` (R-WF-KERNEL-001); mudança só no consumidor.
</requirements>

## Subtarefas

- [ ] 2.1 Declarar em `workflows`: `type IdempotentWriteFn func(ctx context.Context) (uuid.UUID, bool, error)` e `type IdempotentWriter interface { Execute(ctx, userID, wamid, itemSeq, operation, resourceKind, IdempotentWriteFn) (uuid.UUID, agent.ToolOutcome, error) }`.
- [ ] 2.2 Adicionar `idem IdempotentWriter` como parâmetro de `BuildPendingEntryWorkflow`.
- [ ] 2.3 Em `executeWrite`, envolver `callLedger` numa `IdempotentWriteFn` e chamar `idem.Execute(...)`; mapear `resourceKind(state)` (`recurring_template` para recorrência, `transaction` caso contrário).
- [ ] 2.4 Tratar o retorno: replay (`ToolOutcomeReplay`) completa o run com o texto de sucesso renderizado de `state`, sem segundo INSERT; primeira escrita segue o fluxo atual.
- [ ] 2.5 Atualizar `newPEHarness` para injetar um fake `IdempotentWriter` (sobre um `WriteLedger` fake in-memory), mantendo os testes existentes verdes.

## Detalhes de Implementação

Ver `techspec.md` › **Interfaces Chave** (assinatura da porta e de `BuildPendingEntryWorkflow`) e **ADR-001** (motivação do ponto de integração e do anti-ciclo). Não duplicar.

## Critérios de Sucesso

- `executeWrite` executa a escrita exatamente uma vez por `(wamid_original, itemSeq, operation)`; a 2ª execução da mesma chave retorna replay sem novo INSERT.
- `workflows` não importa `usecases` (sem ciclo); a porta retorna primitivos + `agent.ToolOutcome`.
- Harness compila e os cenários G7 existentes permanecem verdes.
- `go build`/`go vet` limpos; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera o workflow durável do consumidor agentivo e a escrita financeira idempotente (gatilho explícito da skill).

## Testes da Tarefa

- [ ] Testes unitários (mapeamento `resourceKind`; replay completa com texto de sucesso)
- [ ] Testes de integração (harness in-memory: 1 INSERT para 2 execuções da mesma chave — reforçado em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_workflow.go` — `BuildPendingEntryWorkflow`, `executeWrite`, `callLedger`, `resourceKind`, porta.
- `internal/agents/application/usecases/idempotent_write.go`, `write_ledger.go` — `IdempotentWrite`, `WriteFn` (consumidos).
- `internal/agents/application/agents/pending_entry_harness_test.go` — injeção do fake.
