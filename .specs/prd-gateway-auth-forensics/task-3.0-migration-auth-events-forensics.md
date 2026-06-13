# Tarefa 3.0: Migration 000015 auth_events forensics + entity + repo mapping

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adiciona colunas `request_id TEXT NULL` e `client_ip INET NULL` em `mecontrola.auth_events`, junto com 4 valores novos no CHECK de `reason` (`gateway_missing_header`, `gateway_invalid_timestamp`, `gateway_stale_timestamp`, `gateway_invalid_signature`). Atualiza entity `AuthEvent` para carregar os novos campos e o repositório para mapear.

<requirements>
- RF-14: colunas `request_id TEXT NULL`, `client_ip INET NULL` em `auth_events`
- RF-15: migration golang-migrate com `up` e `down`, ambos preservando dados (`down` apenas `DROP COLUMN`)
- ADR-008 referenciado para sanitização XFF (impl em tarefa 4.0)
- Migration numerada `000015` (sequencial à última `000014_create_transactions_baseline`)
- CHECK atualizado preservando valores legados do `prd-auth-foundation` (`invalid_signature` etc.)
- Index parcial `auth_events_request_id_idx WHERE request_id IS NOT NULL`
- Zero comentários em `.go` de produção (R-ADAPTER-001.1)
</requirements>

## Subtarefas

- [ ] 3.1 Criar `migrations/000015_auth_events_forensics.up.sql` adicionando colunas + ajustando CHECK constraint de `reason` para incluir os 4 valores novos preservando legados.
- [ ] 3.2 Criar `migrations/000015_auth_events_forensics.down.sql` revertendo CHECK + `DROP COLUMN client_ip` + `DROP COLUMN request_id` + `DROP INDEX` (ordem reversa).
- [ ] 3.3 Atualizar `internal/identity/domain/entities/auth_event.go` adicionando campos `requestID string` e `clientIP string` (representação simples; conversão `INET` ocorre no repo). Adicionar `RequestID()` e `ClientIP()` getters. Atualizar `NewAuthEvent`/`HydrateAuthEvent` para aceitar os novos campos.
- [ ] 3.4 Atualizar repositório que insere/lê `auth_events` (em `internal/identity/infrastructure/repositories/postgres/`) para popular as novas colunas. Manter compatibilidade com inserts antigos via NULL.
- [ ] 3.5 Atualizar testes unitários da entity para cobrir novos getters.
- [ ] 3.6 Rodar migration `up` + `down` + `up` em ambiente local Postgres para confirmar idempotência.

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados > Tabela auth_events — alterações". CHECK enum deve incluir TODOS os valores legados do `prd-auth-foundation` (não enumerar aqui; ler migration `000001` do auth-foundation).

Tarefa 4.0 implementa o extractor que popula esses campos; tarefa 5.0 publica via outbox.

## Critérios de Sucesso

- `task migrate:up` aplica `000015` sem erro; `task migrate:down` reverte limpo.
- `psql` mostra colunas + CHECK + index com nomes esperados.
- `go test ./internal/identity/domain/entities/... -run "AuthEvent" -v` PASS.
- `migrations_integration_test.go` (suite existente) continua verde.
- `task lint:user-isolation` PASS (não regrediu — migration não muda queries existentes).
- Diff em `go.mod`: zero.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário entity AuthEvent com novos campos
- [ ] Teste de integração de migration (`up → down → up` em Postgres local)
- [ ] Inspeção manual: `\d mecontrola.auth_events` no `psql` confirma colunas + CHECK + index

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000015_auth_events_forensics.up.sql` (novo)
- `migrations/000015_auth_events_forensics.down.sql` (novo)
- `internal/identity/domain/entities/auth_event.go` (modificado)
- `internal/identity/domain/entities/auth_event_test.go` (modificado)
- `internal/identity/infrastructure/repositories/postgres/auth_event_repository.go` (modificado — caminho real a confirmar durante execução)
