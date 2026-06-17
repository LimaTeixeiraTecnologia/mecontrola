# Generated: 2026-06-17T17:01:19Z

# FASE 5 — Alerta proativo de vencimento de fatura de cartao

## Resumo
Implementado pipeline completo (job -> evaluate -> outbox -> consumer -> notify -> channel)
para alerta proativo de vencimento de fatura no modulo `internal/card`, espelhando o molde
de threshold alerts do modulo `budgets`. Dedup em duas fases via ledger
`card_invoice_alerts_sent`. Trabalho deixado na working tree (sem commit).

## Arquivos criados
- internal/card/domain/services/decide_invoice_due_alerts.go (decider puro)
- internal/card/domain/services/decide_invoice_due_alerts_test.go
- internal/card/application/interfaces/user_channel_resolver.go
- internal/card/application/interfaces/invoice_due_publisher.go
- internal/card/application/usecases/evaluate_invoice_due_alerts.go
- internal/card/application/usecases/evaluate_invoice_due_alerts_test.go
- internal/card/application/usecases/notify_invoice_due.go
- internal/card/application/usecases/notify_invoice_due_test.go
- internal/card/infrastructure/identity/user_channel_resolver.go
- internal/card/infrastructure/jobs/handlers/invoice_due_alerts_job.go
- internal/card/infrastructure/messaging/database/producers/invoice_due_publisher.go
- internal/card/infrastructure/messaging/database/consumers/invoice_due_notifier.go
- internal/card/infrastructure/messaging/database/consumers/invoice_due_notifier_test.go
- internal/card/infrastructure/repositories/postgres/invoice_due_alert_sent_repository.go
- migrations/000003_create_card_invoice_alerts_sent.up.sql
- migrations/000003_create_card_invoice_alerts_sent.down.sql
- mocks gerados: interfaces/mocks/{invoice_due_alert_sent_repository,invoice_due_publisher,user_channel_resolver}.go;
  consumers/mocks/notify_invoice_due_use_case.go

## Arquivos modificados
- internal/card/application/interfaces/repository.go (FindCardsWithInvoiceDueWithin, InvoiceDueAlertSentRepository, factory method, sentinel)
- internal/card/infrastructure/repositories/factory.go (factory do ledger)
- internal/card/infrastructure/repositories/postgres/card_repository.go (FindCardsWithInvoiceDueWithin + dueDayWindow)
- internal/card/module.go (nova assinatura, job, consumer, EventHandlers)
- internal/card/module_test.go (nova assinatura + caso job wired)
- configs/config.go (CardConfig + defaults + envKeys)
- .env.example (bloco Card)
- .mockery.yml (entradas card)
- cmd/worker/worker.go (reorder cardModule pos-gateway, buildCardChannelResolver, append job)
- cmd/server/server.go (nova assinatura, passa nil/nil)

## Validacao executada
- gofmt -l internal/ configs/ cmd/ migrations/ -> CLEAN
- go build ./... -> OK
- go vet ./internal/card/... ./cmd/... ./configs/... -> OK; go vet ./... -> exit 0
- go test ./internal/card/... ./configs/... -count=1 -race -> todos ok
- Gate zero-comentarios (internal/ configs/ cmd/) -> PASS
- Gate SQL-direto-em-adapter (handlers/consumers/producers/jobs) -> PASS
- R0 init / R5 panic / R6.4 var _ Iface / R7 interface{} -> PASS

## Nova assinatura NewCardModule
func NewCardModule(ctx context.Context, cfg *configs.Config, o11y observability.Observability,
  mgr manager.Manager, gatewayAuth func(http.Handler) http.Handler,
  channelGateway notification.ChannelGateway, channelResolver interfaces.UserChannelResolver) (CardModule, error)

## Migration
000003_create_card_invoice_alerts_sent.{up,down}.sql (proximo sequencial; baseline consolidada vai ate 000002 na working tree).
Tabela PK (user_id, card_id, ref_due_date), colunas sent_at/notified_at/notify_channel, FK card ON DELETE CASCADE,
indice de scan cards_due_day_scan_idx em cards(due_day) WHERE deleted_at IS NULL.

## Idempotencia (duas fases)
Fase 1 (evaluate, dentro da tx do outbox): ListSentForDueDates pre-carrega ledger; para cada alerta nao enviado,
Publish no outbox + InsertSent (ON CONFLICT DO NOTHING). Fase 2 (notify): IsNotified -> ResolvePreferred ->
MarkNotified (UPDATE ... WHERE notified_at IS NULL, retorna updated) -> SendText. Identico ao budgets.

## Formato da mensagem
"Sua fatura do cartao <nome> vence em <N> dias (DD/MM). Limite: R$<limite>." (variacoes hoje/amanha).
Sem valor monetario de fatura: dominio card nao agrega compras; payload carrega limit_cents.

## Divergencias vs design proposto (documentadas)
1. Nao existia tabela budget_alerts_sent isolada para "espelhar" — schema consolidado em 000001_initial_baseline;
   budget_alerts_sent ja existe la. Criada card_invoice_alerts_sent nova.
2. Numeracao de migration: baseline na working tree vai so ate 000002 (000003..000008 estao em worktrees .claude);
   proximo sequencial valido = 000003.
3. Resolver: budgets usa interface propria budgets/application/interfaces.UserChannelResolver + adapter em
   budgets/infrastructure/identity. Criados equivalentes card-locais (nao reusado o de budgets, para nao
   acoplar modulos).
4. SendText assinatura real: ChannelGateway.SendText(ctx, channel, externalID, text) — confirmado.
   ResolvePreferredChannel.Execute(ctx, userID) (ResolvedChannel{Channel,ExternalID}, bool, error) — confirmado.
5. Server NAO monta gateway antes do cardModule e nao roda jobs/consumers; passa nil/nil (job so no worker),
   conforme permitido pelo design.
6. Flag enabled: job so registrado quando CARD_INVOICE_DUE_ALERTS_ENABLED=true E gateway/resolver non-nil;
   consumer registrado sempre que gateway/resolver non-nil (entregar eventos ja publicados).
7. Filtro SQL FindCardsWithInvoiceDueWithin usa due_day = ANY($days) com o conjunto de dias-do-mes calculado
   no repositorio (calendario, nao regra de billing); a data exata de vencimento e o gate de janela ficam no
   decider puro do dominio.

## Riscos residuais
- Tabela card_invoice_alerts_sent precisa ser adicionada ao baseline consolidado se o projeto reconsolidar
  migrations; hoje vive como 000003 incremental.
- Taskfile mocks:mocks busca `mockery.yml` mas o arquivo e `.mockery.yml`; mocks gerados via `mockery --config .mockery.yml`.
  (pre-existente, fora do escopo)
- Cobertura de integracao (testcontainers) do novo repo nao adicionada nesta fase; coberta por unit + gates.
