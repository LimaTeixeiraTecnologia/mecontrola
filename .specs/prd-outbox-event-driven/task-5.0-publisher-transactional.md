# Tarefa 5.0: Publisher transacional opt-in

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o `outbox.Publisher`: a API de superfície invocada pelo use case dentro de um `UnitOfWork[T].Do(ctx, fn)` ativo, recebendo a `database.DBTX` exposta pelo UoW (ADR-002). Faz `Storage.InsertEvent` + N `Storage.InsertDeliveries` na mesma `tx`, sem abrir transação própria, sem retry, sem rede. O commit é responsabilidade do caller — esse é o ponto que sustenta a garantia at-least-once.

<requirements>
- RF-01: `Publish(ctx, tx database.DBTX, evt Event) error` recebendo a `tx` exposta pelo UoW. NÃO expor `pgx.Tx` na assinatura pública.
- RF-02: rejeitar publish (retornar `ErrHandlerNotRegistered`) quando `event_type` sem handler registrado — sem inserir registros parciais.
- RF-05: coexistência com `events.Bus` (ADR-003) — não importar nem alterar nada em `internal/infrastructure/events` além do uso de tipos canônicos.
</requirements>

## Subtarefas

- [ ] 5.1 Criar `publisher.go` com interface `Publisher` (idêntica à techspec) e struct `transactionalPublisher` com construtor `NewPublisher(storage Storage, registry Registry, tracer trace.Tracer) Publisher` (tracer opcional via observability provider).
- [ ] 5.2 Implementar `Publish(ctx, tx, evt)`: (a) iniciar span `outbox.publish` (kind INTERNAL) e injetar `traceparent` em `evt.Headers` antes do insert; (b) consultar `Registry.SubscriptionsFor(evt.Type())`; (c) retornar `ErrHandlerNotRegistered` se vazio; (d) chamar `Storage.InsertEvent(ctx, tx, evt)` seguido de `Storage.InsertDeliveries(ctx, tx, evt.ID(), names)`; (e) propagar erros wrappados via `fmt.Errorf("outbox.publish: %w", err)`.
- [ ] 5.3 Garantir que nenhum caminho do código abre nova transação — usar apenas a `tx` recebida.
- [ ] 5.4 Criar `publisher_test.go` com `testify/suite` + table-driven, usando `mocks.Storage` e `mocks.Registry` gerados em 3.0/4.0.

## Detalhes de Implementação

Ver techspec.md seções **Design de Implementação → Interfaces Chave** (`Publisher`), **Modelos de Dados → Schema SQL** (tabelas alvo dos inserts) e **Abordagem de Testes → Testes Unitários → Publisher** (cenários obrigatórios listados).

## Critérios de Sucesso

- `go test ./internal/infrastructure/outbox/...` verde (sem build tag de integração).
- Cenário 1: 1 handler registrado → 1 chamada `InsertEvent` + 1 `InsertDeliveries(names=[s1])`; nenhuma transação criada internamente.
- Cenário 2: 3 handlers registrados → 1 chamada `InsertEvent` + 1 `InsertDeliveries(names=[s1, s2, s3])` (lote único, não N chamadas individuais).
- Cenário 3: `SubscriptionsFor` retorna vazio → `Publish` retorna `errors.Is(err, ErrHandlerNotRegistered)`.
- Cenário 4: `Storage.InsertEvent` falha → erro retornado com `errors.Is` para o erro original e mensagem prefixada por `outbox.publish:`.
- Cenário 5: `tx` mock recebida em `InsertEvent`/`InsertDeliveries` é exatamente a mesma instância passada para `Publish` (assert de ponteiro).
- Cenário 6: span `outbox.publish` criado e `traceparent` injetado em `evt.Headers` antes dos inserts (asserts via `tracetest.SpanRecorder`).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `publisher_test.go` com os 6 cenários acima usando `mocks.Storage` e `mocks.Registry`; suite `testify/suite` com `SetupTest` resetando mocks.
- [ ] Testes de integração: caminho integration completo é coberto pela task 8.0 (`subsystem_integration_test.go`); aqui basta unitário com mocks.

**Definition of Done**:
- [ ] Assinatura pública do `Publisher` é exatamente `Publish(ctx context.Context, tx database.DBTX, evt Event) error`; nenhum import de `pgx` no `publisher.go`.
- [ ] Span `outbox.publish` instrumentado com `event_id` e `event_type` como atributos.
- [ ] `traceparent` injetado em `Headers` antes do `InsertEvent` (verificado por cenário 6).
- [ ] Wrap de erros usa `fmt.Errorf("...: %w", err)` consistentemente (R-ERR-001).
- [ ] Sem alocação extra de slice quando há 0 ou 1 handler (verificar via inspeção, sem benchmark formal aqui — bench fica em 9.0).
- [ ] Cobertura ≥ 90% no `publisher.go`.
- [ ] `gofmt -w .` + `golangci-lint run` verde.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/infrastructure/outbox/publisher.go` (novo)
- `internal/infrastructure/outbox/publisher_test.go` (novo)
- `internal/infrastructure/outbox/storage.go` (consumido — criado em 3.0)
- `internal/infrastructure/outbox/registry.go` (consumido — criado em 4.0)
- `internal/infrastructure/outbox/event.go` (consumido — criado em 2.0)
- `internal/infrastructure/outbox/mocks/storage.go` (consumido)
- `internal/infrastructure/outbox/mocks/registry.go` (consumido)
