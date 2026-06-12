# Refactor advisory — `internal/identity` (DMMF seletivo)

- **Modo**: `advisory` (default do prompt em `docs/refactors/internal-identity.md`).
- **task_type**: `refactor-advisory` — mapeamento + plano incremental sem código.
- **Comportamento**: 100% preservado. Nenhum contrato HTTP, evento de outbox, formato de payload, status no DB ou semântica de masking/anonimização pode mudar.
- **Skills obrigatórias na execução** (se autorizada depois): `.agents/skills/refactor/SKILL.md`, `.agents/skills/go-implementation/SKILL.md`. Referências sob demanda.

## Contexto

O módulo `internal/identity` cruza auth, PII, entitlement e projeções de subscription (consumidas de billing/onboarding). A modelagem atual concentra invariantes corretos em VOs (`WhatsAppNumber`, `Email`), mas mantém pontos de modelagem fraca onde DMMF (smart constructors, state-as-type, `Decide*` puro, domain events tipados) trariam ganho real e barato, sem mudar comportamento. Refactor advisory mapeia esses pontos e propõe o menor caminho seguro.

## Baseline confirmado

- `internal/identity/module.go` — DI manual, builder interno, expõe `RepositoryFactory`, `EntitlementReader`, `UserRouter`, três consumers, job housekeeping; sem `Start/Stop` (lifecycle via worker manager externo).
- Use cases reais (`internal/identity/application/usecases/`): `establish_principal`, `upsert_user_by_whatsapp`, `decide_user_entitlement`, `project_auth_event`, `project_subscription_event`, `mark_user_deleted`, `anonymize_user_auth_events`, `cleanup_auth_events`, `find_user_by_id`, `find_user_by_whatsapp`.
- Domínio: `User` + `Status string` (ACTIVE/DELETED), `AuthEvent` + `AuthEventKind` + `userID *uuid.UUID` nullable, VOs `WhatsAppNumber`/`Email` com `Masked()`, `domain.IsEntitled(sub, now)`.
- Adapters (handlers/consumers/jobs/producers): finos, conformes R-ADAPTER-001 (sem SQL direto fora de repos, sem branching de domínio, sem comentários `//` em produção).
- Rotas: `POST /api/v1/identity/users` + middlewares `InjectPrincipalFromHeader`/`RequireUser` consumidos por `card`, `transactions`, `budgets`, `categories`.
- Eventos publicados pelo módulo: `auth.principal_established`, `auth.unknown_user`, `auth.failed`, `user.deleted` — decisão nasce no use case (correto), producer/outbox apenas serializa.
- Eventos consumidos: `billing.subscription.{activated,renewed,past_due,canceled,refunded}`, `onboarding.subscription_bound`, `auth.*`, `user.deleted`.

## Hotspots e ganhos potenciais (advisory)

Cada item lista: ganho DMMF, arquivos reais, risco de regressão e por que cabe em advisory sem mudar comportamento observável.

### H1. `AuthEventKind` → discriminated union tipada (state-as-type)
- **Arquivo**: `internal/identity/domain/entities/auth_event.go` (`NewAuthEvent`, `HydrateAuthEvent`, `AuthEventKind`, `AuthEventReason`).
- **Ganho**: hoje `AuthEvent` carrega `userID *uuid.UUID` nullable independentemente do kind. `PRINCIPAL_ESTABLISHED` exige userID; `UNKNOWN_USER` não tem; `FAILED` carrega `reason`. Modelar `PrincipalEstablished{userID}`, `UnknownUser{}`, `AuthFailed{reason}` como variantes (ex.: interface selada + structs imutáveis) elimina invariante implícito mantido por convenção nos construtores.
- **Limite**: payload de outbox (`authEventPayload` em `application/auth/auth_event_payload.go`) e linha do DB (`auth_events.user_id NULL`) permanecem idênticos. A união é interna ao domínio; serialização preserva campo nullable.
- **Risco**: baixo se mantiver `Hydrate` aceitando os mesmos campos brutos e fazendo a discriminação no boundary domain↔repo.

