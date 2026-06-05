# Run — Executar `create-technical-specification` para `identity-foundation`

> Data: 2026-06-05
> Origem do pedido: `docs/prompts/prompt-create-technical-specification-identity-foundation.md`
> Skill alvo: `.agents/skills/create-technical-specification`
> PRD consumido: `.specs/prd-identity-foundation/prd.md`

## Contexto

O usuário pediu para executar o prompt enriquecido em `docs/prompts/prompt-create-technical-specification-identity-foundation.md`. Esse prompt invoca a skill `.agents/skills/create-technical-specification` em modo mandatório/production-proof para gerar a especificação técnica de `internal/identity` a partir de `.specs/prd-identity-foundation/prd.md`.

O artefato a ser produzido é **somente o documento técnico** (techspec + ADRs). **Não há implementação Go nesta etapa.** O E1 é raiz do roadmap (bloqueia E2 `billing-pipeline` e E3 `onboarding-magic-token`) e o working tree de `internal/identity/` está vazio (scaffold), o que torna a fidelidade da spec especialmente crítica — qualquer drift compromete dois épicos a jusante.

A spec precisa amarrar:
- Padrão Obrigatório de Módulo (`NewIdentityModule(...) IdentityModule`) de `AGENTS.md`.
- Layout fixo de superfícies de I/O (`infrastructure/http/server`, `jobs/handlers`, `messaging/database/{consumers,producers}`).
- Cadeia `handler → usecase/service → repository/client`.
- DI manual com `o11y observability.Observability` e `db database.DBTX` via devkit-go v0.4.0.
- Shape de repository com `PrepareContext` + `span.RecordError` + log estruturado + fechamento de stmt.
- Handoff obrigatório para `.agents/skills/go-implementation/SKILL.md` em qualquer implementação Go subsequente.

## Estado atual relevante (working tree)

- `internal/identity/module.go`: contém apenas `package identity`.
- `internal/identity/{application,domain,infrastructure}/...`: árvore vazia (apenas pastas).
- `internal/billing/`: também scaffold vazio — não há `InvoiceModule` real para copiar; o padrão vem de `AGENTS.md` (seção "Padrão Obrigatório de Módulo").
- `cmd/server/server.go`: bootstrap real (cobra `server`), inicializa `o11y` via `otel.NewProvider`, `dbManager` via `manager.New(...)` do devkit-go v0.4.0 e `httpserver` via `chi_server`. **Não registra nenhum módulo de negócio hoje** — a chamada `srv.RegisterRouters(...)` é o ponto de extensão; o método deve existir no `chi_server` mas precisa ser confirmado/documentado como drift.
- `cmd/worker/worker.go`: bootstrap real, monta `[]worker.Job` (outbox dispatcher/reaper/housekeeping) e `worker.NewManager(cfg, jobs, nil, schedLogger)`. **O parâmetro `consumers` está `nil` hoje** — qualquer consumer/producer de identity precisa ser injetado nessa lista.
- `internal/platform/outbox/storage_postgres.go`: referência canônica para repository transacional (`database.FromContext(ctx)`, `BeginTx`, `errors.Join` de rollback, statement close explícito).
- `internal/platform/worker/{job,consumer}`: `Job`, `Consumer`, `Manager`, `job.NewAdapter`, `consumer.NewAdapter`.
- `internal/platform/{events,id,outbox}`: capacidades reutilizáveis (Dispatcher, UUIDGenerator, Publisher).
- `migrations/000001_outbox_events.{up,down}.sql`: próximo número é `000002_`.
- `.golangci.yml`: já tem regras `depguard` para `domain`, `application`, cross-module — a spec deve **estender** (adicionar regras para `identity/domain` ↔ `application/infrastructure` se ainda não cobertas).
- `go.mod`: `go 1.26.2`, `toolchain go1.26.4`, devkit-go `v0.4.0`.
- Devkit-go expõe `database.DBTX`, `database.FromContext(ctx)`, `manager.Manager`, `observability.Observability` (com `Logger()`, `Shutdown(ctx)`) e helpers `observability.String/Int/Error` para campos estruturados.

> **Drift documental relevante:** o prompt enriquecido pede `o11y.Tracer().Start(...)` como shape obrigatório. O working tree mostra apenas `o11y.Logger()` em uso real — o método `Tracer()` faz parte de `observability.Observability` do devkit v0.4.0 mas ainda não foi exercitado no repositório. A techspec deve declarar o uso de `Tracer()` como padrão alvo e registrar essa lacuna como drift verificável na implementação.

## Abordagem recomendada

Executar a skill `create-technical-specification` em PT-BR, gerando dois conjuntos de artefatos sob `.specs/prd-identity-foundation/`:

