# Relatorio de Bugfix — Review Drift E1 Identity Foundation

- Total de bugs no escopo: 9 (6 achados iniciais + 3 follow-ups)
- Corrigidos: 8
- Skipped (false positive): 1
- Testes de regressao adicionados: 13 (3 errors_test + 3 pii_log_test + 7 outbox_integration_test)
- Pendentes (fora do escopo declarado): scheduler flaky, cmd/worker function-length — ambos pre-existentes baseline
- Estado final: done

## Bugs

### BUG-1
- ID: REVIEW-DRIFT-1
- Severidade: major
- Origem: finding de review (Task 1.0 + ADR-007 + R-T1)
- Estado: fixed
- Causa raiz: Migration `000002_create_identity_users.up.sql` foi escrita com nomenclatura Postgres-default (`users_pkey`, `users_status_check`, `users_status_deleted_at_check`, `users_whatsapp_number_active_uniq_idx`, `users_email_active_uniq_idx`) e `000003_create_identity_user_whatsapp_history.up.sql` com `user_whatsapp_history_pkey`, `user_whatsapp_history_user_id_fkey`, `user_whatsapp_history_active_unlinked_at_check`. ADR-007 e `task-1.0` continuavam afirmando nomes sem `_idx`/`_pkey`/`_fkey`. O repo Postgres ja havia sido corrigido pelo bugfix C-2 para casar com os nomes reais, mas a documentacao nao acompanhou — risco de regressao silenciosa em renomeacoes futuras orientadas pela spec.
- Arquivos alterados:
  - `.specs/prd-identity-foundation/adr-007-postgres-partial-unique-indexes.md` (CREATE TABLE/INDEX no bloco SQL + secao "Historico" com data 2026-06-06)
  - `.specs/prd-identity-foundation/task-1.0-migrations-postgres-invariantes.md` (requirements + "Constraints obrigatorias" + criterios de sucesso e testes da tarefa)
- Teste de regressao: N/A — alteracao puramente documental. Validacao indireta: `user_repository_integration_test.go::TestUniqueConstraintMapping_Email_ViaRepo` e `::TestUniqueConstraintMapping_WhatsApp_ViaReanimate` ja exercitam os nomes reais (`*_uniq_idx`) via `pgErr.ConstraintName` switch em `user_repository.go:76-79,262-265`.
- Validacao: `go build ./...` OK; `golangci-lint run ./...` 0 issues; nenhum codigo alterado. Os nomes documentados agora casam 1:1 com `migrations/000002` e `000003`.

### BUG-2
- ID: REVIEW-DRIFT-2
- Severidade: major
- Origem: finding de review (Task 8.0 + ADR-005)
- Estado: fixed
- Causa raiz: ADR-005 e `task-8.0` declararam shape MVP do `IdentityModule` com 2 campos (`RepositoryFactory`, `UserRouter`). Waves 5/6/7 de E2 (`prd-billing-pipeline`) adicionaram 7 campos sem ADR de evolucao: `UpsertUserUseCase`, `FindUserByIDUseCase`, `FindUserByWhatsApp`, `MarkUserDeleted`, `EntitlementReader`, `SubscriptionProjector`, `EventHandlers`. O codigo e funcional e necessario ao E2, mas a documentacao virou drift.
- Arquivos alterados:
  - `.specs/prd-identity-foundation/adr-005-bis-identity-module-shape-post-e2.md` (novo ADR aditivo registrando shape pos-E2, invariantes preservadas, alternativas consideradas)
  - `.specs/prd-identity-foundation/task-8.0-wiring-module-doc-bootstrap.md` (requirements + secao "Detalhes de Implementacao" referenciando ADR-005-bis)
- Teste de regressao: N/A — alteracao documental. Codigo em `internal/identity/module.go` e validado por `internal/identity/module_test.go` (preexistente).
- Validacao: `go build ./...` OK; `go vet ./...` OK; `go test -race -count=1 ./internal/identity/...` PASS.

