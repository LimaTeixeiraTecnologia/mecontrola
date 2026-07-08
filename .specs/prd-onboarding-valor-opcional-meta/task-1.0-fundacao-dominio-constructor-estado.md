# Tarefa 1.0: Fundação de domínio — constructor puro `DecideGoalValueCents` + campos de estado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a base de domínio da funcionalidade: o smart constructor puro do valor da meta (ausência é válida) e os dois campos novos no `OnboardingState` que carregam o valor e a flag "asked once" pelo `Snapshot.State` do kernel. Sem migration, sem IO.

<requirements>
- RF-07: converter valor informado para inteiro em centavos, aceitar qualquer positivo; ausência/zero/negativo = "não informado", sem teto.
- RF-08: validação em smart constructor puro NOVO e distinto de `DecideIncomeCents`, com semântica "ausência é válida".
- RF-10: o valor sobrevive no `OnboardingState` do `step-goal` ao `step-conclusion`.
- ADR-002: estado como `int64` sentinela (`0`=ausente) + flag booleana `GoalValueAsked`; SEM `omitempty`; sem `Option`/`Result`/`Either`/currying/DSL.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `func DecideGoalValueCents(hasAmount bool, amountBRL float64) (int64, bool)` junto a `DecideGoal`/`DecideIncomeCents` (`internal/agents/application/workflows/onboarding_workflow.go` ~L161-174). Puro: `!hasAmount || amountBRL <= 0` → `(0, false)`; senão `(round(amountBRL*100), true)`.
- [ ] 1.2 Adicionar campos `GoalValueCents int64 \`json:"goalValueCents"\`` e `GoalValueAsked bool \`json:"goalValueAsked"\`` ao `OnboardingState` (~L146-159). SEM `omitempty` (espelha `IncomeCents`/`CardsDone`).
- [ ] 1.3 Teste unitário puro (tabela input→output) de `DecideGoalValueCents` cobrindo `(true,400000)→(40000000,true)`, `(true,0.01)→(1,true)`, `(true,0)→(0,false)`, `(true,-50)→(0,false)`, `(false,400000)→(0,false)`.
- [ ] 1.4 Teste de regressão de resume (Risco R1): merge-patch parcial `{"resumeText":"..."}` sobre `Snapshot.State` com `goalValueCents>0`/`goalValueAsked=true` preserva ambos os campos.

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave", "Modelos de Dados" e ADR-002 (`adr-002-estado-valor-sentinela-flag.md`). Constructor e campos verbatim na techspec. O sentinela `0` é seguro porque o domínio do valor é estritamente positivo (RF-07). O `bool` de retorno é comma-ok idiomático, não `Option`.

## Critérios de Sucesso

- `DecideGoalValueCents` é puro, determinístico, sem `context.Context`/IO, testável sem mock.
- Campos serializam no snapshot e sobrevivem a suspend/resume por merge-patch parcial.
- `go build ./...`, `go vet ./...`, `go test -race ./internal/agents/...` verdes; zero comentários (R-ADAPTER-001.1).
- Nenhuma alteração em `DecideIncomeCents` (zero regressão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — smart constructor puro, state-as-type e "make illegal states unrepresentable" para o valor sentinela e a flag.
- `design-patterns-mandatory` — gate confirmou factory-function/smart-constructor (sem padrão de catálogo); registrar a decisão.
- `mastra` — os campos vivem no `OnboardingState` serializado no `Snapshot.State` do kernel de workflow; contrato de suspend/resume por merge-patch.
- `go-testing` — testes unitários puros e o teste de preservação em merge-patch.

## Testes da Tarefa

- [ ] Testes unitários (constructor puro; preservação em merge-patch)
- [ ] Testes de integração (não aplicável nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — constructor (~L161) e struct `OnboardingState` (~L146).
- `internal/platform/workflow/codec.go` — `MergePatch` (referência do contrato de resume).
- Teste: `internal/agents/application/workflows/onboarding_workflow_test.go` (ou arquivo de teste do pacote).