1. **`techspec.md`** — documento principal, com `<!-- spec-hash-prd: ... -->` no topo (calculado via `ai-spec hash`).
2. **ADRs separadas** em `adr-NNN-<slug>.md`, uma por decisão material.

A spec respeita rigorosamente o `assets/techspec-template.md` da skill e fecha as questões abertas (Q-01..Q-06) do PRD com justificativa explícita.

## Estrutura da techspec (resumo executivo do que será escrito)

### Seções obrigatórias (Markdown PT-BR)

1. **Cabeçalho com `spec-hash-prd`** e referência ao PRD/épico.
2. **Resumo executivo** — objetivo, escopo, confirmação de não-implementação.
3. **Leitura do estado atual** — `cmd/server/server.go`, `cmd/worker/worker.go`, `internal/identity/module.go`, drift identificado (`Tracer()` não exercitado; `srv.RegisterRouters` a confirmar; `consumers` nil no worker).
4. **Escopo incluído / fora de escopo** — replicando o PRD, sem reinterpretar.
5. **Arquitetura e fronteiras** — camadas hexagonais, R0–R7 aplicáveis, contratos cross-module (`Subscription` mínima em `identity/domain` para E2).
6. **Design por superfície** com paths canônicos:
   - HTTP inbound: `internal/identity/infrastructure/http/server/{router.go,handlers/*}`.
   - Repositories: `internal/identity/infrastructure/repositories/postgres/`.
   - Jobs: `internal/identity/infrastructure/jobs/handlers/` (vazio no MVP — declarado como slot futuro).
   - Consumers/Producers: `internal/identity/infrastructure/messaging/database/{consumers,producers}/` (vazio no MVP).
   - Application: `internal/identity/application/{dtos,interfaces,usecases}/`.
   - Domain: `internal/identity/domain/{entities,valueobjects,services}/` incluindo `User`, `WhatsAppNumber`, `Email`, `Reason`, `Subscription` (contrato mínimo), `IsEntitled`.
7. **Wiring e DI em `module.go`** — shape:
   ```go
   type IdentityModule struct {
       UserRouter *userhttp.Router  // pode ser nil no MVP de E1
   }

   func NewIdentityModule(
       db database.DBTX,
       cfg *configs.Config,
       o11y observability.Observability,
       idGen id.Generator,
   ) IdentityModule { /* repo → usecase → handler → router */ }
   ```
   Ordem de composição obrigatória: `repository/client → usecase → handler → router/job/consumer/producer`.
8. **Bootstrap e runtime** — instruções de wiring para `cmd/server/server.go` (`srv.RegisterRouters(identityModule.UserRouter.Register)` ou equivalente confirmado na implementação) e `cmd/worker/worker.go` (jobs/consumers de identity ficam vazios no MVP, mas slot existe).
9. **Observabilidade obrigatória** — uso de `o11y observability.Observability`, propagação de `ctx`, abertura de span por operação relevante (handler entry, usecase entry, repository SQL), logs estruturados sem PII (helpers de `WhatsAppNumber.Masked()`, `Email.Masked()`, `display_name` mask), `span.RecordError` em todo erro de I/O.
10. **Persistência** — shape mandatório por método de repository (snippet ilustrativo de `Insert/Upsert`):
    ```go
    ctx, span := r.o11y.Tracer().Start(ctx, "user_repository.upsert_by_whatsapp_number")
    defer span.End()

    query := `insert into users (...) values (...)
              on conflict (whatsapp_number) where deleted_at is null
              do update set ...`
    stmt, err := r.db.PrepareContext(ctx, query)
    if err != nil {
        span.RecordError(err)
        r.o11y.Logger().Error(ctx, "users.upsert.prepare_failed",
            observability.String("layer", "repository"),
            observability.String("entity", "user"),
            observability.Error(err),
        )
        return User{}, fmt.Errorf("identity: prepare upsert user: %w", err)
    }
    defer func() {
        if closeErr := stmt.Close(); closeErr != nil {
            span.RecordError(closeErr)
            r.o11y.Logger().Error(ctx, "users.upsert.stmt_close_failed",
                observability.Error(closeErr),
            )
        }
    }()
    ```
    Inclui regras de transação (`database.FromContext(ctx)` quando dentro de TX herdada), invariante CHECK `status='DELETED' ⇔ deleted_at IS NOT NULL`, índices parciais únicos.
