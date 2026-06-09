# Relatorio de Bugfix

- Total de bugs no escopo: 5
- Corrigidos: 5 (F2 e F5 endereçados diretamente nesta sessao; F1, F3, F4 ficaram obsoletos pelo avanço da migração Strangler Fig durante a sessão — ver Nota sobre evolução do working tree)
- Testes de regressao adicionados: 1 (`TestComposeWithStatus_CallbackCountsPerScenario` com 4 sub-cenarios, cobrindo F5 e travando o contrato cuja violação gerou F1)
- Pendentes: nenhum
- Estado final: done

## Nota sobre evolução do working tree

Durante a sessão, o working tree avançou de um estado intermediário (revisado: handler `WhatsAppInboundHandler` migrando para `dedup.MessageRepository` + `signature.ComposeWithStatus` com double-count) para o estado final do Strangler Fig (`prd-auth-foundation` RF-28 / ADR-002): o webhook HTTP de onboarding foi desativado (`Handle` retorna 410 Gone) e o processamento inbound passou para `services.WhatsAppMessageProcessor`. Esse avanço apagou organicamente o ponto de injeção que originava F1, F3 e F4. F2 e F5 continuaram aplicáveis e foram corrigidos no diff.

## Bugs

- ID: F1
- Severidade: major
- Origem: finding de review (Strangler Fig PR `prd-onboarding-magic-token` spec-v2 / `prd-auth-foundation` RF-28); impactava RF-13 (telemetria)
- Estado: fixed (por evolução do working tree)
- Causa raiz: o wiring em `internal/onboarding/module.go` chamava `signature.ComposeWithStatus(..., onInvalid, onStatus)` registrando o mesmo counter `meta_signature_status_total` em ambos os callbacks. Como `signature.HMACWithMetrics` chama `onStatus` incondicionalmente e `onInvalid` apenas quando status==invalid, todo request inválido produzia 2 incrementos com a mesma label.
- Resolução: o trecho ofensor (`sigStatusC`, `NewWhatsAppRouterWithStatus`, `buildRouters`) foi inteiramente removido durante o avanço do Strangler Fig. O módulo passou a usar `services.NewWhatsAppMessageProcessor`, sem callbacks duplicados. Não existe mais o ponto de bug.
- Arquivos alterados: nenhum por esta skill (resolvido pela migração); `internal/onboarding/module.go` no working tree atual demonstra a remoção.
- Teste de regressao: `internal/platform/whatsapp/signature/compose_test.go::TestComposeSuite/TestComposeWithStatus_CallbackCountsPerScenario` — adicionado por esta skill (F5). Trava o contrato dos callbacks de `signature.ComposeWithStatus`: `onStatus` chamado exatamente 1x com o status real e `onInvalid` chamado apenas quando inválido. Protege contra qualquer reintrodução futura do wiring duplicado.
- Validacao: `go test -count=1 -v -run "TestComposeSuite/TestComposeWithStatus_CallbackCountsPerScenario" ./internal/platform/whatsapp/signature/` PASS (4 sub-cenarios).

- ID: F2
- Severidade: minor
- Origem: finding de review (Strangler Fig incompleto — ADR-002 / RF-28 do `prd-auth-foundation`)
- Estado: fixed
- Causa raiz: caminho antigo (`internal/onboarding/.../MetaMessageRepository`) permanecia vivo após a substituição pelo `internal/platform/whatsapp/dedup.MessageRepository`. Duas implementações Postgres apontavam para `onboarding.meta_processed_messages` — risco de drift.
- Arquivos alterados:
  - Deletados: `internal/onboarding/application/interfaces/meta_message_repository.go`, `internal/onboarding/application/interfaces/mocks/meta_message_repository.go`, `internal/onboarding/infrastructure/repositories/postgres/meta_message_repository.go`
  - Modificados: `internal/onboarding/application/interfaces/repository_factory.go` (removido `MetaMessageRepository(db) MetaMessageRepository`), `internal/onboarding/infrastructure/repositories/factory.go` (removida implementação correspondente), `mockery.yml` (removida entrada `MetaMessageRepository`), `internal/onboarding/application/interfaces/mocks/repository_factory.go` (regenerado).
- Teste de regressao: cobertura indireta via `go build ./internal/onboarding/... ./internal/platform/whatsapp/...` (qualquer consumidor residual quebraria a compilação) + `grep -rn "MetaMessageRepository\b" --include="*.go"` retorna 0 referências de símbolo (apenas comentário textual sobre a tabela permanece em `dedup/postgres/repository.go`).
- Validacao: `go build` OK; `go vet ./internal/onboarding/... ./internal/platform/whatsapp/...` OK; `grep` final confirma extinção.

