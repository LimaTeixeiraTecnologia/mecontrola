# Tarefa 3.0: Reestruturação de `BuildGoalStep` + testes unitários dos 7 cenários

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reestruturar o `BuildGoalStep` para: extração combinada meta+valor; repergunta combinada quando falta o objetivo; repergunta específica de valor quando só falta o valor; guarda "asked once"; avanço independente da resposta de valor. O objetivo permanece obrigatório (zero regressão). Cobrir os 7 cenários com testes unitários (mock de `agent.Agent`).

<requirements>
- RF-01: meta+valor juntos → extração combinada, sem repergunta.
- RF-02: valor opcional nunca bloqueia o avanço.
- RF-03: objetivo permanece obrigatório (loop até objetivo válido, bounded por `MaxAttempts`); zero regressão.
- RF-03.1: sem objetivo válido → repergunta combinada (objetivo + valor opcional).
- RF-03.2: objetivo válido sem valor → repergunta específica de valor.
- RF-03.3: valor perguntado no máximo uma vez por execução do step.
- RF-04: após a repergunta de valor, avança independentemente (válido salva; recusa/não-numérico avança sem valor).
- RF-05: valor inválido (negativo/zero/não numérico) = "não informado", sem erro técnico.
- RF-06: recusa já na 1ª mensagem aplica a regra uniforme (no máx. uma repergunta de valor).
- RF-13.1: valor capturado inline não gera eco por campo; avança direto ao próximo step.
</requirements>

## Subtarefas

- [ ] 3.1 Reescrever `BuildGoalStep` (~L492-521) conforme a closure da techspec: branch `Goal==""` (extração combinada via `goalWithValueSchema`) e branch value-only (`goalValueSchema`), com `GoalValueAsked` guardando a cota única.
- [ ] 3.2 Garantir o único `completeStep` do branch `Goal==""` DEPOIS de `state.Goal = goal` (invariante de meta obrigatória).
- [ ] 3.3 Testes unitários com mock de `agent.Agent` (testify/suite, whitebox) cobrindo os 7 cenários da tabela de branch trace da techspec.
- [ ] 3.4 Teste explícito de não-regressão: nenhuma combinação de valor completa o step com `Goal==""`.

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave" (closure completa de `BuildGoalStep`), "Fluxo de Dados" e a tabela "Branch trace". Consome o constructor da 1.0 e os schemas/prompts da 2.0. LLM só nas duas call-sites de parse sancionadas (R-AGENT-WF-001.4). Estado de espera persistido no `Snapshot` via `suspendStep` antes de repergunta (R-AGENT-WF-001.7).

## Critérios de Sucesso

- Os 7 cenários passam; meta obrigatória preservada (zero regressão).
- Valor perguntado no máximo uma vez; avanço garantido após a repergunta.
- `go build`, `go vet`, `go test -race ./internal/agents/...` verdes; zero comentários.
- Testes seguem R-TESTING-001 (whitebox, testify/suite, `fake.NewProvider()`, mock por IIFE).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — reestruturação de step de workflow com suspend/resume, call-sites de parse LLM e pending state no `Snapshot` (R-AGENT-WF-001).
- `domain-modeling-production` — invariante de meta obrigatória e semântica de estado (asked-once) sem vazar validação para o step.
- `design-patterns-mandatory` — gate confirmou "sem State pattern"; branching em closure com flag de tipo fechado.
- `go-testing` — suite whitebox testify com mock de `agent.Agent` para os 7 cenários.

## Testes da Tarefa

- [ ] Testes unitários (7 cenários do branch trace + não-regressão de meta)
- [ ] Testes de integração (extração real coberta pela 5.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `BuildGoalStep` (~L492).
- `internal/platform/agent/ports.go` — `agent.Request`/`agent.Result.RawJSON`.
- `internal/platform/workflow/step.go` — `StepOutput[S]`.
- Teste: `internal/agents/application/workflows/onboarding_workflow_test.go`.