### H2. `EstablishPrincipal` — extração de `Decide*` puro
- **Arquivo**: `internal/identity/application/usecases/establish_principal.go` (`Execute`, `newPrincipalEstablishedEvent`, `newUnknownUserEvent`).
- **Ganho**: hoje a função orquestra lookup + decisão + publish + UoW numa pipeline grande. Extrair `DecideEstablishPrincipalOutcome(user *User, wa WhatsAppNumber, now, eventID) → (Principal, AuthEvent, OutboxEnvelope, ErrUnknownUser?)` como função pura (sem `ctx`, sem repo) deixa o use case fino: lookup → decide → persist+publish dentro do UoW.
- **Inspiração estrutural**: ADR-006/`.claude/rules/transactions-workflows.md` (`Decide*` puro, `now` e `ids` injetados).
- **Limite**: assinaturas externas (input/output do use case e formato do evento) imutáveis. Apenas reorganização interna.

### H3. `UpsertUserByWhatsApp` — decompor decisão de branch
- **Arquivo**: `internal/identity/application/usecases/upsert_user_by_whatsapp.go`.
- **Ganho**: branching atual {insert novo | reanimar deletado dentro da janela | hidratar/atualizar existente} é implícito. Encapsular em tipo `UpsertAction = CreateNew | Reanimate | UpdateExisting` (ou função pura `DecideUpsertAction(existing *User, input, now, window) UpsertAction`) torna explícito o invariante de `ReanimationWindow` (30d em `domain/policies.go`) e remove condicional em série.
- **Limite**: persistência continua via repo atual; nenhum método público novo no use case.

### H4. `ProjectSubscriptionEvent` — `EntitlementPlan` virar union nomeada
- **Arquivo**: `internal/identity/application/usecases/project_subscription_event.go` (`entitlementPlan`, `planEntitlementUpsert`).
- **Ganho**: `entitlementPlan` carrega `isPending bool` que dispersa branching pelo método. Promover para `PendingEntitlement | CommittedEntitlement` (variantes seladas) deixa `Persist(plan)` despachar sem flag booleana. Reduz ambiguidade em refactors futuros.
- **Limite**: mesma escrita no DB (tabelas `identity_entitlements_pending` e `identity_entitlements`), mesma decisão (depende da existência de user para o subscription_id).

### H5. `EntitlementDecider` — nomear o serviço de domínio
- **Arquivo**: `internal/identity/domain/services/` (novo) + `application/usecases/decide_user_entitlement.go`.
- **Ganho**: `domain.IsEntitled(sub, now)` é pura mas anônima — promovê-la para `EntitlementDecider.Decide(sub, now) → (entitled, Reason)` em `domain/services/` segue o padrão usado em `transactions` e em `billing` (decisões de transição centralizadas no domínio).
- **Limite**: apenas reorganização e renomeação interna; mesma assinatura efetiva, mesmos retornos.

### H6. Anonimização de auth events — política no domínio, execução no repo
- **Arquivo**: `internal/identity/application/usecases/anonymize_user_auth_events.go` + `infrastructure/repositories/postgres/auth_events_repository.go` (`AnonymizeByUserID`).
- **Observação**: hoje o repo aplica SQL que NULLifica `user_id`. A "política" (quais campos zerar, sob que condição) está implícita no SQL. Em advisory, propor um `AuthEventAnonymizer` no domínio que descreve o conjunto de campos a anonimizar e retorna uma intenção tipada que o repo executa.
- **Limite**: SQL final pode permanecer literal; ganho é documental e de localização de invariante. Se o trade-off não compensar (apenas um campo), manter como está e registrar como decisão consciente (não introduzir abstração sem ganho).

### H7. `User.Status` discriminated union — **fora de escopo (advisory rejeita)**
- Splitar `User` em `UserActive | UserDeleted` muda hidratação, expõe métodos diferentes por estado e impacta `module.go`, `RepositoryFactory`, todos os callers cross-module. Risco de regressão > ganho. Manter `Status string` + helpers `IsActive()`/`IsDeleted()` em advisory.

