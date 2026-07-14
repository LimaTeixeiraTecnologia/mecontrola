# Tarefa 7.0: Validação de não-regressão — integração, golden real-LLM e gates

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o ciclo provando que não houve regressão em hipótese nenhuma: estender a integração de resume para o novo sub-estado, cobrir os comportamentos com golden real-LLM, e executar o gate completo de validação e as invariantes NR-01..NR-08.

<requirements>
- RF-12: nenhum caminho atual regride (aceite "sim", valores válidos, reabertura no resumo, soma inválida sem ativação parcial).
- RF-17: rollout direto, sem feature flag nem toggle de ambiente.
</requirements>

## Subtarefas

- [x] 7.1 Estender `onboarding_workflow_postgres_resume_integration_test.go` com um ciclo suspend→resume no `reviewAwaitPersonalize`, provando que o novo sub-estado persiste e retoma por merge-patch.
- [x] 7.2 Adicionar golden real-LLM (`RUN_REAL_LLM=1`) cobrindo: recusa→personalizar, valores em reais/percentual válidos, over, under, categoria zerada com aviso, valores por extenso, tolerância de arredondamento, unidades misturadas.
- [x] 7.3 Confirmar RF-17: nenhuma feature flag/env toggle foi introduzida para esta mudança.
- [x] 7.4 Executar o gate completo (NR-07) e registrar evidência de cada item.

## Detalhes de Implementação

Ver `techspec.md` seções "Abordagem de Testes" e "Garantia de Não-Regressão" (NR-01..NR-08). Validação real com credenciais OpenRouter (`.env`), pois mocks não exercitam a extração real. Build tag `//go:build integration` para testes de banco.

## Critérios de Sucesso

- NR-01..NR-08 verificadas: caminhos felizes idênticos, extração compartilhada preservada, budget_creation sem regressão, over/under sem ativação, reabertura preservada, sem vazar "renda", kernel intocado.
- Gate completo verde (NR-07): `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race` (`internal/agents/...`, `internal/platform/...`), `golangci-lint run`, `mockery --config mockery.yml --dry-run`, greps R0/R1/R5/R7 e gates `.claude/rules/` (R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001).
- Golden real-LLM dos 8 comportamentos passam de forma determinística.
- RF-17 confirmado: ausência de feature flag.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — validação end-to-end do fluxo agentivo (Thread→Run, suspend/resume) e dos comportamentos golden real-LLM no substrato.

## Testes da Tarefa

- [x] Testes unitários: consolidação/execução da suíte completa dos pacotes afetados com `-race`.
- [x] Testes de integração: resume Postgres do `reviewAwaitPersonalize` (`//go:build integration`) e golden real-LLM (`RUN_REAL_LLM=1`) dos 8 comportamentos.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow_postgres_resume_integration_test.go` (resume do novo sub-estado)
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`, `budget_creation_workflow_real_llm_test.go` (golden real-LLM)
- `internal/agents/application/workflows/onboarding_workflow.go`, `budget_creation_workflow.go` (alvos da validação)