- ID: F3
- Severidade: minor
- Origem: finding de review (acoplamento de teste por duck typing)
- Estado: fixed (por evolução do working tree)
- Causa raiz: `whatsapp_inbound_handler_test.go` usava o mock antigo `appinterfacesmocks.NewMetaMessageRepository` para satisfazer o parâmetro `dedup.MessageRepository` por shape idêntico.
- Resolução: na evolução do Strangler Fig, o handler `WhatsAppInboundHandler` deixou de injetar dedup (virou stub 410 Gone, ver `whatsapp_inbound_handler.go` no working tree). Não há mais teste do handler que injete `dedup.MessageRepository`. O risco de acoplamento foi eliminado por remoção do call site.
- Arquivos alterados: nenhum por esta skill (resolvido pela migração).
- Teste de regressao: como precaução para uso futuro do mock canônico em outros call sites (ex.: `services.WhatsAppMessageProcessor` ou consumidores do `dedup.MessageRepository`), esta skill manteve a entrada `internal/platform/whatsapp/dedup.MessageRepository` em `mockery.yml` e o mock gerado em `internal/platform/whatsapp/dedup/mocks/message_repository.go`. Qualquer novo teste que precisar do dedup já encontra o mock canônico pronto.
- Validacao: `go test -count=1 ./internal/onboarding/infrastructure/http/server/handlers/...` PASS; `grep "appinterfacesmocks.NewMetaMessageRepository"` retorna vazio.

- ID: F4
- Severidade: minor
- Origem: finding de review (parâmetro morto pós-migração)
- Estado: fixed (por evolução do working tree)
- Causa raiz: `buildRouters(... factory appinterfaces.RepositoryFactory ...)` deixou de consumir o parâmetro após a migração para dedup, poluindo a fronteira do helper.
- Resolução: na evolução do Strangler Fig, `buildRouters` foi renomeado para `buildPublicRouter` e o parâmetro `factory` foi removido por completo. A chamada em `NewOnboardingModule` foi simplificada para `publicRouter := buildPublicRouter(cfg, ucs, o11y)`. Sem assinatura morta restante.
- Arquivos alterados: nenhum por esta skill (resolvido pela migração).
- Teste de regressao: não exigido — `go build ./...` falha imediatamente em qualquer regressão de assinatura.
- Validacao: `go build ./internal/onboarding/...` OK.

- ID: F5
- Severidade: minor
- Origem: finding de review (cobertura de teste insuficiente — sem essa cobertura, F1 não seria detectado pelo CI)
- Estado: fixed
- Causa raiz: `compose_test.go` validava apenas o caminho feliz (raw body disponível + valid + 401 + `onInvalid` chamado), mas não asseverava a CONTAGEM por status. Sem esse invariante, regressões de wiring no tipo F1 passariam.
- Arquivos alterados: `internal/platform/whatsapp/signature/compose_test.go` — novo subtest `TestComposeWithStatus_CallbackCountsPerScenario` com 4 cenários (valid, rotated, invalid, missing header).
- Teste de regressao: o próprio subtest acima. Asserts: `onStatus` chamado uma vez com label correspondente (`valid`/`rotated`/`invalid`); `onInvalid` chamado 0 vezes em valid/rotated e 1 vez em invalid/missing-header.
- Validacao: `go test -count=1 -v -run "TestComposeSuite/TestComposeWithStatus_CallbackCountsPerScenario" ./internal/platform/whatsapp/signature/` PASS (4 sub-cenarios).

## Comandos Executados

- `go build ./internal/onboarding/... ./internal/platform/whatsapp/...` -> OK (sem output)
- `go vet ./internal/onboarding/... ./internal/platform/whatsapp/...` -> OK (sem output)
- `go test -count=1 ./internal/onboarding/... ./internal/platform/whatsapp/signature/... ./internal/platform/whatsapp/dedup/... ./internal/platform/whatsapp/payload/...` -> PASS
- `go test -count=1 -v -run "TestComposeSuite/TestComposeWithStatus_CallbackCountsPerScenario" ./internal/platform/whatsapp/signature/` -> PASS em 4 sub-cenarios
- `grep -rn "MetaMessageRepository\b" --include="*.go"` -> 0 referências de símbolo (apenas comentário textual em `dedup/postgres/repository.go`)
- `grep -rn "appinterfacesmocks.NewMetaMessageRepository"` -> vazio
- `task mocks` -> regenerou `RepositoryFactory` (sem `MetaMessageRepository`) e gerou `dedup/mocks/message_repository.go`
- `go test ./...` global -> falhas pré-existentes em `cmd/worker` (assinatura `identity.NewIdentityModule` mudou no WIP de `prd-auth-foundation`) e em `internal/platform/whatsapp/dispatcher` (untracked WIP do mesmo PRD) — ambos fora deste escopo e não introduzidos por este bugfix.

## Riscos Residuais

- `cmd/worker/worker.go` chama `identity.NewIdentityModule(r.cfg, r.o11y, r.dbManager, nil)` mas o módulo identity em HEAD aceita apenas 3 args; falha de build pré-existente WIP de `prd-auth-foundation`. Fora do escopo dos 5 achados — quando aquele PRD for fechado, esta falha deve sumir junto.
- `internal/platform/whatsapp/dispatcher` (untracked) com 1 teste falhando — também pré-existente do mesmo PRD paralelo.
- 4 lint issues pré-existentes em arquivos não tocados (`payload/types.go` goimports; campos `unused` em `rate_limit_test.go` e `token_expiration_job_test.go`).
- A constante `signature.maxMetaBodyBytes` (256 KiB) e validações de body too large continuam preservadas em `signature.RawBody`. Cobertura de teste para o limite específico (413 Request Entity Too Large) não foi adicionada nesta rodada e pode ser feita como follow-up — não bloqueante.
- A evolução simultânea do working tree por outra fonte (provavelmente WIP paralelo de `prd-auth-foundation`) reverteu, durante esta sessão, várias edições intermediárias da skill bugfix. O estado final convergiu corretamente, mas o histórico de pequenas reescritas/restaurações está preservado no relatório por transparência.