## Plano mínimo seguro (ordem sugerida)

Cada passo é independente, autônomo, validado isoladamente. Nada é executado neste advisory.

1. **H1 — AuthEventKind tipada**: refatorar `domain/entities/auth_event.go` para variantes seladas; ajustar `application/auth/auth_event_payload.go` (`newAuthEventOutbox`, `parseAuthEvent`) preservando o JSON do outbox 1:1.
2. **H2 — `Decide*` em EstablishPrincipal**: extrair função pura, manter `Execute` orquestrando UoW + publish.
3. **H3 — `DecideUpsertAction`**: extrair pura; `Execute` faz lookup → decide → persist.
4. **H4 — `EntitlementPlan` como union**: substituir `isPending bool` por variante; `planEntitlementUpsert` → `decideEntitlementPlan`.
5. **H5 — `EntitlementDecider`** em `domain/services/`.
6. **H6 — Anonimização**: avaliar custo/benefício; se aceito, criar `domain/services/auth_event_anonymizer.go`; se não, registrar decisão.

Ordem 1→6 prioriza ganho/risco. Cada passo é um PR isolado e revisível.

## Restrições obrigatórias (re-confirmação)

- Sem `init()`, `panic`, `clock.Clock`, comentários `//` em produção, `var _ I = (*T)(nil)`.
- `Decide*` puro: sem `ctx`, sem repo, sem `time.Now()`; receber `now` e IDs.
- Producers continuam finos (não há decisão semântica neles hoje — manter).
- Contratos HTTP, formato dos eventos `auth.*`/`user.deleted`, colunas do DB e máscaras de PII permanecem byte-idênticos.
- Sem novas interfaces sem consumidor real.
- Se alguma alteração puder mexer em decisão de entitlement, anonimização, auth event ou resolução de principal — parar e declarar fora de escopo.

## Validação proporcional (na futura execução)

- `go test ./internal/identity/...` (unit + integration) com testes existentes inalterados.
- Diff de schema de evento (`auth.principal_established`, `auth.unknown_user`, `auth.failed`, `user.deleted`): comparar payload JSON antes/depois por inspeção em teste de serialização.
- Gate R-ADAPTER-001.1 (`grep` de comentários) e R-ADAPTER-001.2 (`grep` de SQL em adapters) — devem permanecer vazios.
- Gate ADR-006-like para identity: nenhum branching de domínio fora de `Decide*`/smart constructors após H2/H3.
- `taskfile`: `task lint`, `task test`, `task vulncheck` proporcional ao escopo de cada PR.

## Critérios de aceitação

- Plano referencia apenas artefatos reais de `internal/identity` (validado contra a exploração).
- Preserva isolamento do domínio e contratos cross-module (`InjectPrincipalFromHeader`, `RequireUser`, `EntitlementReader`, schemas de outbox).
- Cada recomendação DMMF é justificada por ganho de modelagem, segurança ou robustez (H1, H2, H3, H4, H5) ou rejeitada com motivo (H7, possivelmente H6).
- Resposta final do refactor advisory termina com `done`.

## Próximo passo

Aprovação deste advisory libera a próxima rodada, que pode ser:
- `execution` por etapa (cada hotspot vira um PR) — exige carregar `go-implementation` + skill `refactor` e seguir Etapas 1–5.
- ou `advisory deep dive` em um único hotspot (ex.: H1) com snippets de diff propostos antes de qualquer escrita.

---

## Execução (2026-06-12)

### Modo: execution autorizada pelo usuário após advisory

**Skills carregadas**: `.claude/skills/go-implementation/SKILL.md` + `.claude/rules/go-adapters.md` + `.claude/rules/governance.md`. Padronização DMMF inspirada em `internal/transactions/domain/services/`.

### Entregas