### BUG-3
- ID: REVIEW-DRIFT-3
- Severidade: minor
- Origem: finding de review (Task 8.0 / RF-17)
- Estado: fixed
- Causa raiz: `internal/identity/doc.go` continha apenas `package identity` — placeholder nunca preenchido. RF-17 exigia comentario de pacote enumerando portas exportadas sem mencionar `JWT|RBAC|role|is_admin|session`.
- Arquivos alterados:
  - `internal/identity/doc.go` (comentario de pacote enumerando `NewIdentityModule`, `RepositoryFactory`, `UserRouter`, `UpsertUserUseCase`, `FindUserByIDUseCase`, `FindUserByWhatsApp`, `MarkUserDeleted`, `EntitlementReader`, `SubscriptionProjector`, `EventHandlers`, sentinels do ADR-004 e garantias de dominio)
- Teste de regressao: validacao por `grep` (gate de CA-03/RF-17): `grep -E "JWT|RBAC|\brole\b|is_admin|session" internal/identity/doc.go` retorna 0.
- Validacao: `golangci-lint run ./internal/identity/...` 0 issues; `grep` proibido retorna 0; build verde.

### BUG-4
- ID: REVIEW-DRIFT-4
- Severidade: minor
- Origem: finding de review (Task 4.0 / ADR-004)
- Estado: fixed
- Causa raiz: `ErrEntitlementNotFound` foi declarado em `internal/identity/application/interfaces/entitlement_repository.go`, divergindo da centralizacao de sentinels em `internal/identity/application/errors.go` exigida por ADR-004. Adicionalmente, `internal/identity/application/usecases/decide_user_entitlement.go:17` continha um `ErrEntitlementNotFound` duplicado e nao-utilizado (sombra do sentinel do port — codigo morto), o que abria espaco para confusao futura.
- Arquivos alterados:
  - `internal/identity/application/errors.go` (adicionado `ErrEntitlementNotFound = errors.New("identity: entitlement not found")` — mesma mensagem do sentinel original do port; tipo e semantica preservados para `errors.Is`)
  - `internal/identity/application/interfaces/entitlement_repository.go` (removida declaracao do sentinel; import `"errors"` removido junto)
  - `internal/identity/application/usecases/decide_user_entitlement.go` (removido `var ErrEntitlementNotFound` duplicado e nao-utilizado; import de `application` adicionado; `interfaces.ErrEntitlementNotFound` -> `application.ErrEntitlementNotFound` no `errors.Is` da linha 50)
  - `internal/identity/application/usecases/decide_user_entitlement_test.go` (import de `application` adicionado; `interfaces.ErrEntitlementNotFound` -> `application.ErrEntitlementNotFound` no mock `Return`)
  - `internal/identity/infrastructure/repositories/postgres/entitlement_repository.go` (import de `application` adicionado; `interfaces.ErrEntitlementNotFound` -> `application.ErrEntitlementNotFound` na linha 78)
- Teste de regressao: criado `internal/identity/application/errors_test.go` com 3 testes:
  - `TestSentinelsAreDistinct` — garante que `errors.Is` distingue cada sentinel dos outros 3 (matriz NxN).
  - `TestErrEntitlementNotFoundIsMatched` — exercita `errors.Is(wrap, application.ErrEntitlementNotFound)` apos `fmt.Errorf("%w")` simulando o wrapping do repo Postgres.
  - `TestErrUserNotFoundIsMatched` — sanity check com o sentinel pre-existente para garantir que a movimentacao nao quebrou o padrao geral.
  - `decide_user_entitlement_test.go::TestNoSubscriptionNotEntitled` ja exercitava o fluxo end-to-end via mock; continua verde apos a movimentacao, confirmando que o UC reconhece o novo path do sentinel.
- Validacao: `go test -race -count=1 -v -run "^(TestSentinelsAreDistinct|TestErrEntitlementNotFoundIsMatched|TestErrUserNotFoundIsMatched)$" ./internal/identity/application/` retornou 3/3 PASS; `TestDecideUserEntitlement` (7 subtestes) verde; `go build ./...` OK; `go vet ./...` OK; `golangci-lint run ./...` 0 issues.

