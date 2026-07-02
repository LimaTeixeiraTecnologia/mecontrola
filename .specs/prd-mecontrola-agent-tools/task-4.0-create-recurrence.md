# Tarefa 4.0: Tool create_recurrence com IdempotentWrite

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a tool `create_recurrence` em `internal/agents/application/tools/`, envolvendo a escrita em
`IdempotentWrite` para garantir idempotência por `wamid|itemSeq|operation` (ADR-003). A tool é um
adapter fino: valida o input contra o schema, delega a escrita a um closure que chama
`RecurrenceManager.CreateRecurrence`, e mapeia o resultado idempotente para o output tipado.
Depende da Tarefa 2.0 (binding adapters + `RecurrenceManager`). Paralelizável com as Tarefas 3.0 e 5.0.

<requirements>
- RF-15, RF-35
- Dependência: Tarefa 2.0
- Paralelizável com Tarefas 3.0 e 5.0
- Idempotência de escrita via `IdempotentWrite` (ADR-003), chave `wamid|itemSeq|"create_recurrence"`
- Adapter fino: zero regra de negócio, zero SQL, zero branching de domínio (R-ADAPTER-001, R-AGENT-WF-001.2)
- Estados de fronteira como tipos fechados (`ToolOutcome`); `isReplay` derivado de `ToolOutcomeReplay`
- Zero comentários em Go de produção (R-ADAPTER-001.1)
</requirements>

## Subtarefas

- [ ] 4.1 Schema + structs de input/output: campos do template recorrente mais `wamid` (string) e
  `itemSeq` (integer)
- [ ] 4.2 `exec` com `IdempotentWrite.Execute(ctx, userID, wamid, itemSeq, "create_recurrence",
  "recurring_template", writeClosure)`; o closure chama `RecurrenceManager.CreateRecurrence`; mapear
  `IdempotentWriteResult` → output (`isReplay` a partir de `ToolOutcomeReplay`)
- [ ] 4.3 Testes unitários (criação + replay)

## Detalhes de Implementação

Ver techspec.md desta pasta — seção **Tools novas (15) — mapeamento tool → capacidade real**
(linha `create_recurrence`), **Modelos de Dados** (tipos agent-owned `RawRecurrence`/`EntryRef`) e
**ADR-003** (idempotência de novas write tools). Reusar o idioma de `IdempotentWrite` já presente
no módulo `internal/agents`. Não duplicar conteúdo da techspec/ADR — referenciar.

## Critérios de Sucesso

- Teste unitário de criação: `RecurrenceManager.CreateRecurrence` é invocado dentro do closure e o
  output mapeia o `EntryRef` retornado
- Teste unitário de replay: mesmo `wamid|itemSeq|create_recurrence` não duplica a escrita; output
  com `isReplay=true` derivado de `ToolOutcomeReplay`
- Adapter fino verificado: sem SQL direto, sem branching de domínio, sem cálculo de negócio
- Zero comentários em código Go de produção

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tools de escrita e gate destrutivo montam primitivos do substrato internal/platform (tool, workflow) no molde internal/agents.

## Testes da Tarefa

- [ ] Testes unitários (criação + replay: mesmo `wamid|itemSeq|create_recurrence` não duplica, `isReplay=true`)
- [ ] Testes de integração (idempotência de escrita N/A neste escopo — coberta no gate de integração
  da Tarefa 5.0/7.0 quando aplicável; teste de integração de replay recomendado)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/create_recurrence.go`
- `internal/agents/application/usecases/idempotent_write.go` (consumo)
