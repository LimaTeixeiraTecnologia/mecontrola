# Registro de Decisão Arquitetural (ADR-006)

## Metadados

- **Título:** Idempotência atômica via `uow.UnitOfWork` — `Storage.Put` dentro da mesma transação da escrita de negócio
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Jailton (tech lead)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (US-05, F-04, RF-30–RF-32), `.specs/prd-card-crud-mvp/adr-001-platform-idempotency-package.md`, `.specs/prd-card-crud-mvp/techspec.md`

## Contexto

Garantir exactly-once em `POST/PUT/DELETE /api/v1/cards` exige que a gravação do registro idempotente e a escrita do recurso de negócio sejam atômicas. Existem três posicionamentos viáveis:

1. **Middleware grava sempre** (pattern clássico Stripe-style): janela de inconsistência se o servidor cai entre `COMMIT` da tx do INSERT e o `INSERT` na `idempotency_keys`. Retry duplica o cartão.
2. **Use case grava dentro do UoW**: exactly-once real. Exige propagar `IdempotencyContext` do middleware para o use case via `context.Context`.
3. **Two-phase (placeholder + UPDATE)**: grava key com body vazio em tx, faz UPDATE pós-resposta. Retry pode encontrar placeholder e disparar bug.

O repo já adota `uow.New[T](mgr, uow.WithObservability(o11y))` em `internal/identity` (`UpsertUserByWhatsApp`, `EstablishPrincipal`, `MarkUserDeleted`) e `internal/billing` (`ProcessSaleApproved`, `ProcessSubscriptionRenewed` etc.). Use cases mutadores recebem o UoW e chamam `Execute(ctx, fn)` para encapsular a transação. Reaproveitar este pattern minimiza fricção.

Decisão complementar (UX): respostas 4xx (validation errors) também devem ser cacheadas para que retries com mesma `Idempotency-Key` retornem o mesmo erro byte-idêntico, mas não há benefício de exactly-once em validation (é determinística por definição) — portanto pode ficar fora do UoW.

## Decisão

Adotar o seguinte split de responsabilidade entre middleware e use case:

### Middleware (`internal/platform/idempotency.Middleware`)

1. Lê `Idempotency-Key` (1–128 ASCII). Ausência → 400 `missing_idempotency_key`.
2. Calcula `request_hash = sha256(body)`. Body é re-injetado em `r.Body` via `io.NopCloser(bytes.NewReader)`.
3. Lê `auth.Principal` do ctx (canônico identity). Ausência → 401 (o `RequireUser` já garantiu, mas defesa em profundidade).
4. `Storage.Get(scope, key, userID)`:
   - hit + `request_hash` igual: escreve `response_status` + `response_body` direto no `ResponseWriter`. Log `card.idempotency.replay`. **Fim**.
   - hit + `request_hash` diferente: 409 `idempotency_conflict`. Log `card.idempotency.hash_mismatch`. **Fim**.
   - miss: injeta `IdempotencyContext{Scope, Key, UserID, RequestHash, ExpiresAt: now+ttl}` no ctx via `idempotency.WithContext` e invoca `next.ServeHTTP` em um `responseRecorder` com cap 64 KB.
5. Após `next` retornar:
   - Se status ∈ [200, 299]: assume que o use case já gravou via UoW. Middleware NÃO grava. Faz `Storage.Get` rápido em modo debug-assertion (apenas em testes) para detectar bug do desenvolvedor que esqueceu de chamar `Storage.Put` no use case.
   - Se status ∈ [400, 499]: middleware grava `Storage.Put` em tx separada (best-effort). Falha de gravação loga `card.idempotency.put_4xx_failed` mas NÃO altera resposta ao cliente.
   - Se status ∈ [500, 599]: nada é gravado.
   - Se `responseRecorder` reportar `ErrResponseTooLarge`: o handler já terminou (response possivelmente truncada antes do flush — `responseRecorder` faz buffer, então client ainda não recebeu). Middleware troca por 500 `internal_error` e descarta cache.