### BUG-5
- ID: REVIEW-DRIFT-5
- Severidade: minor
- Origem: finding de review (artefato espurio em git status)
- Estado: skipped (false positive)
- Causa raiz: o `git status` foi truncado a 2k caracteres pelo harness, exibindo `internal/identity/infrastructure/repositories/postgres/e` como if fosse arquivo orfao. Inspecao direta do diretorio (`ls -la internal/identity/infrastructure/repositories/postgres/`) mostrou que o arquivo real e `entitlement_repository_integration_test.go` (E2 untracked legitimo). Nao existe arquivo `e`.
- Arquivos alterados: nenhum.
- Teste de regressao: N/A — false positive; sem mudanca.
- Validacao: `ls -la internal/identity/infrastructure/repositories/postgres/` confirma que nao existe arquivo `e`; o untracked legitimo (`entitlement_repository_integration_test.go`) e wireado pelo E2 e nao deve ser removido.

### BUG-6
- ID: REVIEW-DRIFT-6
- Severidade: minor
- Origem: finding de review (Task 7.0 vs feedback memory R6.4)
- Estado: fixed
- Causa raiz: `task-7.0` exigia `var _ chi_server.Router = (*UserRouter)(nil)` como teste de compilacao. A regra R6.4 — documentada em `.claude/projects/.../memory/feedback_no_interface_assertion.md` — proibe esse padrao. O conflito era de spec (a regra foi codificada apos a redacao do AC).
- Arquivos alterados:
  - `.specs/prd-identity-foundation/task-7.0-http-router-handler-responses.md` (criterio de sucesso reescrito: satisfacao da interface fica garantida implicitamente pela assinatura de `srv.RegisterRouters(NewUserRouter(handler))` compilar; referencia explicita a R6.4)
- Teste de regressao: N/A — alteracao de spec. Compatibilidade ja era validada implicitamente por `router_test.go` (`srv.RegisterRouters` aceita `*UserRouter`).
- Validacao: `go build ./...` OK; `go test -race -count=1 ./internal/identity/infrastructure/http/...` PASS; nenhum `var _ Iface = (*T)(nil)` introduzido.

## Comandos Executados

```
$ go build ./...
exit=0 (sem output)

$ go vet ./...
exit=0 (sem output)

$ go test -race -count=1 ./internal/identity/...
ok  internal/identity/application                           1.415s
ok  internal/identity/application/usecases                  1.514s
ok  internal/identity/domain                                3.157s
ok  internal/identity/domain/entities                       1.834s
ok  internal/identity/domain/pii                            2.830s
ok  internal/identity/domain/valueobjects                   2.149s
ok  internal/identity/infrastructure/http/server            3.484s
ok  internal/identity/infrastructure/http/server/handlers   2.511s
ok  internal/identity/infrastructure/messaging/database/consumers   3.243s
ok  internal/identity/infrastructure/repositories           2.722s
ok  internal/identity/infrastructure/repositories/postgres  2.729s
exit=0 (todos verdes)

$ go test -race -count=1 -v -run "^(TestSentinelsAreDistinct|TestErrEntitlementNotFoundIsMatched|TestErrUserNotFoundIsMatched)$" ./internal/identity/application/
=== RUN   TestSentinelsAreDistinct
--- PASS: TestSentinelsAreDistinct (0.00s)
=== RUN   TestErrEntitlementNotFoundIsMatched
--- PASS: TestErrEntitlementNotFoundIsMatched (0.00s)
=== RUN   TestErrUserNotFoundIsMatched
--- PASS: TestErrUserNotFoundIsMatched (0.00s)
PASS
ok  github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application  1.219s

$ go test -race -count=1 -v -run "^TestDecideUserEntitlement$" ./internal/identity/application/usecases/
=== RUN   TestDecideUserEntitlement
    --- PASS: TestDecideUserEntitlement/TestNoSubscriptionNotEntitled  (0.00s)
    --- PASS: TestDecideUserEntitlement/TestActiveEntitled              (0.00s)
    ... (7/7 subtestes PASS)
PASS

$ golangci-lint run ./internal/identity/...
0 issues.

$ golangci-lint run ./...
0 issues.

$ grep -E "JWT|RBAC|\brole\b|is_admin|session" internal/identity/doc.go
exit=1 (nenhum match — RF-17 satisfeito)

$ grep -RIn "interfaces\.ErrEntitlementNotFound" --include="*.go"
exit=1 (nenhum match — movimentacao completa)
```

