# Tarefa 9.0: Wiring + governança operacional (module.go, server.go, lint anti-PCI, runbook, dashboard)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cravar o módulo `card` no ciclo de vida do binário: `internal/card/module.go` com `NewCardModule(cfg, o11y, mgr) CardModule` e registro em `cmd/server/server.go`. Plus governança operacional: regra `golangci-lint forbidigo` anti-PCI escopada em `internal/card/...`, runbook completo de rollback, dashboard Grafana "Card Module", validação holística R0-R7 (init/panic/clock/iface assertion). Última etapa antes da evidência de SLO (Tarefa 10.0).

<requirements>
- `internal/card/module.go` exporta `NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule` retornando struct com `RepositoryFactory`, `CardRouter`, `CardLookup`. Sem `error` no retorno (sem IO complexo no constructor).
- `MustLoadSaoPauloOrExit(slog.Logger)` chamado APENAS aqui (NÃO em `init()` — R0). Falha de load → `slog.Error` + `os.Exit(1)`.
- Wiring em `cmd/server/server.go` após `onboardingModule`: `cardModule := card.NewCardModule(cfg, o11y, dbManager)`; se `cardModule.CardRouter != nil`, `srv.RegisterRouters(cardModule.CardRouter)`; log "card module wired" com `router_registered=true`.
- Regra `.golangci.yml`: `forbidigo` com pattern `\b(pan|cvv|cvc|track|pin)\b` e `paths: [internal/card/]`. Mensagem custom em pt-BR.
- Validação R0-R7 holística no escopo `internal/card/...` + `internal/platform/idempotency/...`:
  - 0 `init()`: `grep -rn '^func init()' internal/card/ internal/platform/idempotency/`.
  - 0 `panic` em produção: `grep -rn 'panic(' --include="*.go" --exclude="*_test.go" internal/card/ internal/platform/idempotency/`.
  - 0 `clock.Clock`.
  - 0 `var _ Interface = (*Type)(nil)`.
- `docs/runbooks/card-rollback.md` cobre: revert do registro em `server.go`, `migrate down`, nome literal das tabelas arquivadas, troubleshooting de `mecontrola.idempotency_keys`, pré-condição `X-User-ID` via gateway, dependência transitória do `InjectPrincipalFromHeader`.
- Dashboard Grafana "Card Module" em `docs/grafana/card-module.json` cobrindo:
  - Painel "Request rate by route" (`/api/v1/cards*`).
  - Painel "Latency p50/p95/p99" por route.
  - Painel "Error rate" por status class.
  - Painel "Idempotency outcomes" (replay/miss/conflict por minuto).
  - Painel "InvoiceFor duration p99".
- Taskfile (`Taskfile.yml`) atualizado com tasks: `card:lint`, `card:test`, `card:integration`, `card:audit` (executa gates R0-R7 + zero-comments + anti-PCI).
- Estender CI workflow para incluir `card:audit` como gate de PR.
- Dockerfile valida `tzdata` instalado para `America/Sao_Paulo` (R1, ADR-002).
</requirements>

## Subtarefas

- [ ] 9.1 `internal/card/module.go` — constructor + struct + chamada a `MustLoadSaoPauloOrExit`.
- [ ] 9.2 `cmd/server/server.go` — registro de `cardModule` + log.
- [ ] 9.3 `.golangci.yml` — bloco `forbidigo` escopado para `internal/card/...`.
- [ ] 9.4 `Taskfile.yml` — tasks `card:lint`, `card:test`, `card:integration`, `card:audit`.
- [ ] 9.5 `docs/runbooks/card-rollback.md` — runbook completo.
- [ ] 9.6 `docs/grafana/card-module.json` — dashboard JSON exportável.
- [ ] 9.7 Auditoria R0-R7 — script shell ou test Go que roda todos os gates e falha o CI em violação.
- [ ] 9.8 Validar Dockerfile/imagem base contém `tzdata`; adicionar `apk add tzdata` (alpine) ou equivalente se ausente.
- [ ] 9.9 Smoke test E2E manual: subir binário com `task run`, executar `curl POST /api/v1/cards` com `X-User-ID` + `Idempotency-Key`, observar 201; repetir → replay byte-idêntico; observar logs/spans no Grafana local.

## Detalhes de Implementação

Ver `.specs/prd-card-crud-mvp/techspec.md` §"Sequenciamento de Desenvolvimento" + §"Monitoramento e Observabilidade". Espelhar wiring de `internal/identity/module.go` + registro de `identity.UserRouter` em `cmd/server/server.go`.

## Critérios de Sucesso

- Binário compila e sobe (`task build`); log "card module wired" aparece com `router_registered=true`.
- `task card:audit` verde — todos os gates R0-R7 + zero comentários + anti-PCI passam.
- `golangci-lint run ./internal/card/...` com `forbidigo` ativa rejeita PR sintético que introduza coluna `card_pan`.
- Dashboard Grafana importa sem erro e exibe métricas reais após smoke test.
- Runbook valida procedimentos via inspeção manual + dry-run (`migrate down` em ambiente descartável).
- Smoke test E2E (9.9) cobre fluxo crítico end-to-end.

### Definition of Done

- [ ] Wiring funcionando; smoke test E2E (9.9) verde.
- [ ] `task card:audit` verde em CI.
- [ ] Runbook + dashboard commitados.
- [ ] Dockerfile validado para `tzdata`.
- [ ] Gates manuais de R-ADAPTER-001.1 e R-ADAPTER-001.2 verdes (comandos `grep` do `.claude/rules/go-adapters.md`).
- [ ] RF-16 (enforcement), RF-20, RF-37, RF-38, RF-49, RF-50 explicitamente apontados no PR.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `otel-grafana-dashboards` — gerar `card-module.json` cobrindo métricas, logs e spans dos endpoints `/api/v1/cards*` e do `BillingCycle.InvoiceFor` instrumentados via OpenTelemetry.
- `taskfile-production` — adicionar tasks `card:lint`, `card:test`, `card:integration`, `card:audit` ao `Taskfile.yml` e integrá-las à pipeline CI (GitHub Actions / Azure Pipelines conforme stack do repo).

## Testes da Tarefa

- [ ] Testes unitários: `module_test.go` valida que constructor retorna struct com campos populados e router não-nil.
- [ ] Testes de integração: smoke test E2E (9.9) com binário rodando + Postgres real + observabilidade local.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/card/module.go` (novo)
- `internal/card/module_test.go` (novo)
- `cmd/server/server.go` (modificar)
- `.golangci.yml` (modificar)
- `Taskfile.yml` (modificar)
- `docs/runbooks/card-rollback.md` (novo — preenchimento final iniciado em 1.0)
- `docs/grafana/card-module.json` (novo)
- `Dockerfile` (modificar se `tzdata` ausente)
- CI workflow (`.github/workflows/*.yml` ou equivalente — modificar para incluir `card:audit`)