11. **Migration `000002_identity_users.up/down.sql`** — apenas DDL descrito (sem aplicar): `users`, `user_whatsapp_history`, CHECK constraint, índices parciais únicos, `display_name TEXT NULL`.
12. **Estratégia de testes** — `go test -cover` 100% para `NewWhatsAppNumber/NewEmail/IsEntitled` (incluindo `sub == nil` e as 11 transições de RF-12); smoke E2E Postgres real cobrindo CA-04 (a-h); mocks via `mockery` (já em `go.mod`).
13. **Tratamento de erros** — `ErrUserNotFound`, `ErrWhatsAppNumberInUse` em `internal/identity/application/errors.go`; uso de `errors.Join` e `fmt.Errorf("ctx: %w", err)` por R7.
14. **Mapeamento requisito → decisão → teste** — tabela RF-01..RF-18 ↔ seção da spec ↔ caso de teste/CA.
15. **Handoff para `go-implementation`** — declaração mandatória de que toda implementação Go subsequente DEVE carregar `.agents/skills/go-implementation/SKILL.md`, executar Etapas 1-5, validar via `gofmt`/`go vet`/`go test -race`/`golangci-lint run`, respeitar R0–R7 (em especial R6.4, R6.7, R5.12, R5.26).
16. **Critérios de aceite da própria spec** — invalidação se não ancorar no working tree, se desviar para implementação, se omitir wiring, `o11y`, `database.DBTX` ou `go-implementation` no handoff.
17. **Riscos residuais e suposições** — replicar R-01..R-09 do PRD com mitigação técnica e adicionar drift do `Tracer()` e do método `RegisterRouters`.

### ADRs a produzir

- **ADR-001 — `Reason` como `type Reason string` com constantes nomeadas.** Fecha Q-06.
- **ADR-002 — `Subscription` mínima como interface em `identity/domain`.** Fecha Q-02.
- **ADR-003 — Mascaramento de PII como método nos VOs (`Masked()`).** Fecha Q-01.
- **ADR-004 — Erros tipados em `internal/identity/application/errors.go`.** Fecha Q-04.
- **ADR-005 — `NewIdentityModule` exporta `IdentityModule{UserRouter *Router}` no MVP de E1 com `UserRouter` permitindo `nil` quando não houver rotas reais.** Fecha Q-05.
- **ADR-006 — Janela de reanimação parametrizada como constante `ReanimationWindow = 30 * 24 * time.Hour` no domínio de identity.** Fecha R-06.
- **ADR-007 — Índice parcial único para `email` e `whatsapp_number` em Postgres.** Fecha Q-03.

## Arquivos que serão criados (somente artefatos de documentação)

- `.specs/prd-identity-foundation/techspec.md`
- `.specs/prd-identity-foundation/adr-001-reason-string-type.md`
- `.specs/prd-identity-foundation/adr-002-subscription-contract-interface.md`
- `.specs/prd-identity-foundation/adr-003-pii-masking-vo-methods.md`
- `.specs/prd-identity-foundation/adr-004-typed-errors-application-package.md`
- `.specs/prd-identity-foundation/adr-005-identity-module-shape-mvp.md`
- `.specs/prd-identity-foundation/adr-006-reanimation-window-constant.md`
- `.specs/prd-identity-foundation/adr-007-postgres-partial-unique-indexes.md`
- `docs/runs/2026-06-05-techspec-identity-foundation.md` (este arquivo)

## Restrições inegociáveis

- **Não implementar nenhum arquivo Go.** A saída é apenas Markdown.
- **Não criar nem editar `internal/identity/**/*.go`.**
- **Não rodar migrations.**
- **Não executar `go build`, `go test`, etc.** — a spec é teórica nesta etapa.
- **Working tree prevalece sobre docs históricos** — qualquer divergência vira seção de drift.
- **Handoff para `go-implementation` é mandatório.**

## Verificação

A spec será considerada válida se:
1. `cat .specs/prd-identity-foundation/techspec.md | head -5` mostra cabeçalho com `spec-hash-prd`.
2. Existem 7 ADRs em `.specs/prd-identity-foundation/adr-*.md` cobrindo Q-01..Q-06 + R-06 + Q-03.
3. Toda referência a path do módulo aponta para `internal/identity/...` real.
4. A spec contém seção "Handoff para `go-implementation`" com instruções R0–R7 explícitas.
5. `ai-spec hash .specs/prd-identity-foundation/prd.md` (se disponível) bate com o hash injetado no cabeçalho da techspec.
6. Nenhum arquivo `.go` foi tocado: `git status` mostra apenas Markdown novo em `.specs/prd-identity-foundation/` e em `docs/runs/`.

## Ordem de execução pós-aprovação

1. Calcular `spec-hash-prd` via `ai-spec hash .specs/prd-identity-foundation/prd.md` (fallback: `sha256sum` se a tool não estiver instalada, registrando como suposição).
2. Ler `assets/techspec-template.md` e `assets/adr-template.md` da skill.
3. Escrever as 7 ADRs primeiro (decisões fundamentam a spec).
4. Escrever `techspec.md` referenciando cada ADR pela URL relativa.
5. Reportar `done` com path final, paths das ADRs e itens em aberto.
