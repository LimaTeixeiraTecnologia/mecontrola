# Tarefa 4.0: Repositórios Postgres billing

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar as 5 implementações Postgres dos repositórios declarados em 3.0 (`subscription`, `processed_event`, `kiwify_event`, `plan`, `reconciliation_checkpoint`) e o `factory.go` que os exporta de forma compatível com UoW (estilo `internal/identity/infrastructure/repositories/factory.go`). Validar idempotência via PK + tratamento `pgErrCode 23505`, garantir índice único parcial bloqueando 2ª sub ativa e suportar atualização de `last_event_at` em todas as transições.

<requirements>
- Implementações em `internal/billing/infrastructure/repositories/postgres/`.
- Reutilizar `internal/platform` (`uow`, conexão via `devkit-go/pkg/database/manager`), sem instanciar `sql.DB` próprio.
- Conflito de PK `event_key` em `processed_events` mapeado para sentinel de "já aplicado" definido em 3.0 — caller decide fluxo idempotente.
- Conflito de índice único parcial em `subscriptions(user_id)` mapeado para sentinel `ErrConcurrentActiveSub`.
- Não introduzir cache, métricas ou logs de IO próprios — observabilidade vive na camada de use case e platform.
</requirements>

## Subtarefas

- [ ] 4.1 `subscription_repository.go`: SELECT/UPSERT por `kiwify_order_id`; UPDATE de `status`/`period_end`/`grace_end`/`last_event_at`; lookup por `user_id`. Mapeia conflito 23505 do índice único parcial.
- [ ] 4.2 `processed_event_repository.go`: INSERT com PK `event_key`; conflito → retorna sentinel "já aplicado"; método `MarkSuperseded(eventKey)` para ordering (RF-12).
- [ ] 4.3 `kiwify_event_repository.go`: INSERT em `billing_kiwify_events`; UPDATE de `processed_at`.
- [ ] 4.4 `plan_repository.go`: SELECT por `kiwify_product_id` e por `code`.
- [ ] 4.5 `reconciliation_checkpoint_repository.go`: SELECT FOR UPDATE e UPSERT.
- [ ] 4.6 `factory.go`: estrutura compatível com `uow.New[...]` e seguindo o padrão de `internal/identity/infrastructure/repositories/factory.go`.
- [ ] 4.7 Integration tests por repositório usando `testcontainers-go` + Postgres real (build tag `//go:build integration`). Mockar Postgres é proibido.
- [ ] 4.8 Teste integ específico cobrindo RF-17 (2ª sub ativa concorrente → `ErrConcurrentActiveSub`) e RF-11 (replay 3× do mesmo `event_key` → 1 INSERT + 2 sentinels).

## Detalhes de Implementação

- Padrão de wiring com UoW: comparar com `internal/identity/infrastructure/repositories/factory.go` e `factory_test.go`.
- Erros sentinel exportados pelos repositórios devem ser detectáveis via `errors.Is` no use case (R5.10).
- Wrap de erro: `fmt.Errorf("billing/postgres: <op>: %w", err)`.
- `processed_events.status` pode ser atualizado para `'superseded'` quando o use case detectar evento staled — método dedicado, sem expor SQL ao caller.
- Conexão e transação obtidas via `manager.Manager` (mesmo que identity).
- Schema de referência: techspec §6.7.

## Critérios de Sucesso

- `go build ./internal/billing/infrastructure/repositories/...` verde.
- `go test -race -count=1 -tags=integration ./internal/billing/infrastructure/repositories/...` verde com testcontainers.
- 2ª sub ativa para mesmo `user_id` falha com `ErrConcurrentActiveSub` (RF-17).
- Replay 3× do mesmo `event_key` resulta em exatamente 1 INSERT + 2 sentinels (RF-11).
- `last_event_at` persistido e retornado em SELECT (precondição de RF-12).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration tests com Postgres real (testcontainers) por repositório.
- [ ] Teste específico RF-17: 2ª sub ativa concorrente falha.
- [ ] Teste específico RF-11: replay 3× → 1 INSERT + 2 sentinels.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/billing/infrastructure/repositories/postgres/{subscription,processed_event,kiwify_event,plan,reconciliation_checkpoint}_repository.go` + `*_integration_test.go`
- `internal/billing/infrastructure/repositories/factory.go` + `factory_test.go`
- Referência: `internal/identity/infrastructure/repositories/postgres/user_repository.go`, `factory.go`.
- Referência: techspec §6.7, §7.1, §7.3.
