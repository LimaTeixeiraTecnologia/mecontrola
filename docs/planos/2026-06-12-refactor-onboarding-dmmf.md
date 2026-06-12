# Refactor advisory — `internal/onboarding` (DMMF seletivo)

Modo: **advisory** · Equivalência comportamental estrita · Skill obrigatória: `go-implementation`

## Contexto

O módulo `internal/onboarding` já tem boa fronteira (use cases finos, repositórios SQL puros, producer fino, eventos via outbox) mas concentra **decisão de estado e formatação de evento** dentro de `application/binding/subscription_binding.go` e dentro dos próprios use cases. A modelagem atual:

- `MagicToken` é entidade rica com transições idempotentes (PENDING → PAID → CONSUMED / EXPIRED), porém invariantes ficam espalhadas entre `domain/services/transitions.go` (predicados/validadores) e a própria entidade (`MarkPaid`, `MarkConsumed`, `MarkExpired`).
- `valueobjects/token.go` já é um smart constructor sólido (32 bytes + SHA256, secret redact). `token_status.go`, `activation_path.go` e `support_signal_kind.go` são enums `uint8` com `Parse*`, sem erros sentinela uniformes.
- `application/events/subscription_bound.go` define o payload do evento `onboarding.subscription_bound`, mas a **decisão** desse payload nasce dentro do binding service junto com IO (upsert user, bind subscription, MarkConsumed, persist, publish). Não há `Decide*` puro.
- `internal/transactions` já introduziu (ADR-006) padrões DMMF seletivos: smart constructor para command, `option.Option[T]`, eventos tipados em `domain/entities/events.go`, `Decide*` puro retornando `{Aggregate, Event}` consumido pelo use case.
- Consumer `SubscriptionPaidConsumer` (`infrastructure/messaging/database/consumers/subscription_paid_consumer.go:58-64`) faz check `if p.FunnelToken == ""` com skip silencioso — branching de domínio em adapter, viola R-ADAPTER-001.2 mas reflete comportamento existente que deve ser preservado.

A refatoração visa **clareza, robustez de invariantes e reuso do modelo já validado em transactions**, sem mover comportamento observável. Tudo abaixo é proposta — execução só com aprovação explícita.

## Classificação

- `task_type`: `refactor-modeling` (modelagem de domínio + reorganização de fronteiras de aplicação; sem mudança de schema, contratos HTTP, payload de evento ou cron).
- Escopo: somente `internal/onboarding/**`. Sem mudanças cross-módulo.

## Hotspots (caminhos reais)

| Área | Caminho | Observação |
|------|---------|------------|
| State machine de token | `internal/onboarding/domain/entities/magic_token.go` + `domain/services/transitions.go` | Predicados duplicam validação da entidade; oportunidade de consolidar |
| VO enums | `domain/valueobjects/token_status.go`, `activation_path.go`, `support_signal_kind.go` | Faltam sentinelas `ErrInvalid*`; parsing é tolerante demais |
| Decisão de evento | `application/binding/subscription_binding.go` | Mistura IO + decisão; payload do evento construído inline |
| Decisão de fallback | `application/usecases/try_fallback_activation.go` | `HasOutreach()` + ActivationPath escolhidos no use case (ok) |
| Decisão de expiração | `application/usecases/expire_tokens.go` | Decide emissão de `OrphanExpiredSubscription` lendo flag PAID lateral; sem `Decide*` puro |
| Adapter com branching | `infrastructure/messaging/database/consumers/subscription_paid_consumer.go:58-64` | Skip silencioso de `FunnelToken == ""`; mover para use case **sem mudar efeito observável** |
| Producer | `infrastructure/messaging/database/producers/onboarding_event_publisher.go` | Já fino — apenas marshalling/outbox; manter |

## Invariantes a reforçar (sem alterar)

1. `PENDING → PAID` é idempotente: chamadas repetidas com mesmo `subscription_id` são noop.
2. `PAID → CONSUMED` requer `userID` + `mobileE164` + `path`; reuse por outro E.164 emite `TokenReuseAttempt` (signal) sem reabrir transição.
3. `MarkExpired` é noop em CONSUMED/EXPIRED; emite `OrphanExpiredSubscription` somente quando estado anterior era PAID.
4. `MarkOutreachSent` exige PAID; outreach reset (5xx) limpa timestamp mas não muda status.
5. Token cleartext nunca pode aparecer em log, evento, métrica ou erro retornado.
6. Evento `onboarding.subscription_bound` carrega `token_hash_prefix` (8 hex) — nunca cleartext nem hash completo no payload publicado.

## Plano incremental advisory

### Passo 1 — Sentinelas e smart constructors estritos para VO enums
Arquivos: `domain/valueobjects/token_status.go`, `activation_path.go`, `support_signal_kind.go`.

- Adicionar `ErrInvalidTokenStatus`, `ErrInvalidActivationPath`, `ErrInvalidSupportSignalKind`.
- `Parse*` mantém literais aceitos; troca `errors.New(…)` por wrap `fmt.Errorf("…: %w", ErrInvalid…)`.
- Ganho: classificação por `errors.Is` em adapters; zero mudança de string serializada.

