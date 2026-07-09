# Tarefa 8.0: Testes de integração, harness real-LLM e regressão determinística do incidente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a entrega com a suíte de validação de fronteira de IO e comportamento conversacional do cadastro de cartão: testes de integração (Postgres via testcontainers) do ciclo suspend→resume, harness real-LLM dos cenários Gherkin do PRD com gate estatístico ≥ 0.90, e o teste determinístico de regressão do incidente de 2026-07-08 (nenhuma afirmação de cadastro sem tool call; falha sempre com erro real persistido no run e no log). Depende da tarefa 7.0 (fluxo completo wired: tool, workflow, continuer, reaper, resume chain e instruções do agente). Ver `techspec.md` §Abordagem de Testes e o mapeamento Requisito→Decisão→Teste.

<requirements>
- Integração (`//go:build integration`, testcontainers Postgres): ciclo completo suspend→resume "sim" cria cartão para banco reconhecido (fechamento derivado) e banco não reconhecido (fechamento explícito); replay de "sim" (mesmo wamid) NÃO cria segundo cartão (RF-14); apelido duplicado → `ErrNicknameConflict` → mensagem acionável, sem duplicata (RF-12); falha de infra → `run.error` preenchido, nunca silenciosa (RF-15); exclusão mútua: `card-create` não inicia com outro gate suspenso (RF-18); TTL 15 min expirado → cancela, texto segue ao `ParseInbound` (RF-04).
- Harness real-LLM (`RUN_REAL_LLM=1` com `.env` OPENROUTER_*) dos cenários Gherkin do PRD com scorer e gate estatístico ≥ 0.90 (RF-22), no estilo de `pending_entry_harness_test.go`.
- Teste determinístico de regressão do incidente (RF-13/RF-15): pedido de cadastro nunca responde sucesso/falha sem `create_card` tool call; qualquer falha persiste erro real no run e no log estruturado — nunca erro vazio.
- Requisitos cobertos: RF-04, RF-12, RF-13, RF-14, RF-15, RF-18, RF-22.
- Validação real-LLM é mandatória para mudanças do agente: `RUN_REAL_LLM=1` com `.env` (OPENROUTER_*); mocks não bastam.
- Conformidade com R-TESTING-001 (whitebox `package workflows`, `fake.NewProvider()`, mocks do `.mockery.yml`) exceto nos `*_integration_test.go`, que podem usar `package workflows_test`.
</requirements>

## Subtarefas

- [ ] 8.1 Testes de integração `card_create_confirm_workflow_integration_test.go` (`//go:build integration`, testcontainers Postgres) cobrindo suspend→resume "sim" (banco reconhecido/derivado e não reconhecido/explícito), replay de "sim" idempotente (RF-14), apelido duplicado → `ErrNicknameConflict` (RF-12), falha de infra populando `run.error` (RF-15), exclusão mútua com outro gate suspenso (RF-18) e TTL 15 min expirado (RF-04). Padrão de `onboarding_workflow_integration_test.go`.
- [ ] 8.2 Harness real-LLM `card_create_harness_test.go` (`RUN_REAL_LLM=1`, `.env` OPENROUTER_*) dos cenários Gherkin do PRD (fluxo feliz, banco não reconhecido, slot-filling, confirmação negada, ambiguidade×2, TTL, apelido duplicado, dia inválido, regressão) com scorer e gate estatístico ≥ 0.90 (RF-22), no estilo de `pending_entry_harness_test.go`.
- [ ] 8.3 Teste determinístico de regressão do incidente (RF-13/RF-15): sem tool call `create_card` não há resposta de sucesso/falha de cadastro; toda falha persiste erro real no run e no log estruturado (nunca erro vazio).

## Detalhes de Implementação

Ver `techspec.md` §Abordagem de Testes (Testes de Integração, Testes E2E) e §Mapeamento Requisito → Decisão → Teste (linhas RF-04, RF-12, RF-13, RF-14, RF-15, RF-18, RF-22). Reusar o mecanismo de escrita e idempotência descrito em §Idempotência, Auditoria e Métrica (`IdempotentWriter`, `operation="create_card"`, `wamid`) e a ordem de resume de §Exclusão Mútua e Ordem de Resume. O guardrail anti-alucinação validado aqui está descrito em §Guardrail Anti-Alucinação (RF-13). Não duplicar prosa da techspec.

Padrões de referência existentes:
- `internal/agents/application/workflows/pending_entry_harness_test.go` — harness real-LLM, scorer, gate estatístico e store in-memory.
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — `//go:build integration`, testcontainers Postgres, `package workflows_test`.

## Critérios de Sucesso

- `build`, `vet`, `test -race` e `lint` verdes em `internal/agents`.
- Harness real-LLM com gate estatístico ≥ 0.90 (RF-22) verde sob `RUN_REAL_LLM=1` com `.env` OPENROUTER_*.
- Teste determinístico de regressão do incidente (RF-13/RF-15) verde: nenhuma afirmação de cadastro sem tool call; falha sempre com erro real persistido no run e no log.
- Suíte de integração verde sob `-tags integration` (testcontainers Postgres), cobrindo RF-04, RF-12, RF-14, RF-15, RF-18.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — harness real-LLM, scorers e testes de integração no padrão do substrato agentivo.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/card_create_confirm_workflow_integration_test.go` (novo) — integração testcontainers Postgres, ciclo suspend→resume, idempotência, conflito, falha de infra, exclusão mútua, TTL.
- `internal/agents/application/workflows/card_create_harness_test.go` (novo) — harness real-LLM dos cenários Gherkin com gate estatístico ≥ 0.90.
- `internal/agents/application/workflows/card_create_regression_test.go` (novo) — regressão determinística do incidente (nenhuma afirmação sem tool call; falha sempre com erro persistido).
- `internal/agents/application/workflows/pending_entry_harness_test.go` — padrão de harness real-LLM (referência).
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — padrão de integração testcontainers (referência).