### Use Case (`internal/card/application/usecases/{create,update,soft_delete}_card.go`)

```go
type CreateCard struct {
    uow     uow.UnitOfWork[entities.Card]
    factory interfaces.RepositoryFactory
    mgr     manager.Manager
    idem    idempotency.Storage
    o11y    observability.Observability
}

func (u *CreateCard) Execute(ctx context.Context, in input.CreateCard) (output.Card, error) {
    ic, hasIdem := idempotency.FromContext(ctx)
    card, err := u.uow.Execute(ctx, func(ctx context.Context) (entities.Card, error) {
        repo := u.factory.CardRepository(u.mgr.DBTX(ctx))
        c, err := entities.NewCard(in)
        if err != nil { return entities.Card{}, err }
        if err := repo.Insert(ctx, c); err != nil { return entities.Card{}, err }
        if hasIdem {
            body, err := json.Marshal(toOutput(c))
            if err != nil { return entities.Card{}, fmt.Errorf("create_card: marshal output: %w", err) }
            rec := idempotency.Record{
                Scope: ic.Scope, Key: ic.Key, UserID: ic.UserID,
                RequestHash: ic.RequestHash,
                ResponseStatus: http.StatusCreated, ResponseBody: body,
                ExpiresAt: ic.ExpiresAt,
            }
            if err := u.idem.Put(ctx, rec); err != nil {
                return entities.Card{}, fmt.Errorf("create_card: idempotency put: %w", err)
            }
        }
        return c, nil
    })
    if err != nil { return output.Card{}, err }
    return toOutput(card), nil
}
```

A `Storage.Put` chama `u.factory.CardRepository(mgr.DBTX(ctx))` — mas para `idempotency_keys` usa-se `idempotency.NewPostgresStorage(mgr.DBTX(ctx))` injetado. Ambos compartilham o `DBTX` que o UoW está orquestrando, garantindo participação na mesma transação.

`UpdateCard` e `SoftDeleteCard` seguem o mesmo padrão. `Get` e `List` (read-only) não usam UoW nem `Storage.Put`.

## Alternativas Consideradas

1. **Middleware grava sempre via `responseRecorder` (Opção 1 do contexto)** — Vantagens: use case ignora idempotência, código menor; Desvantagens: janela de inconsistência crash-time entre commit do INSERT e INSERT do idempotency_keys (não é exactly-once real). Rejeitada porque PRD US-05 exige zero duplicação em retries.
2. **Two-phase (Opção 3)** — Vantagens: middleware mantém responsabilidade; Desvantagens: placeholder pode ser servido em retry; UPDATE adiciona round-trip. Rejeitada.
3. **`uow.UnitOfWork` para 2xx e 4xx** — Vantagens: simetria; Desvantagens: validation errors não precisam de exactly-once (são determinísticos), só de cache best-effort para UX; manter dentro do UoW exigiria abrir tx mesmo em erro de decode (overhead). Rejeitada — manter 4xx no middleware é mais barato e suficiente.
4. **Middleware injeta callback `commitIdempotency(status, body)` no ctx** — Vantagens: use case não conhece `idempotency.Record`; Desvantagens: closure dentro de ctx é anti-pattern Go; testes ficam frágeis. Rejeitada.

## Consequências

### Benefícios Esperados

- Exactly-once real em 2xx: zero duplicação em retries mesmo com crash entre operações.
- Cache de 4xx mantém UX consistente sem custo de tx adicional.
- Reuso do pattern `uow.UnitOfWork` já em produção (identity, billing) — sem novas abstrações.
- Replay byte-idêntico: response_body persistido é o mesmo bytes que `json.Marshal` produzirá na próxima chamada (encoder estável).

### Trade-offs e Custos

