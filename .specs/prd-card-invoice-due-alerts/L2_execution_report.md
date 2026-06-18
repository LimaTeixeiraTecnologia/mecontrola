# Generated: 2026-06-18T00:00:00Z

## Task
L2 — Job Handler Integration Test

## Status
done

## Arquivo Criado
`internal/card/infrastructure/jobs/handlers/invoice_due_alerts_job_integration_test.go`

## Critérios de Aceite

- [x] Build tag `//go:build integration` na linha 1 -> comprovado: primeira linha do arquivo
- [x] Package `handlers_test` -> comprovado: package declarado na linha 3
- [x] `TestInvoiceDueAlertsJob_Run_DispatchesEventForCardsDueWithinWindow` implementado -> comprovado: testa cartão com due_day dentro da janela, assert COUNT=1 em card_invoice_alerts_sent e outbox_events
- [x] `TestInvoiceDueAlertsJob_Run_IsIdempotent` implementado -> comprovado: chama Run duas vezes, assert COUNT=1 em card_invoice_alerts_sent
- [x] `TestInvoiceDueAlertsJob_Run_NoCandidates_NoError` implementado -> comprovado: cartão com due_day fora da janela, assert COUNT=0
- [x] Helpers `countAlertsSent`, `countOutboxByCard`, `seedUser`, `seedCardWithDueDay` implementados
- [x] Wiring via `card.NewCardModule` com DB real, cfg, noop observability, stubs de gateway/resolver
- [x] Zero comentários em código Go (exceto build tag) -> comprovado: gate `grep` retornou vazio
- [x] Sem SQL direto em adapter -> comprovado: gate `grep` retornou vazio
- [x] Sem `var _ Interface = (*Type)(nil)` -> comprovado: `grep` retornou vazio

## Comandos Executados

```
go build ./internal/card/...
# exit 0 — sem erros

go vet -tags integration ./internal/card/infrastructure/jobs/handlers/...
# exit 0 — sem erros

grep zero-comments gate -> PASS
grep adapter SQL gate -> PASS
grep banned interface assertion -> PASS
```

## Decisões Técnicas

- Usou `pickJobDueDays` (cópia local de `pickDueDays` do repositório) para determinar `inWindow` e `outWindow` de forma determinística a partir de `time.Now().UTC()`
- `buildTestModule` centraliza o wiring do módulo, evitando repetição nos três testes
- `passthrough` como função nomeada de nível de pacote (não closure) para clareza
- Stubs `stubChannelGateway` e `stubUserChannelResolver` implementam as interfaces mínimas necessárias
- `t.Skip` no terceiro teste quando `outDay == 0` (janela ocupa todos os dias do mês perto de fim de mês)

## Riscos Residuais

- Nenhum crítico identificado
- O teste de idempotência depende de ON CONFLICT DO NOTHING no repositório (comportamento já testado em `invoice_due_phase5_integration_test.go`)