### Passo 2 — Consolidar `TransitionService` na entidade
Arquivo: `domain/services/transitions.go`.

- Mover discriminação de erro (NotYetPaid / Expired / AlreadyConsumedSame / TransitionNotAllowed) para dentro de `MagicToken.MarkConsumed` usando os sentinelas já existentes em `domain/errors.go`.
- Deprecar `TransitionService` se nenhum consumidor externo restar; manter caso contrário.
- Comportamento preservado: mesmos sentinelas, mesmas mensagens.

### Passo 3 — Tipar domain events em `domain/entities/events.go`
Arquivo novo: `domain/entities/events.go` (inspirado em `internal/transactions/domain/entities/events.go`).

- Struct `SubscriptionBound` com **exatamente os mesmos campos** que `application/events/subscription_bound.go` serializa hoje.
- `application/events/subscription_bound.go` vira mapper struct → envelope outbox (mesmo `event_type`, `aggregate_type`, payload binário idêntico).
- `OrphanExpiredEmitted` / `TokenReuseDetected` não criar agora (são `SupportSignal`, não event publicado).

### Passo 4 — `DecideBindAndConsume` puro
Arquivo novo: `domain/services/token_workflow.go`.

- `DecideConsume(token *MagicToken, userID, fromE164, path, now) (BindDecision, error)` retornando `{Token mutado, Event entities.SubscriptionBound}`.
- `application/binding/subscription_binding.go` reordena: IO (identity gateway, binder) → `DecideConsume` puro → persistir → publicar via mapper.
- Equivalência: mesma ordem de IO, mesmo payload, mesma idempotência.

### Passo 5 — Lift do branching do `SubscriptionPaidConsumer`
Arquivos: `infrastructure/messaging/database/consumers/subscription_paid_consumer.go` + `application/usecases/mark_token_paid.go`.

- Mover `if p.FunnelToken == ""` para o início do `MarkTokenPaid` (retorna nil silencioso).
- Consumer fica adapter fino: parse envelope → use case.
- Comportamento idêntico: zero side-effect com FunnelToken vazio; outbox continua marcando processado.
- Resolve R-ADAPTER-001.2.

### Passo 6 — `DecideExpire` para emissão de signal de órfão
Arquivo: `domain/services/token_workflow.go`.

- `DecideExpire(token, now) (ExpireDecision, error)` retornando `{Token, EmitOrphanSignal bool, SignalPayload []byte}`.
- `application/usecases/expire_tokens.go` itera tokens do `BulkExpire`, chama `DecideExpire`, persiste signal só quando decisão pedir.
- Contrato de `BulkExpire` (lista com flag `wasPaid`) preservado.

### Passo 7 — `Option[T]` (avaliar; default skip)

- Custo de migrar 7 entidades + repositórios + serialização SQL.
- **Recomendação advisory: não adotar agora.** Reavaliar quando houver novo campo opcional.

## Onde DMMF ajuda

- Smart constructor + sentinel (passo 1): erros classificáveis.
- `Decide*` puro (passos 4 e 6): isola decisão de IO, melhora teste unitário, reforça invariantes em ponto único.
- Domain event tipado (passo 3): elimina divergência struct/entity.

## Onde DMMF NÃO compensa

- State-as-type para `TokenStatus`: reescrita de serialização SQL/scans/índices. Ganho marginal.
- `Option[T]` agora: custo > ganho.
- Workflow pipeline / Result customizado: rejeitado por `domain-modeling.md` (governança).

## Restrições mantidas

- Sem mudança em payload outbox, schema banco, contrato HTTP, cron schedule, métricas Prometheus ou logs estruturados.
- Sem `panic`, `init()`, clock injetado (`time.Now().UTC()` inline).
- Producer continua fino — apenas marshalling.
- Zero comentários em `.go` (R-ADAPTER-001.1).

## Validações proporcionais

- `go build ./internal/onboarding/...`
- `go vet ./internal/onboarding/...`
- `go test ./internal/onboarding/...` (unit) — testes existentes devem passar sem alteração; novos testes para `DecideConsume` e `DecideExpire`.
- `go test -tags integration ./internal/onboarding/infrastructure/...` se executar passos 5–6.
- Gates R-ADAPTER-001.1 e R-ADAPTER-001.2 (grep) — vazios.
- Diff binário do payload do evento `onboarding.subscription_bound` antes/depois: **idêntico**.

## Critérios de aceitação

- [x] Cita apenas caminhos reais de `internal/onboarding/**`.
- [x] Cada uso de DMMF justificado por ganho concreto.
- [x] Preserva contratos públicos, assíncronos e comportamento observável.
- [x] Não introduz interface sem consumidor.

## Sequência sugerida em `execution`

PRs separados, nesta ordem, cada um com `go-implementation` Etapas 1–5 e gates R-ADAPTER-001:

1. Passo 1 (sentinelas VO).
2. Passo 2 (consolidação TransitionService).
3. Passo 3 (event tipado + mapper).
4. Passo 4 (`DecideConsume` + reordena binding).
5. Passo 5 (lift branching consumer).
6. Passo 6 (`DecideExpire`).

`needs_input`
