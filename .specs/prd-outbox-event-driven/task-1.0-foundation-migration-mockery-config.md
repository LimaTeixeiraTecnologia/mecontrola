# Tarefa 1.0: Fundação — migration 0002_outbox, mockery.yml, OutboxConfig e dep cron/v3

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estabelecer a fundação Postgres + configuração + mocks + dependência externa necessária para todo o resto da entrega. Nenhum código de domínio do Outbox depende de bibliotecas que ainda não existam no projeto, então esta tarefa cria explicitamente o schema two-table, o pin de `robfig/cron/v3@v3.0.1`, o `mockery.yml` raiz (inexistente hoje — D-16) e o grupo `OutboxConfig` em `configs.Config` com defaults e validações já fechadas (D-03/D-05).

<requirements>
- RF-26: feature flag `OUTBOX_DISPATCHER_ENABLED` + demais chaves flat SCREAMING_SNAKE com defaults D-03 lidas via novo grupo `OutboxConfig` agregado a `configs.Config` com `mapstructure:",squash"`.
- RF-28: migração `0002_outbox.up.sql` idempotente (`CREATE TABLE IF NOT EXISTS`, índices condicionais) + `0002_outbox.down.sql` que reverte a estrutura.
- D-03: defaults `TICK_INTERVAL=500ms`, `BATCH_SIZE=50`, `HANDLER_TIMEOUT=10s`, `RETRY_MAX_ATTEMPTS=15`, `RETRY_BASE_BACKOFF=2s`, `RETRY_MAX_BACKOFF=5m`, `HOUSEKEEPING_RETENTION_DAYS=90`, `HOUSEKEEPING_SCHEDULE=@daily`, `REAPER_INTERVAL=@every 1m`, `REAPER_STUCK_AFTER=5m`.
- D-04: `github.com/robfig/cron/v3@v3.0.1` pinado em `go.mod`.
- D-07: tabelas no schema `public`, sem `CREATE SCHEMA`.
- D-10: coluna `partition_key TEXT NULL` reservada sem índice.
- D-16: `mockery.yml` na raiz com declarações de `outbox.Storage` e `outbox.Registry`.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `migrations/0002_outbox.up.sql` conforme techspec (tabelas `outbox_events` e `outbox_deliveries`, índices `ix_outbox_*`, constraint `uq_outbox_deliveries_event_subscription`, schema `public`).
- [ ] 1.2 Criar `migrations/0002_outbox.down.sql` revertendo na ordem inversa (índices, depois tabelas).
- [ ] 1.3 Criar `mockery.yml` na raiz declarando `outbox.Storage` e `outbox.Registry` com `outpkg: mocks`, `filename: "{{.InterfaceName | snakecase}}.go"`, `dir: "{{.InterfaceDir}}/mocks"`, `with-expecter: true`.
- [ ] 1.4 Adicionar receita `task mocks` no `Taskfile.yml` executando `mockery --config mockery.yml`.
- [ ] 1.5 Rodar `go get github.com/robfig/cron/v3@v3.0.1` para pinar a dependência em `go.mod`/`go.sum`.
- [ ] 1.6 Adicionar `OutboxConfig` em `configs/config.go` (campos da tabela RF-26 com tags `mapstructure:"OUTBOX_*"`) e incluir em `Config` via `mapstructure:",squash"`.
- [ ] 1.7 Registrar defaults D-03 no `configLoader.load()` e validar `RetryMaxAttempts in [1..50]`, `DispatcherBatchSize in [1..500]`, `HousekeepingRetentionDays in [1..3650]`, parse-check de `HousekeepingSchedule`/`ReaperInterval` via `cron.ParseStandard`.
- [ ] 1.8 Adicionar testes em `configs/config_test.go` cobrindo defaults aplicados, override via env e falha-fast em valores inválidos.

## Detalhes de Implementação

Ver techspec.md seções **Arquitetura do Sistema → Componentes modificados**, **Modelos de Dados → Schema SQL `migrations/0002_outbox.up.sql`** e **Configuração (RF-26 / D-03 / D-05)** — copiar o SQL e a estrutura da `OutboxConfig` exatamente como descritos.

## Critérios de Sucesso

- `go build ./...` verde.
- `go test ./configs/...` verde com novos casos cobrindo defaults D-03 e falha em valores fora dos ranges.
- `go list -m github.com/robfig/cron/v3` retorna `v3.0.1`.
- `task mocks --dry-run` (ou execução real) executa sem erro de parsing do `mockery.yml`.
- Aplicação `cmd/migrate up` em ambiente local Postgres cria as 2 tabelas + 4 índices + constraint UNIQUE; `cmd/migrate down` reverte sem erro.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `taskfile-production` — adiciona/padroniza receita `task mocks` ligada a `mockery --config mockery.yml`, configuração estrutural do Taskfile.yml exigida pela tarefa.

## Testes da Tarefa

- [ ] Testes unitários: `configs/config_test.go` cobrindo defaults D-03, override via env, validação de ranges, parse-check de cron-spec.
- [ ] Testes de integração: aplicar `0002_outbox.up.sql` e `0002_outbox.down.sql` em Postgres local (ou testcontainer one-shot) verificando criação/remoção das tabelas e índices via `\d+ outbox_events`/`outbox_deliveries`.

**Definition of Done**:
- [ ] Migrations idempotentes verificadas (`up` aplicado 2× sem erro; `down` aplicado após `up` retorna ao estado inicial).
- [ ] Constraint `uq_outbox_deliveries_event_subscription` presente e impedindo duplo insert (verificar em integration test desta tarefa via `INSERT` cru pgx).
- [ ] `mockery.yml` parseável e referenciando exatamente `outbox.Storage` e `outbox.Registry` (interfaces ainda não existem — o arquivo prepara, mas a geração real só passa na 3.0/4.0).
- [ ] `OutboxConfig` carregado por `LoadConfig()` com todos os defaults D-03 e validações fechadas (revisar via teste table-driven).
- [ ] `go.mod` mostra `github.com/robfig/cron/v3 v3.0.1` direto (não indireto).
- [ ] `gofmt -w .` aplicado; `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0002_outbox.up.sql` (novo)
- `migrations/0002_outbox.down.sql` (novo)
- `mockery.yml` (novo, raiz do repo)
- `Taskfile.yml` (modificado — nova receita `mocks`)
- `configs/config.go` (modificado — `OutboxConfig` + `mapstructure:",squash"`)
- `configs/config_test.go` (modificado — casos de defaults/validação)
- `go.mod` / `go.sum` (modificado — `robfig/cron/v3@v3.0.1`)
