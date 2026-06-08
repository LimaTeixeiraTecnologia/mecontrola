# Tarefa 9.0: HTTP routes + wiring module.go + cmd/api/main.go shutdown order + cross-PRD bumps

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Expõe os endpoints HTTP do webhook unificado WhatsApp, finaliza o wiring no `module.go` do identity, ordena os lifecycle hooks em `cmd/api/main.go` para shutdown determinístico, e bump cross-PRD nos PRDs dependentes (`prd-card-crud-mvp`, `prd-categories-crud`, `prd-budgets-monthly`) removendo a referência ao `RequireUser` transitório.

<requirements>
- RF-21: atualizar PRDs dependentes (`card`, `categories`, `budgets`) com spec-version bump removendo `X-User-ID` transitório.
- RF-25: rotas HTTP `POST /api/v1/whatsapp/inbound` e `GET /api/v1/whatsapp/verify` com middlewares na ordem canônica (signature.Compose(...) primeiro).
- Shutdown order canônico em `cmd/api/main.go`: HTTP → Dispatcher webhook → Limiter → Outbox consumers → Housekeeping → PG.
</requirements>

## Subtarefas

- [ ] 9.1 Adicionar rotas em `internal/identity/infrastructure/http/server/router.go` (ou novo router em `internal/platform/whatsapp` registrado no módulo de identity): `POST /api/v1/whatsapp/inbound` com middleware `signature.Compose(...)` → handler do dispatcher; `GET /api/v1/whatsapp/verify` com handler de verify_token Meta.
- [ ] 9.2 Criar `internal/platform/whatsapp/handlers/verify_handler.go` (lê `hub.mode`, `hub.verify_token`, `hub.challenge`; retorna `hub.challenge` se token bate; 403 caso contrário).
- [ ] 9.3 Criar `internal/platform/whatsapp/handlers/inbound_handler.go` invocando `dispatcher.Route(ctx, raw)`; mapeia outcomes para status HTTP (200 sempre exceto signature inválido = 401 e 503 quando PG/outbox indisponível).
- [ ] 9.4 Atualizar `internal/identity/module.go` consolidando wiring de todos os componentes: `EstablishPrincipal`, `AuthEventsConsumer`, `AuthEventsHousekeepingJob`, `Limiter`, `Dispatcher`, `StubAgent`, novos handlers.
- [ ] 9.5 Atualizar `cmd/api/main.go` com ordem canônica de Start (PG → outbox → Limiter.Start → Consumer → Housekeeping → HTTP server.Serve) e Shutdown reverso conforme techspec `## Shutdown order canônico (C1-bis)`.
- [ ] 9.6 Criar `cmd/api/main_test.go` (ou test do lifecycle em pacote dedicado) validando que Shutdown em SIGTERM simulado completa em < 15s e não vaza goroutines.
- [ ] 9.7 Bump `.specs/prd-card-crud-mvp/prd.md` (spec-version) removendo `RequireUser` transitório por `X-User-ID` e referenciando `prd-auth-foundation`.
- [ ] 9.8 Bump `.specs/prd-categories-crud/prd.md` (mesmo padrão).
- [ ] 9.9 Bump `.specs/prd-budgets-monthly/prd.md` (mesmo padrão).

## Detalhes de Implementação

Ver techspec `## Endpoints de API`, `## Shutdown order canônico (C1-bis)` e `## Componentes modificados`. Para mapeamento outcome → HTTP status, ver tabela em `## Endpoints de API`.

## Critérios de Sucesso

- `curl POST /api/v1/whatsapp/inbound` com HMAC válido e payload Meta válido retorna 200 + linha em `auth_events`.
- `curl POST /api/v1/whatsapp/inbound` com assinatura inválida retorna 401 sem corpo descritivo.
- `curl GET /api/v1/whatsapp/verify?hub.mode=subscribe&hub.verify_token=...&hub.challenge=...` retorna o challenge.
- Test de lifecycle: SIGTERM aciona shutdown ordenado; `go test -run TestLifecycle -v` valida que nenhuma goroutine vaza (delta de `runtime.NumGoroutine` ≤ baseline + tolerância).
- PRDs dependentes têm spec-version bump documentado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários dos handlers
- [ ] Test de integração end-to-end via servidor HTTP real (httptest)
- [ ] Test de lifecycle/shutdown
- [ ] Verificação manual de spec-version bumps nos 3 PRDs dependentes

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/handlers/inbound_handler.go` + `_test.go` (criar)
- `internal/platform/whatsapp/handlers/verify_handler.go` + `_test.go` (criar)
- `internal/identity/infrastructure/http/server/router.go` (atualizar)
- `internal/identity/module.go` (atualizar — wiring final)
- `cmd/api/main.go` (atualizar — lifecycle order)
- `cmd/api/main_test.go` ou `cmd/api/lifecycle_test.go` (criar)
- `.specs/prd-card-crud-mvp/prd.md` (atualizar — bump)
- `.specs/prd-categories-crud/prd.md` (atualizar — bump)
- `.specs/prd-budgets-monthly/prd.md` (atualizar — bump)
