# Tarefa 8.0: Observabilidade e job de abandono

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Instrumentar o funil do onboarding e detectar abandono por etapa.

<requirements>
- `Run` auditável por execução: `thread_id`, `run_id`, `workflow=onboarding`, `step`, `status` (`RunStatus`), `duration_ms`, `error` (RF-29).
- Métricas com cardinalidade controlada (sem `user_id`/`correlation_key`/`category_id`): `onboarding_step_total{step,outcome}`, `onboarding_completed_total`, `onboarding_run_duration_seconds`, `onboarding_step_abandoned_total{step}` (RF-30, R-WF-KERNEL-001.4).
- Job periódico de abandono: varre `workflow_runs` (workflow=onboarding, status=suspended, inativo > TTL configurável) e emite `onboarding.step_abandoned{step}`; política de marcação documentada (RF-30).
- Entregue como `worker.Job` via wiring do módulo (Padrão Obrigatório de Módulo).
</requirements>

## Subtarefas

- [ ] 8.1 Métricas de funil (counters/histograms) com labels enumerados.
- [ ] 8.2 Run auditável com campos mínimos.
- [ ] 8.3 Job de abandono (scan + emissão de evento/métrica) + config de TTL.

## Detalhes de Implementação

Ver `techspec.md` → "Monitoramento e Observabilidade" (job QT-04) e regras de cardinalidade (R-TXN-004 / R-WF-KERNEL-001.4).

## Critérios de Sucesso

- Métricas expostas com labels controlados; nenhuma de alta cardinalidade.
- Job detecta sessões inativas e emite `onboarding.step_abandoned{step}`.
- Run auditável presente em toda execução de etapa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite, mocks por IIFE): job de abandono (seleciona inativos, emite evento, idempotência); registro de métricas com labels enumerados.
- [ ] Testes de integração — scan de `workflow_runs` coberto na 9.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/infrastructure/jobs/handlers/` (job de abandono)
- `internal/agent/application/workflow/` (métricas de funil)
- `internal/agent/module.go` (wiring do job)
- `configs/` (TTL de abandono)
