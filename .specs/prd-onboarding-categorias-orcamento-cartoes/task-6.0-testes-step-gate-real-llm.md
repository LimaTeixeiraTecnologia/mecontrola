# Tarefa 6.0: Testes de step (mock) + gate real-LLM + gate M-02

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a cobertura de testes: consolidar os testes de step com mocks, atualizar o harness real-LLM
para dirigir a nova jornada (welcome → conclusão, com revisão de resumo e múltiplos cartões) medindo
≥ 0,90 por categoria de extração, e adicionar o gate M-02 (ausência de "renda líquida").

<requirements>
- RF-42: extração de cada categoria (goal, goal_value, monthly_budget, allocation_input, summary_confirm, recurrence, card) ≥ 0,90 por categoria em harness com LLM real, sem baixar a régua.
- Validação end-to-end de RF-39/RF-40 (erros tipados sem falso sucesso) e RF-37 (sem termos internos) via testes.
- M-02 (amplo): 0 ocorrências de qualquer "renda" (`\brenda\b`, case-insensitive) em prompts/erros/schemas/resumo/WM do onboarding — cobre "renda líquida", "renda mensal", "sua renda", "com base na sua renda".
</requirements>

## Subtarefas

- [ ] 6.1 Consolidar/ajustar os testes de step com mocks (`OnboardingWorkflowSuite`, whitebox) para a nova estrutura: welcome, monthly_budget, budget_review (loop), activation, recurrence, cards (loop), conclusion.
- [ ] 6.2 Teste de varredura M-02 (amplo): assert que nenhum prompt/erro/schema/resumo/WM do onboarding contém `\brenda\b` (case-insensitive) — inclui "sua renda"/"com base na sua renda".
- [ ] 6.3 Atualizar o harness real-LLM (`onboarding_workflow_integration_test.go`, `//go:build integration`, `RUN_REAL_LLM=1` + `OPENROUTER_API_KEY`, `AGENT_HARNESS_MODEL`) para dirigir a jornada completa com drive-until-state e invariante semântico (sem keyword frágil), medindo ratio ≥ 0,90 por categoria.
- [ ] 6.4 Verificar/atualizar o teste de resume Postgres (`onboarding_workflow_postgres_resume_integration_test.go`) para dirigir corretamente a NOVA sequência (welcome→goal→monthly_budget→…) e confirmar suspensão/retomada por merge-patch; o arquivo hoje dirige por texto de meta e não referencia `incomeCents`, então o foco é a nova ordem de steps, não um rename de chave.
- [ ] 6.5 Rodar a matriz de validação Go do módulo `internal/agents` (build, vet, test race, lint) e o gate real-LLM.

## Detalhes de Implementação

Ver `techspec.md` "Abordagem de Testes". Harness real-LLM já existe; estendê-lo, não recriar. Evitar
falso-vermelho por brittleness de teste (lição de features anteriores): dirigir por estado-alvo, não
por igualdade de string.

## Critérios de Sucesso

- `go test ./internal/agents/... -count=1 -race` verde (testes de step com mock).
- `go test -tags integration ./internal/agents/application/workflows/... -run RealLLM` com `RUN_REAL_LLM=1`
  atinge ratio ≥ 0,90 por categoria.
- Gate M-02 verde (varredura de termos).
- `golangci-lint run` (quando disponível) sem novos achados; zero comentários em Go de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — testes de workflow/agent do consumidor `internal/agents`, incluindo harness real-LLM com OpenRouter e scorers/gates de extração.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes de step (mock).
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — harness real-LLM.
- `internal/agents/application/workflows/onboarding_workflow_postgres_resume_integration_test.go` — resume/merge-patch.
- `.mockery.yml` — mocks de `BudgetPlanner`/`CardManager` (já presentes).
