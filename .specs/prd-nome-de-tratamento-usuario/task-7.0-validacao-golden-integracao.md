# Tarefa 7.0: Validação: golden, invariantes, integração Postgres e gate real-LLM

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a validação production-ready: categoria golden dedicada + casos, stub de tool no catálogo, invariante unit de composição da working memory (sentinel preservado), teste de integração Postgres (working_memory + metadata + sentinel) e execução do gate real-LLM ≥0,90 por categoria com 0 falso-sucesso, reutilizando os scorers de tom existentes.

<requirements>
- RF-05: uso natural do nome — scorer de tom.
- RF-10: isolamento — integração assere que só platform_resources muda.
- RF-12: Tom de Voz — scorers.
- RF-14: gate real-LLM ≥0,90 e 0 falso-sucesso.
- RF-16: métricas/KPIs de qualidade via scorers.
</requirements>

## Subtarefas

- [ ] 7.1 Em `internal/agents/application/golden/case.go`: novo const fechado `CategoryTreatmentName`, estendendo `IsValid()` e `AllCategories()`; incluir na lista `required` de `registry_test.go`.
- [ ] 7.2 Criar `internal/agents/application/golden/cases_treatment_name.go` (builder) e dar `append` em `AllCases()` (`registry.go`): casos "alterar com nome" (ExpectedTool `edit_treatment_name`, ExpectedArgs `{name}`), "alterar sem nome" (ExpectedTool `edit_treatment_name`, sem arg), "confirmação no tom" (ResponseProperty asterisco simples + emoji oficial; `Metadata["requires_brand_emoji"]=true`).
- [ ] 7.3 Registrar stub `edit_treatment_name` no `goldenToolCatalog` (`harness_realllm_test.go`).
- [ ] 7.4 Invariante unit estilo `journey_test.go` (pura, sem LLM): composição da working memory da conclusão do onboarding preserva o sentinel `## Objetivo Financeiro` e inclui `## Nome de Tratamento`.
- [ ] 7.5 Teste de integração (`//go:build integration`, testcontainers) estendendo a suíte de working memory: pós-onboarding com nome, `SELECT working_memory, metadata->>'nome_tratamento'` retorna ambas as seções e a chave; edição substitui a seção preservando o objetivo; nenhuma alteração em tabelas de identity (RF-10).
- [ ] 7.6 Executar o gate real-LLM: `RUN_REAL_LLM=1 OPENROUTER_API_KEY=... go test -tags integration ./internal/agents/application/golden/ -run TestGoldenSetGate` e confirmar `CategoryTreatmentName` ≥ `goldenGateThreshold` (0.90) nas 3 repetições, com scorers `tone_adherence` + `verbatim_tone_adherence` + `no_hallucination` (0 falso-sucesso).

## Detalhes de Implementação

Ver `techspec.md` (Abordagem de Testes) e ADR-004. Harness `harness_realllm_test.go:24-55` (env), `:24` threshold 0.90, `:390` 3 repetições, gate por categoria `:392-426`. `Case` em `case.go:79-98`; `validate.go` regras; scorers `behavioral_scorers.go:307-346` (verbatim tom), `mecontrola_scorers.go:181-183` (tone_adherence), `no_hallucination` (0 falso-sucesso).

## Critérios de Sucesso

Categoria/casos válidos (`ValidateAll`); invariante e integração verdes; gate real-LLM ≥0,90 por categoria; 0 falso-sucesso; RF-10 asserido; zero comentários.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — casos golden, scorers de tom e gate real-LLM do substrato de agents.
- `postgresql-production-standards` — teste de integração sobre platform_resources (working_memory + metadata + sentinel).

## Testes da Tarefa

- [ ] Testes unitários (RegistrySuite/JourneyGoldenSuite/invariante)
- [ ] Testes de integração (Postgres testcontainers + gate real-LLM)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/application/golden/case.go` (mod)
- `internal/agents/application/golden/registry.go` (mod)
- `internal/agents/application/golden/registry_test.go` (mod)
- `internal/agents/application/golden/cases_treatment_name.go` (novo)
- `internal/agents/application/golden/harness_realllm_test.go` (mod)
- `internal/agents/application/golden/journey_test.go` (invariante)
- Teste de integração de working memory (`internal/platform/memory/infrastructure/postgres/working_memory_repository_integration_test.go` ou vizinho)