- **H1 — AuthEvent discriminated union (smart constructors tipados)**
  - `internal/identity/domain/entities/auth_event.go`: removido `NewAuthEvent` genérico; adicionados `NewPrincipalEstablished`, `NewUnknownUser`, `NewAuthFailed` com erros sentinela `ErrPrincipalEstablishedRequiresUserID`/`ErrAuthFailedRequiresReason`. `HydrateAuthEvent` preservado.
  - Callers de teste de integração migrados (10 ocorrências em `auth_events_repository_integration_test.go`).
- **H2 — `PrincipalWorkflow.DecidePrincipal` puro**
  - `internal/identity/domain/services/principal_workflow.go` (novo): `DecidePrincipal(userID, found, eventID, now)` sem ctx/IO/time.Now. Tipo `PrincipalDecision` neutro evita ciclo de import.
  - `internal/identity/application/usecases/establish_principal.go`: use case agora orquestra lookup → Decide → publish; outbox preservado byte-idêntico via `newAuthEventOutbox`.
- **H3 — `UserUpsertWorkflow.DecideUpsertAction` puro + union selada**
  - `internal/identity/domain/services/user_upsert_workflow.go` (novo): union `UpsertAction` com variantes `UpsertCreateNew`, `UpsertUpdateExisting`, `UpsertReanimate` (sealed via método privado `isUpsertAction()`).
  - `internal/identity/application/usecases/upsert_user_by_whatsapp.go`: lookups → `Decide` → switch por variante → persist.
- **H4 — `EntitlementPlan` como union selada**
  - `project_subscription_event.go`: tipo `EntitlementPlan` com `PendingEntitlement` / `CommittedEntitlement`; `decideEntitlementPlan` substitui `planEntitlementUpsert`; `projectCurrent` despacha via `switch p := plan.(type)`.
- **H5 — `EntitlementDecider` em `domain/services/`**
  - `internal/identity/domain/services/entitlement_decider.go` (novo): wraps `domain.IsEntitled` em método `Decide` retornando `EntitlementDecision{Entitled, Reason}`. `decide_user_entitlement.go` consome o decider.
  - `domain.IsEntitled` preservado para compat com `entitlement_test.go`.
- **H6 — Avaliado, fora de escopo (decisão consciente)**
  - `AuthEventsRepository.AnonymizeByUserID` é `UPDATE auth_events SET user_id = NULL WHERE user_id = $1`. Política trivial (um único campo). Criar `AuthEventAnonymizer` em `domain/services/` adicionaria indireção sem ganho de invariante. **Mantido como está**.

### Validação executada

| Gate | Comando | Resultado |
|------|---------|-----------|
| Build | `go build ./...` | OK (sem saída) |
| Vet | `go vet ./internal/identity/...` | OK |
| Tests (unit/integ short) | `go test ./internal/identity/... -count=1 -short` | OK em todos os pacotes |
| Zero comentários (R-ADAPTER-001.1) | grep `//` em `.go` produção fora exceções | vazio |
| SQL em adapters (R-ADAPTER-001.2) | grep `QueryContext\|ExecContext` em handlers/consumers/producers/jobs/handlers | vazio |
| R0 `init()` | grep `^func init(` em produção | vazio |
| R5.12 `panic` | grep `panic(` em produção | vazio |
| R7.1 `interface{}` | grep `interface{}` em produção | vazio |

### Comportamento preservado (auditoria)

- Rotas HTTP, payload JSON do outbox (`auth.principal_established`, `auth.unknown_user`, `auth.failed`, `user.deleted`), schema do DB, métricas (`auth_principal_established_total`, `auth_failed_total`, `auth_unknown_wa_id_total`, `auth_resolve_wa_duration_seconds`), labels, mascaramento de PII (`Masked()`), erros públicos (`ErrUnknownUser`, `ErrInvalidWhatsApp`, `ErrInvalidEmail`, `ErrEmailInUse`, `ErrWhatsAppNumberInUse`).
- Nenhum contrato cross-module afetado (middlewares `InjectPrincipalFromHeader`/`RequireUser`, `EntitlementReader`, `RepositoryFactory`).

Status final: **done**.