## Follow-ups Fechados (segunda passada)

### BUG-7
- ID: REVIEW-FOLLOWUP-RF14
- Severidade: minor
- Origem: follow-up R-DRIFT-1 do review (RF-14 evidencia positiva)
- Estado: fixed
- Causa raiz: `upsert_user_by_whatsapp_handler.go` cumpria RF-14 apenas por ausencia de vazamento — nenhum log positivo emitia identificador mascarado via `WhatsAppNumber.Masked()`. Sem teste comprovando que raw PII nao chega ao logger, a garantia era frouxa.
- Arquivos alterados:
  - `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go` (mantida referencia `whatsapp` ao VO apos parse; log positivo `identity.handler.upsert_succeeded` com `observability.String("whatsapp_masked", whatsapp.Masked())` adicionado antes do `responses.JSON`; log de erro 500 enriquecido com mesmo `whatsapp_masked`)
  - `internal/identity/infrastructure/http/server/handlers/upsert_pii_log_test.go` (novo arquivo com 3 testes de regressao usando `recordingLogger`/`recordingObservability` que satisfazem `observability.Logger`/`Observability` e capturam todos os `Field`s: `TestUpsertHandler_SuccessLogsMaskedWhatsApp`, `TestUpsertHandler_ErrorLogsMaskedWhatsApp`, `TestUpsertHandler_ConflictDoesNotLeakWhatsApp`)
- Teste de regressao: 3 testes garantem (a) presenca do field `whatsapp_masked` com fragmento `9****-4321`; (b) valor mascarado != raw; (c) raw `+5511987654321` jamais aparece como `StringValue()` em qualquer log capturado nos paths sucesso/erro/conflict.
- Validacao: `go test -race -count=1 -v -run "^TestUpsertHandler_" ./internal/identity/infrastructure/http/server/handlers/` retornou 3/3 PASS + suite preexistente intacta (`TestUpsertHandler` 7/7).

### BUG-8
- ID: REVIEW-FOLLOWUP-OUTBOX-INT
- Severidade: minor
- Origem: follow-up R-DRIFT-2 do review (ausencia de smoke integration outbox)
- Estado: fixed
- Causa raiz: Task 10.0 entregou refactor para UoW + Factory com cobertura via unit tests + mocks. Faltava smoke integration validando idempotencia por `event_id` (clausula `ON CONFLICT (id) DO NOTHING` em `storage_postgres.go:38`) e o ciclo completo `Insert → ClaimBatch → MarkPublished/MarkFailed → ResetStuck → DeletePublishedBatch` contra Postgres real.
- Arquivos alterados:
  - `internal/platform/outbox/storage_postgres_integration_test.go` (novo arquivo `//go:build integration`; reaproveita pattern testcontainers + golang-migrate; 7 testes cobrindo: idempotencia tripla por id, transicao para Published, transicao para Failed com `last_error`, MarkPendingRetry incrementando `attempts`, ClaimBatch + `FOR UPDATE SKIP LOCKED` previne dupla claim, DeletePublishedBatch respeita `retention`, ResetStuck volta processing → pending)
- Teste de regressao: 7 testes greenfield em `TestOutboxStorage_*`. Cobertura E2E real, nao depende de mock.
- Race condition descoberta e corrigida durante validacao: `time.Now().UTC()` no `OccurredAt` exposto a skew entre clock Go e Postgres `now()` (terceira insercao caia ~µs no futuro do `now()` da TX de SELECT; ClaimBatch retornava 2/3). Fix: ancorar `OccurredAt` em `time.Now().UTC().Add(-time.Minute)` — comportamento de producao nao afetado (producers usam timestamp real do evento, nao colidem com `now()` do broker). Causa raiz registrada para evitar repeticao em outros testes baseados em `now()`.
- Validacao: `go test -tags=integration -race -count=1 -timeout=10m ./internal/platform/outbox/` retornou 7/7 PASS em 12.24s contra `postgres:16` via testcontainers.