- Use case fica acoplado a `idempotency.Storage` (interface) e ao `IdempotencyContext`. Mitigação: dependência é opcional via `hasIdem` flag (use case continua executável fora de fluxo HTTP, ex.: porta interna `CardLookup` ou testes diretos).
- JSON serialization no use case duplica o esforço do handler. Custo desprezível (~µs); benefício é replay determinístico.
- Tx aberta inclui call externa ao `idempotency` storage → tempo de tx ligeiramente maior. Cap em 64 KB de body limita o INSERT.

### Riscos e Mitigações

- **Bug do dev esquecer de chamar `Storage.Put` no use case** → resposta 2xx volta sem cache; retries re-executam (duplicação de cartão!). Mitigação: teste de integração de "exactly-once" que verifica linha em `idempotency_keys` após cada 2xx. Adicionalmente, middleware em build de teste faz `Storage.Get` pós-handler e falha o teste se ausente.
- **Drift entre encoder do use case e do handler** (campos opcionais, ordem) → replay difere. Mitigação: ambos chamam `json.Marshal(toOutput(card))` com mesmo DTO; teste de replay byte-a-byte cobre.
- **`UoW.Execute` rola back se `Storage.Put` falhar** → cartão NÃO é persistido. Conservador e correto: retry tenta de novo. Validado em teste com mock que injeta erro no `Put`.
- **TTL extension em retries** → não há (cada hit retorna a row existente sem reset de `expires_at`). Comportamento previsível.

## Plano de Implementação

1. Implementar `internal/platform/idempotency/{storage,postgres_storage,context,middleware,recorder}.go`.
2. Migrar use cases mutadores do `card` para receber `uow.UnitOfWork[entities.Card]` + `idempotency.Storage`.
3. Testes:
   - Unitário do middleware: hit-match, hit-mismatch, miss + 2xx (não grava), miss + 4xx (grava best-effort), miss + 5xx (não grava).
   - Unitário do use case com mock de `Storage.Put` retornando erro → assert que `UoW.Execute` rolou back e `repo.Insert` foi revertido.
   - Integração: 10 goroutines paralelas mesma key → 1 cartão criado, 9 replays byte-idênticos; 1 linha em `idempotency_keys`.
4. Atualizar `module.go` para injetar `idempotency.PostgresStorage` nos use cases.

Adoção concluída quando: todos os testes verdes + métrica `card_create_duplicates_total` (futura) = 0 em 30 dias.

## Monitoramento e Validação

- Span `card.idempotency.middleware` com atributo `outcome=replay|conflict|pass|4xx_cached`.
- Log `card.idempotency.replay` (hit), `card.idempotency.hash_mismatch` (409), `card.idempotency.put_4xx_failed` (warning).
- Critério de revisão: 0 incidente de duplicação reportado em 30 dias após release.
- Critério de reversão: se latência p99 de POST aumentar > 30% atribuível ao `Storage.Put` dentro do UoW, considerar variante "metadata-only no UoW + body via UPDATE pós-commit".

## Impacto em Documentação e Operação

- `AGENTS.md` — adicionar ao seção "Plataforma Compartilhada" o pattern `idempotency.FromContext` consumido por use cases.
- `docs/runbooks/card-rollback.md` — descrever consulta `SELECT * FROM mecontrola.idempotency_keys WHERE scope='card' AND key=$1` para diagnóstico.
- Onboarding técnico: incluir exemplo de `CreateCard.Execute` no playbook de "como adicionar endpoint idempotente".

## Revisão Futura

Revisitar quando:

- Outro módulo (billing, identity) migrar para `internal/platform/idempotency` — possível extração de helper `WithIdempotencyCommit(uow, idem, key, status, body) func(ctx)` para reduzir boilerplate.
- Volume de 4xx cacheados crescer expressivamente — avaliar mover 4xx também para o UoW.
- `uow.UnitOfWork` do devkit-go mudar API — atualizar pattern.
