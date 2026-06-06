# ADR-005-bis — `IdentityModule` shape pós-E2

## Metadados

- **Título:** Evolução aditiva da struct `IdentityModule` após E2 (billing-pipeline)
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Substitui (parcialmente):** [ADR-005](./adr-005-identity-module-shape-mvp.md) — apenas a seção "Decisão" sobre os campos da struct. Restante de ADR-005 (construtor com 3 parâmetros, item 6 do Padrão, regra R6 de interface no consumidor) permanece em vigor.
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-18
  - PRD do épico consumidor: `.specs/prd-billing-pipeline/prd.md`
  - ADR-005 original (shape MVP de E1)
  - Tasks: `task-8.0` (E1) + waves 5/6/7 de E2

## Contexto

ADR-005 fixou o shape MVP do `IdentityModule` com **apenas dois campos**:

```go
type IdentityModule struct {
    RepositoryFactory interfaces.RepositoryFactory
    UserRouter        *server.UserRouter
}
```

Esse contrato era suficiente para o entregável de E1 (Identity Foundation). Em E2 (billing-pipeline), as waves 5/6/7 adicionaram:

- **Use cases identity diretamente expostos** (`UpsertUserUseCase`, `FindUserByIDUseCase`, `FindUserByWhatsApp`, `MarkUserDeleted`) — billing precisa garantir upsert idempotente em handlers de webhook e em projeções de evento.
- **`EntitlementReader`** — leitura somente do estado de entitlement projetado a partir de eventos de billing; consumido por billing e por consumidores futuros sem ter que reimplementar o read-model.
- **`SubscriptionProjector`** (`*consumers.SubscriptionEventProjector`) e **`EventHandlers`** (`[]EventHandlerRegistration`) — handlers para a família de eventos `billing.subscription.*` registrados no dispatcher do worker.

A struct cresceu para **9 campos** sem ADR de evolução, criando drift entre documentação (ADR-005) e implementação (working tree). Este ADR-005-bis registra a decisão explicitamente.

## Decisão

`IdentityModule` mantém a forma e o construtor de ADR-005, mas com superfície expandida:

```go
type IdentityModule struct {
    RepositoryFactory     interfaces.RepositoryFactory
    UserRouter            *server.UserRouter
    UpsertUserUseCase     *usecases.UpsertUserByWhatsApp
    FindUserByIDUseCase   *usecases.FindUserByID
    FindUserByWhatsApp    *usecases.FindUserByWhatsApp
    MarkUserDeleted       *usecases.MarkUserDeleted
    EntitlementReader     interfaces.EntitlementReader
    SubscriptionProjector *consumers.SubscriptionEventProjector
    EventHandlers         []EventHandlerRegistration
}
```

Invariantes preservadas:

- Construtor `NewIdentityModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) IdentityModule` mantém **3 parâmetros** — sem `opts ...Option`, sem `With...` (item 6 do Padrão).
- `UserRouter` continua placeholder; bootstrap em `cmd/server` só registra se `!= nil`.
- Todos os campos novos são consumidos por billing ou pelo bootstrap (`cmd/server` para router, `cmd/worker` para EventHandlers); nenhum campo decorativo.
- Tipos de retorno expostos seguem regra R6 (interface no consumidor): use cases publicados como structs ponteiros porque o consumidor (billing) declara sua própria interface mínima quando precisa abstrair.

## Alternativas Consideradas

### A) Manter shape MVP e expor use cases via getters

- **Vantagens:** preserva a superfície original.
- **Desvantagens:**
  - Cria getters proibidos pelo item 6 do Padrão (`Routers()`/`Runners()` ilustram a regra).
  - Não escala — cada novo consumidor exigiria novo getter.
- **Motivo de não escolher:** viola o Padrão e duplica wiring.

### B) Submódulo `identity.SubscriptionsModule` separado

- **Vantagens:** isola crescimento.
- **Desvantagens:**
  - Trade-off de complexidade alto: cmd/server e cmd/worker passariam a wirear dois módulos identity-derivados.
  - Acoplamento billing → identity ficaria espalhado por dois pontos de entrada.
- **Motivo de não escolher:** não há justificativa funcional para split nesse momento.

### C) Adiar a decisão e manter drift

- **Vantagens:** zero esforço.
- **Desvantagens:**
  - ADR-005 vira documentação morta; futuras tasks confiam em shape errado.
- **Motivo de não escolher:** governança proíbe drift documental.

## Consequências

### Benefícios

- ADR-005 + ADR-005-bis juntos descrevem o caminho evolutivo do shape sem reescrita.
- Consumidores (billing) têm contrato estável e auditável.
- Crescimentos futuros (E3, E4) repetem a mesma mecânica: nova ADR-005-ter aditiva.

### Trade-offs

- 9 campos é o limite aceitável; ADR-005 já previa "revisitar se passar de 4–5 campos". Atualizamos esse umbral para 10 — acima disso, considerar subdivisão.
- `EntitlementReader` é instanciado com `mgr.DBTX(context.Background())` no construtor — leitura ad-hoc usa o pool diretamente. Decisão alinhada ao bugfix M-2 (Find não precisa de UoW).

### Riscos e Mitigações

- **Risco:** consumidores externos começarem a depender de campos hoje internos.
  - **Mitigação:** doc.go (`internal/identity/doc.go`) documenta superfície estável; campos não documentados são experimentais.
- **Risco:** `EventHandlers` crescer descontrolado.
  - **Mitigação:** revisar quando passar de 10 entradas; considerar agrupar por subdomínio.

## Plano de Implementação

Já implementado no working tree em `internal/identity/module.go` (waves 5/6/7 de E2). Este ADR só formaliza retroativamente. Validações em `internal/identity/module_test.go` (existente).

## Monitoramento e Validação

- `go build ./...` e `go vet ./...` verdes.
- `internal/identity/module_test.go` cobre construção e satisfação de contratos.
- Bootstrap em `cmd/server/server.go` (registro do `UserRouter`) e `cmd/worker/worker.go` (registro de `EventHandlers`) auditados nos respectivos diffs.

## Impacto em Documentação

- `internal/identity/doc.go` atualizado para listar todos os campos exportados.
- `task-8.0-wiring-module-doc-bootstrap.md` referencia este ADR como evolução pós-MVP.
- ADR-005 permanece como referência histórica do shape MVP.

## Revisão Futura

- Revisitar quando E3 (onboarding ou outro) adicionar campos. Se a struct passar de 10 campos, considerar split em submódulos.
- Revisitar quando billing publicar nova família de eventos exigindo handler dedicado.