### BUG-9
- ID: REVIEW-FOLLOWUP-MOCKERY-V3
- Severidade: minor
- Origem: follow-up R-DRIFT-3 do review (R-4 do bugfix original — mockery v3 ≠ config v2)
- Estado: fixed
- Causa raiz: `mockery.yml` estava em sintaxe v2 (`with-expecter`, `mockname`, `outpkg`, `filename`); mockery instalado e v3.7.0; comando `mockery` falhava com `discovering mockery config: file not found` (sintaxe v2 nao e detectada como config v3). Alem disso, a lista de interfaces estava desatualizada: incluia `identity.IDGenerator` (interface morta — removida quando dominio adotou `entities.NewID()` autossuficiente per ADR-008) e omitia o agrupamento por package.
- Arquivos alterados:
  - `mockery.yml` (reescrito em sintaxe v3 — chaves `dir`, `structname`, `pkgname`, `template: testify`, `template-data.unroll-variadic`; removida interface morta `identity.IDGenerator`; mantidos os 10 interfaces ativos: billing 6, identity 1, outbox 2; comentario topo explica que UoW/Manager/Factory continuam manuais por incompatibilidade com generics)
- Teste de regressao: validacao por `mockery showconfig --config mockery.yml` que carrega sem warning/erro (saida termina em `template-schema: ...` — config aceita) e por `go build ./...` continuar verde (mocks manuais preservados intactos).
- Decisao consciente: **nao regenerar mocks** nesta passada — mocks de `UnitOfWorkUser`/`UnitOfWorkVoid` usam tipos genericos (`uow.UnitOfWork[T]`) com semantica de teste (`fn(ctx, nil)` inline) que mockery v3 nao reproduz fielmente; regenerar quebraria 5+ test suites. R-4 fica fechada (config valida em v3); regeneracao incremental fica como follow-up futuro quando alguem precisar de novo mock listado.
- Validacao: `mockery showconfig` exit 0 sem WRN/ERR; `go build ./...` OK; `go vet ./...` OK; suites de identity e billing rodando todos os mocks manuais sem mudanca de comportamento.

## Riscos Residuais

- **R-DRIFT-4 (pre-existente, fora de escopo E1):** `internal/platform/worker/job/scheduler_test.go::TestOverlapAllow_SemGoroutineLeak` flaky em HEAD baseline; `cmd/worker/worker.go` revive `function-length` warning preexistente. Ambos fora do escopo de bugfix de identity.
- **R-DRIFT-5 (rastreabilidade documental):** ADR-005 original continua descrevendo o shape MVP de 2 campos para preservar historico; quem ler so o ADR-005 sem ADR-005-bis pode achar que a struct esta divergente. Mitigado pelo cross-link em ambos os documentos.
- **R-DRIFT-6 (mockery regeneracao):** futura execucao de `mockery generate` sobrescreve mocks de `UnitOfWorkUser`/`UnitOfWorkVoid`/`RepositoryFactory` — manter regeneracao gated em PR review ate consolidar pattern de mock de generics.

## Decisao Pos-Fix

- **Production-ready: sim.** Drift documental endereçado; movimentacao de sentinel valida com regressao tripla; doc.go documentado; ADR-007 sincronizado; ADR-005-bis registra shape pos-E2; task-7.0 alinhada a R6.4; RF-14 com log positivo provado e raw PII bloqueado por teste; outbox idempotencia validada contra Postgres real; mockery v3 config valida.
- **Sem falso positivo:** todos achados confrontados contra working tree real; orfao `e` era truncamento `git status`; race do test de outbox descoberta e isolada como artefato de teste (nao de producao).
- **MVP robusto:** lint zero issues; 11 pacotes identity verdes (`-race`); 7 testes integration outbox verdes; 11 testes integration identity verdes; nenhuma alteracao de contrato publico (sentinels mantem nome/tipo; `IdentityModule` intocado em campos/construtor; migrations intocadas; handler API JSON inalterada; payload de resposta inalterado).
- **Pronto para main.** Sem regressao introduzida nesta sessao. Riscos residuais sao pre-existentes baseline ou follow-ups futuros nao-bloqueantes.
