# Tarefa 4.0: Captura do nome no onboarding com writer único na conclusão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reaproveitar o passo no-op `step-welcome` como o passo de captura do nome (boas-vindas + pergunta), carregar o nome no estado durável e materializá-lo APENAS na conclusão (writer único de working_memory), compondo `## Nome de Tratamento` + `## Objetivo Financeiro` num único Upsert e adicionando `nome_tratamento` ao metadata. Nome opcional nunca bloqueia.

<requirements>
- RF-01: pergunta no início do onboarding integrada à boas-vindas.
- RF-02: extração NL no onboarding.
- RF-03: persistência seção+metadata.
- RF-04: opcional — não bloqueia.
- RF-11: validação ≤40.
- RF-16: métrica de captura.
</requirements>

## Subtarefas

- [ ] 4.1 Adicionar a `OnboardingState` os campos `TreatmentName string `json:"treatmentName"`` e `TreatmentNameAsked bool `json:"treatmentNameAsked"`` (aditivo; compatível com snapshots suspensos).
- [ ] 4.2 Reescrever `BuildWelcomeStep` como captura: 1ª entrada suspende com boas-vindas + "Antes da gente começar, como você gostaria que eu te chamasse? 💚" (mantém `PhaseWelcome`); no resume extrai via `a.Execute` com schema estrito + `DecideTreatmentName`; usável→`state.TreatmentName`; não usável→segue sem nome (RF-04), sem loop de reprompt. Ajustar a copy de abertura do `step-goal` para não re-saudar.
- [ ] 4.3 `BuildConclusionStep`: compor num único `Upsert` a seção `## Nome de Tratamento` (quando presente) + `## Objetivo Financeiro` (sentinel preservado) e adicionar `nome_tratamento` ao `UpsertMetadata` quando presente. NÃO introduzir segundo Upsert de conteúdo (writer único — ADR-001/ADR-003).
- [ ] 4.4 Counter `agents_onboarding_treatment_name_total{outcome ∈ captured|skipped|parse_error}` (espelha `agents_onboarding_monthly_budget_total`), cardinalidade controlada (sem user_id).
- [ ] 4.5 Testes: step suspende/extrai; conclusão compõe as duas seções + metadata; ausência de nome → só objetivo e sem chave; sentinel preservado.

## Detalhes de Implementação

Ver `techspec.md` (Modelos de Dados, Monitoramento) e ADR-001/ADR-003. Precedente carry-in-state: `GoalValueCents` em `onboarding_workflow.go:1046-1048,1567-1569`. Clobber: `Upsert` sobrescreve coluna inteira (`working_memory_repository.go:58-60`); sentinel `## Objetivo Financeiro` (`resolve_onboarding_or_agent.go:78,119`). `BuildConclusionStep` atual em `onboarding_workflow.go:1556-1588`; `BuildWelcomeStep` em `:1009-1015`; `welcomeCombinedPrompt` em `:696-701`.

## Critérios de Sucesso

Primeiro prompt passa a ser boas-vindas+nome; writer único mantido; sentinel intacto; nome opcional não bloqueia; métrica instrumentada; zero comentários; testes verdes; nenhum snapshot suspenso renumerado (PhaseWelcome mantido).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — passo de onboarding no workflow durável com extração via call-site sancionada.
- `domain-modeling-production` — carry-in-state e materialização determinística na conclusão.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários: step + conclusão + opcionalidade. Integração: composição em Postgres coberta na Tarefa 7.0.

## Arquivos Relevantes

- `internal/agents/application/workflows/onboarding_workflow.go` (mod) + testes.
- Consome `DecideTreatmentName` (Tarefa 1.0), `working_memory_sections.go`/consts (Tarefa 2.0).
- Referência: `resolve_onboarding_or_agent.go`, `working_memory_repository.go`.
