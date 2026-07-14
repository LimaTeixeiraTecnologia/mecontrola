# Tarefa 5.0: Métrica de outcome da distribuição e wiring de observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Instrumentar o passo de distribuição com um contador de outcome de cardinalidade controlada e ligar a observabilidade nos construtores do workflow. Habilita as métricas de sucesso do PRD.

<requirements>
- RF-16: emitir contador `agents_onboarding_distribution_total` com rótulo fechado `outcome`, sem `user_id`/`category_id`.
- CS-01, CS-02, CS-03, CS-04: critérios de sucesso mensuráveis medidos por este outcome (conclusão, abandono, acerto até 2ª tentativa) e pelos sinais existentes.
</requirements>

## Subtarefas

- [ ] 5.1 Criar o contador `agents_onboarding_distribution_total` (unidade `"1"`) via `o11y.Metrics().Counter(...)`, espelhando `budget_creation_continuer.go`.
- [ ] 5.2 Threading de `observability.Observability`/`observability.Counter` para o passo: ampliar `BuildBudgetReviewStep` e `BuildOnboardingWorkflow`; ligar em `internal/agents/module.go` a partir de `deps.O11y`.
- [ ] 5.3 Incrementar exatamente um `outcome` por caminho de retorno dos handlers: `personalize_entered`, `accepted_default`, `accepted_values`, `over`, `under`, `mixed_unit`, `tolerance_absorbed` (precedência `tolerance_absorbed` sobre `accepted_values`).
- [ ] 5.4 Guarda nil-safe do contador para testes com `fake.NewProvider()`.

## Detalhes de Implementação

Ver `techspec.md` seção "Monitoramento e Observabilidade" e ADR-004. Padrão real: `internal/agents/application/usecases/budget_creation_continuer.go:36-65`. Rótulos permitidos apenas de baixa cardinalidade (R-TXN-004, R-AGENT-WF-001.5). Zero comentários.

## Critérios de Sucesso

- Cada um dos 7 valores de `outcome` é emitido no caminho correto; precedência `tolerance_absorbed` respeitada (RF-16).
- Nenhum rótulo carrega `user_id`/`category_id`; cardinalidade fechada.
- Métricas existentes (`agents_onboarding_*` completed/resumed, reaper) preservadas.
- `go build ./...`/`go vet ./...` verdes com as assinaturas ampliadas; contador nil-safe não quebra testes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — instrumentação de Run/outcome no consumidor `internal/agents` com cardinalidade controlada e wiring no `module.go`.

## Testes da Tarefa

- [ ] Testes unitários: assert via `fake.FakeMetrics` de que cada caminho emite o `outcome` esperado (incluindo precedência `tolerance_absorbed`). Package whitebox testify/suite.
- [ ] Testes de integração: não aplicável nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (`BuildBudgetReviewStep`, `BuildOnboardingWorkflow`, incrementos)
- `internal/agents/module.go` (wiring de `observability.Observability`)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (teste de métrica)
