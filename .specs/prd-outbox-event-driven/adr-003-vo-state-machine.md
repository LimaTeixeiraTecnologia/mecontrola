# ADR-003 — Modelagem com Value Objects e State Pattern em `outbox`

## Metadados

- **Título:** Modelagem DDD-pragmática do pacote Outbox com VOs imutáveis e State Pattern explícito
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend
- **Relacionados:** PRD `prd-outbox-event-driven` v4; techspec; R-DDD-001 (`.agents/skills/agent-governance/references/ddd.md`); object-calisthenics-go (regras #3, #4, #9)

## Contexto

R-DDD-001 (severidade `hard`) exige invariantes protegidas, sem structs anêmicas, VOs autovalidáveis, State Pattern explícito e domain não conhecendo infra. Object Calisthenics regra #3 sugere encapsular primitivos que carreguem semântica de domínio. Object Calisthenics regra #9 proíbe getters/setters mecânicos.

Por outro lado, R-GOV-001 estabelece que `go-implementation` prevalece sobre `object-calisthenics-go` quando houver conflito, e `architecture.md` da go-implementation prefere "tipos concretos por padrão". Há tensão: aplicar VOs de forma cega gera over-engineering em infra de mensageria; ignorar gera struct anêmica que viola R-DDD-001 e dificulta teste.

Discovery aprovado (Alt 2) descreveu o Outbox como "fundação canônica de mensageria interna" — não é bounded context de negócio, mas precisa proteger invariantes de contrato (event_id válido, payload JSON válido, status com transições legítimas, attempts dentro do range).

## Decisão

Adotar **modelagem híbrida pragmática**:

1. **VOs imutáveis com construtor validador** para conceitos que carregam invariante:
   - `Event` (composição) — construtor `NewEvent(NewEventParams) (Event, error)` valida campos obrigatórios e `json.Valid(payload)`.
   - `SubscriptionName` — wrapper de string com regex `^[a-z][a-z0-9_-]{2,63}$`.
   - `Attempt` — wrapper de `uint8` com método `Next()` e `IsExhausted(max Attempt) bool`.
   - `BackoffPolicy` — VO com `rand.Rand` injetável (ver ADR-004).
   - `Headers` — VO `map[string]string` com `WithTrace`/`Get`/`Validate` (regra OC #4 — coleção de primeira classe com comportamento).
2. **State Pattern explícito** para `DeliveryStatus`:
   - Tipo `DeliveryStatus struct{ value string }` com singletons `StatusPending`, `StatusClaimed`, `StatusProcessed`, `StatusDeadLetter`.
   - Método `CanTransitionTo(next DeliveryStatus) bool` documenta transições legítimas.
3. **Sem aggregate `OutboxEnvelope` ou entidade `Delivery` mutável**: o ciclo de vida de uma delivery é gerenciado pelo Dispatcher contra o `Storage`; não há regra de domínio que combine `Event` + `Delivery` exigindo proteção transacional além da SQL.
4. **Strings cruas mantidas onde não carregam invariante do Outbox**:
   - `aggregate_id`, `aggregate_type` — forma definida por cada módulo (identity, finance, etc.); Outbox apenas armazena. Documentado no godoc do `Event`.
   - `event_type` — reaproveita `events.EventName` existente (já é VO com validação `<modulo>.<acao>`).
   - `event_id` — reaproveita `events.EventID` existente.

## Alternativas Consideradas

- **Bounded context completo com aggregate `OutboxEnvelope`**:
  - Aggregate root `OutboxEnvelope { event Event; deliveries []Delivery }` com métodos `MarkDeliveryProcessed`/`MarkDeliveryFailed`.
  - Vantagem: DDD purista; mais fácil testar transições sem mock de Storage.
  - Desvantagens: 100% das transições efetivas acontecem no DB via `Storage`; aggregate em memória teria que ser reconstruído após cada `ClaimReady`, custo sem ganho. Adiciona ~3 arquivos sem reduzir complexidade.
  - **Rejeitada**: over-engineering para infra de mensageria; o Storage já é a fonte de verdade.

- **Tipos planos sem VOs (struct anêmica)**:
  - `Event` como struct com campos exportados; `DeliveryStatus` como `string` constante.
  - Vantagem: menos código, idiomatismo "infra simples".
  - Desvantagens: viola R-DDD-001 frontalmente; chamador pode construir `Event{ Payload: []byte("invalid") }` e descobrir só no commit; estado inválido representável.
  - **Rejeitada**: regra `hard` violada.

- **VOs para tudo, inclusive `aggregate_id`**:
  - `AggregateID` como VO com regex universal.
  - Desvantagens: Outbox não conhece o formato de cada módulo; regex universal vira `.+` (inútil) ou bloqueia módulos legítimos.
  - **Rejeitada**: VO sem regra própria viola OC ("evitar wrappers vazios").

## Consequências

**Benefícios**:
- Invariantes do contrato Outbox protegidas no construtor — caller não consegue criar `Event` com payload inválido.
- Transições de `DeliveryStatus` ilegais detectáveis em teste (`StatusProcessed.CanTransitionTo(StatusPending)` retorna `false`).
- Testes determinísticos: `BackoffPolicy` com `rand.Rand` semeado; `Event` com `OccurredAt` injetado.
- Conformidade `hard` com R-DDD-001 sem fragmentar a arquitetura.

**Custos**:
- ~8 arquivos `*.go` adicionais com VOs e respectivos `*_test.go`.
- Boilerplate de getters de leitura em `Event` (regra OC #9 aceita "getters simples quando o contrato público exigir exposição controlada").

**Riscos / Mitigações**:
- **Excesso de tipos pequenos torna navegação cansativa**: mitigado por consolidação em arquivos por tópico (`event.go` contém `Event` + `NewEvent` + getters; um único arquivo por conceito).
- **Tentação de evoluir para aggregate**: registrada na "Revisão Futura"; mover para aggregate só se aparecer regra de domínio que combine `Event` + `Delivery` em memória.

## Plano de Implementação

Detalhes em techspec seção "Modelos de Dados". Cada VO tem suite de teste table-driven (R4) cobrindo construção válida, inválida e edge cases.

## Monitoramento e Validação

- Validação estática: review de PR garante que nenhum arquivo `*_test.go` ou `*.go` constrói `Event{}` literal fora dos construtores (R-DDD-001 "Proibido: struct literal de entidade fora de testes e factories").
- Validação dinâmica: testes de transição de `DeliveryStatus` cobrem todas as 16 combinações via table-driven.

## Revisão Futura

Reavaliar se aparecer regra de domínio do Outbox que combine `Event` + `[]Delivery` em memória exigindo proteção transacional além da SQL (ex.: deduplicação por janela temporal in-memory) — neste caso, promover para aggregate.
