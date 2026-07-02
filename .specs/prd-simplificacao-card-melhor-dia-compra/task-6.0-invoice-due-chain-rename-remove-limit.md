# Tarefa 6.0: Cadeia `invoice_due` — remover `LimitCents`, renomear `card_name`→`card_nickname`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar toda a cadeia do evento `card.invoice_due.v1` ao novo domínio: remover `LimitCents` dos structs
do decider, do payload do producer, do input/consumer e do texto de `NotifyInvoiceDue`; renomear o campo
`CardName`/chave JSON `card_name` para `CardNickname`/`card_nickname`, alimentado por `Nickname`. Producer,
consumer e `NotifyInvoiceDueInput` mudam juntos (mesmo PR) para o evento não quebrar entre publish e consume.

<requirements>
- RF-05: remover `LimitCents` de `decide_invoice_due_alerts.go` (candidato + alerta), `evaluate_invoice_due_alerts.go`, payload do producer, input/consumer e texto renderizado.
- RF-19: renomear `card_name` → `card_nickname` no payload `card.invoice_due.v1` (producer, consumer, `NotifyInvoiceDueInput`); alimentar por `c.Nickname.String()`; notificação não cita limite.
- Zero campo semanticamente enganoso (regra HARD): não manter `card_name` carregando nickname.
</requirements>

## Subtarefas

- [ ] 6.1 Editar `domain/services/decide_invoice_due_alerts.go`: remover `LimitCents`; `CardName` → `CardNickname` nos dois structs.
- [ ] 6.2 Editar `application/usecases/evaluate_invoice_due_alerts.go`: popular `CardNickname` de `c.Nickname.String()`; remover uso de `c.LimitCents`.
- [ ] 6.3 Editar `producers/invoice_due_publisher.go`: payload sem `limit_cents`; chave `card_nickname`.
- [ ] 6.4 Editar `consumers/invoice_due_notifier.go`: payload sem `limit_cents`; chave `card_nickname`; montar `NotifyInvoiceDueInput`.
- [ ] 6.5 Editar `application/usecases/notify_invoice_due.go`: `NotifyInvoiceDueInput` sem `LimitCents`; `renderText` sem menção a limite; remover `formatBRL` se ficar sem uso.

## Detalhes de Implementação

Ver `techspec.md` §"Cadeia invoice_due" e ADR-005. Producer/consumer/notify são adapters finos
(R-ADAPTER-001): só (de)serializam e delegam ao use case. Manter `card_id`, `due_date`, `days_until`.
Sem cartões/uso em produção ⇒ renomeação sem versionar o evento.

## Critérios de Sucesso

- Payload `card.invoice_due.v1` sem `limit_cents`; com `card_nickname` (não `card_name`).
- Notificação de fatura a vencer não cita limite; texto com apelido + data + dias restantes.
- Round-trip produce→consume íntegro nos testes; nenhum `LimitCents` remanescente na cadeia.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `decide_invoice_due_alerts_test.go`, `evaluate_invoice_due_alerts_test.go`, `notify_invoice_due_test.go`, `invoice_due_notifier_test.go`.
- [ ] Testes de integração: `invoice_due_publisher_integration_test.go`, `invoice_due_notifier_integration_test.go` (novo formato de payload).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/card/domain/services/decide_invoice_due_alerts.go`, `application/usecases/{evaluate_invoice_due_alerts,notify_invoice_due}.go`
- `internal/card/infrastructure/messaging/database/producers/invoice_due_publisher.go`, `consumers/invoice_due_notifier.go`
