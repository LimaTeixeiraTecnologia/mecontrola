# Tarefa 9.0: Handlers HTTP + router + cronjobs operacionais + retention purge

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Expor os 9 endpoints sob `/api/v1/budgets` (RT-20) com `RequireUser` de `internal/identity` (RF-71); implementar `GetMonthlySummary` (agregação on-demand, RT-29); entregar os dois cronjobs operacionais — `abandoned_draft_reaper` (RF-18b cron 03:00 BR) e `retention_purge` (RF-66 mensal). Handlers são finos (R-ADAPTER-001.2): decodificam input, chamam use case, traduzem erro/saida; sem regra de negócio, sem SQL.

<requirements>
- Todos os endpoints sob prefixo canônico `/api/v1/budgets` (visto em `internal/identity` e `internal/categories`).
- Todos com `RequireUser` (RF-71); `user_id` derivado de `auth.FromContext(ctx)`; payload com `user_id`/`source`/`version` na criação é 400 (RF-25c/d).
- Status codes alinhados com a tabela de endpoints da techspec.
- `GetMonthlySummary` retorna 404 quando não há orçamento — exceto para `auto_draft`, que retorna 200 com planejado/percentual como `null` (RF-14/RF-15).
- Resposta inclui `version` corrente em mutações (RF-29).
- Cronjob `abandoned_draft_reaper`: schedule `"0 3 * * *"` em America/Sao_Paulo; lista rascunhos cuja `competence` < competência corrente BR; emite métrica + log estruturado; idempotente (RF-18c — flag persistente em `auto_draft.signal_emitted_at` ou tabela auxiliar `budgets_abandoned_draft_signals` com `(budget_id)` UNIQUE).
- Cronjob `retention_purge`: schedule `"0 4 1 * *"` (1º dia do mês 04:00); apaga em lotes (`LIMIT 500`) `budgets_expenses WHERE deleted_at < now() - interval '24 months'`, `budgets_alerts WHERE created_at < now() - interval '24 months'`, etc. Bloqueia expurgo de dado financeiro com evento pendente não terminal (RF-67a); registra adiamento como métrica (RF-67b).
- Zero comentários em `.go` de produção.
</requirements>

## Subtarefas

- [ ] 9.1 `application/usecases/get_monthly_summary.go` + unit test (cobre auto_draft, raízes sem despesa retornam zero, estouro > 100%).
- [ ] 9.2 `infrastructure/http/server/router.go` implementando `Register(chi.Router)` com `RequireUser` global e as 9 rotas.
- [ ] 9.3 8 handlers em `infrastructure/http/server/handlers/` (1 arquivo por endpoint).
- [ ] 9.4 Testes de handlers com `httptest` cobrindo: status codes, payload inválido, isolamento por `user_id`.
- [ ] 9.5 `infrastructure/jobs/handlers/abandoned_draft_reaper.go` + integration test (idempotência entre execuções).
- [ ] 9.6 `infrastructure/jobs/handlers/retention_purge.go` + integration test (não apaga dado com pendente; respeita lote).
- [ ] 9.7 Decidir entre tabela auxiliar de sinal (`budgets_abandoned_draft_signals`) vs coluna em `budgets`. Recomendação: tabela auxiliar para evitar alterar `budgets` com flag operacional. Refletir na migration 1.0 OU em migration incremental 000010.

## Detalhes de Implementação

Ver seção **Endpoints de API**, **Fluxo de Dados** e ADR-004 (índice parcial usado por `GetMonthlySummary`) na `techspec.md`. Padrão de handler espelha `internal/categories/infrastructure/http/server/handlers/`. Padrão de cronjob espelha `internal/billing/infrastructure/jobs/handlers/kiwify_events_housekeeping_job.go` e `internal/identity/infrastructure/jobs/handlers/auth_events_housekeeping_job.go`.

Erros tipados → status codes:
- `ErrConflict` → 409
- `ErrNotFound` → 404
- `ErrValidation` → 400/422 conforme campo
- `ErrForbidden` (cross-user) → 403 (RF-71b)
- `ErrCategoriesUnavailable` → 503 (degrade RT-18)

## Critérios de Sucesso

- Smoke: `task run:server` + `curl` em cada endpoint com JWT válido retorna o esperado.
- Integration test do `GetMonthlySummary` confirma p95 < 50ms para 100 despesas/mês (folga RT-07).
- Cronjob de rascunhos abandonados não emite métrica duplicada em 2 execuções consecutivas.
- Cronjob de retention não apaga dado dentro da retenção; respeita pendentes.
- `golangci-lint run ./internal/budgets/infrastructure/http/... ./internal/budgets/infrastructure/jobs/...` limpo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postman-collection-generator` — gerar coleção Postman para os 9 endpoints novos a partir do código de handler / `openapi.yaml`, para validação manual e onboarding de QA.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/budgets/application/usecases/get_monthly_summary.go` (novo)
- `internal/budgets/infrastructure/http/server/router.go` (novo)
- `internal/budgets/infrastructure/http/server/handlers/*.go` (novo, 8 arquivos)
- `internal/budgets/infrastructure/jobs/handlers/{abandoned_draft_reaper,retention_purge}.go` (novo)
- `migrations/000010_create_budgets_abandoned_draft_signals.{up,down}.sql` (novo, opcional conforme 9.7)
- Referência: `internal/categories/infrastructure/http/server/`, `internal/billing/infrastructure/jobs/handlers/kiwify_events_housekeeping_job.go`
