# ADR-003 — Eventbus in-process tipado via generics + emissão pós-`UoW.Commit`

## Metadados

- **Título:** Eventbus in-process tipado via generics Go 1.26 + atomicidade via emissão pós-commit
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-10, §D-08](./prd.md), [techspec §Eventbus](./techspec.md), [shared-patterns.md §Outbox](../../.agents/skills/agent-governance/references/shared-patterns.md)

## Contexto

A foundation precisa entregar o utilitário de mensageria in-process que os módulos `conversation` e `agent` (PRDs futuros) consumirão. O discovery recomenda canais Go in-process (sem broker externo no MVP) e prevê split server/worker via `APP_MODE`. R-DDD-001 exige que transições de estado sejam centralizadas no aggregate root — o que torna natural acumular eventos no aggregate e drená-los após commit.

Decisões a tomar: (1) tipagem da API; (2) momento de emissão (pré- vs pós-commit); (3) garantia de entrega.

## Decisão

**API tipada via generics Go 1.26**: `Publish[E Event](ctx, evt E)` e `Subscribe[E Event](handler)`. Cada evento é um tipo de domínio próprio (`type MessageReceived struct { ... }` implementa `Event`). Compilador valida assinatura no Subscribe/Publish.

**Emissão pós-`UoW.Commit`**: o `aggregate root` acumula eventos via `addEvent(...)`; o `application` lê via `aggregate.Events()` após `UoW.Do` retornar sucesso e publica no bus. Em caso de rollback, eventos NÃO são publicados.

**Garantia de entrega in-process: at-most-once dentro do processo** (eventos perdidos em crash entre commit e publish são aceitos no MVP). Outbox table fica para o PRD de `conversation` (Epic 06), quando o canal real precisar de reliable delivery.

## Alternativas Consideradas

1. **Untyped `Publish(topic string, payload any)`**.
   - Vantagens: API curta; serialização nativa para broker externo futuro.
   - Desvantagens: viola Object Calisthenics #3 (encapsular primitivos) e #6 (nomes opacos); subscriber faz `payload.(MyEvent)` runtime; desperdiça generics disponíveis em Go 1.26.
2. **Channels diretos por módulo (sem facade)**.
   - Vantagens: zero indireção.
   - Desvantagens: sem ponto comum de observabilidade; impossível adicionar tracing/metrics central; refactor obrigatório quando split server/worker chegar.
3. **Outbox table + worker dedicado no MVP**.
   - Vantagens: reliable at-least-once entre restarts.
   - Desvantagens: overhead alto (tabela + worker + dedup) para uma foundation que não emite evento real ainda; cabe melhor quando webhook chega (Epic 06).
4. **Publicação pré-commit**.
   - Vantagens: subscribers reagem mais cedo.
   - Desvantagens: subscriber pode reagir a evento que vai rollbackar; quebra atomicidade — anti-pattern para production-ready.

## Consequências

### Benefícios Esperados

- Tipagem em compile-time elimina classe inteira de bugs (Subscribe com handler errado).
- Atomicidade via pós-commit garante que evento publicado ↔ estado persistido.
- Aderente a R-DDD-001 (transição centralizada no aggregate).
- Migração futura para broker externo é interface-preserving (substitui implementação do `Bus`).

### Trade-offs e Custos

- Generics aumentam um pouco o binário (medir; cap em 30 MB).
- Subscribe genérico exige type assertion interna no roteador (custo desprezível).
- Eventos perdidos em crash entre commit e publish — aceito no MVP, registrado como dívida.

### Riscos e Mitigações

- **Risco:** subscriber lento bloqueia publisher (backpressure).
  - **Mitigação:** Buffer configurável por subscriber (default 100); quando cheio, log + drop com métrica `events_dropped_total`.
- **Risco:** publish após commit pode falhar e estado fica inconsistente.
  - **Mitigação:** publish é best-effort com retry interno (2 tentativas); falha registrada em `events_publish_failed_total`; alerta opcional pós-Epic 06.
- **Risco:** ordem de eventos perdida entre múltiplos subscribers.
  - **Mitigação:** por subscriber, ordem é preservada (fila FIFO); cross-subscriber a ordem não é garantia (e nem é esperada).

## Plano de Implementação

1. `internal/infrastructure/events/event.go`: interface `Event` + tipos base (`EventID`, `EventName`, `ModuleName`).
2. `internal/infrastructure/events/bus.go`: struct `Bus` com map[reflect.Type] → []handler; `Publish[E Event]` + `Subscribe[E Event]` + `Close`.
3. Integration test com 1000 eventos concorrentes + assert de ordem por subscriber + close idempotente.
4. README em `internal/infrastructure/events/` com exemplo de declaração de evento + handler.

## Monitoramento e Validação

- Métrica `events_published_total{event_name,outcome}` (counter).
- Métrica `events_subscriber_lag_seconds{event_name}` (gauge, placeholder enquanto sem evento real).
- Métrica `events_dropped_total{event_name,reason}` (counter).

## Impacto em Documentação e Operação

- PRDs subsequentes que emitem evento devem listar tipos em `domain/events.go` de cada módulo.
- Convenção de naming: `<modulo>.<acao>` em kebab-case (e.g. `identity.user-created`).

## Revisão Futura

- Revisitar para outbox + broker externo quando: webhook real expõe perda em crash, OU split server/worker entre processos exige delivery cross-process.
- Esperado: PRD de Conversation (Epic 06) ou hardening pós-Fase 5.
