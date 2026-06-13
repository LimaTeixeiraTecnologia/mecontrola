# Tarefa 2.0: outbox.Event/EventInput + NewEvent + Pack + allowlist + métrica

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adiciona campo `AggregateUserID string` em `outbox.Event`, `outbox.EventInput` e envelope JSON. Implementa validação opcional na v1 (warning + métrica), allowlist de event types de sistema (ADR-004), e métrica `outbox_events_inserted_total{has_user_id}`.

<requirements>
- RF-04: coluna aceita NULL (compat); validação opcional v1
- RF-05: `outbox.Event` ganha `AggregateUserID string`
- RF-06: `outbox.EventInput` ganha `AggregateUserID string`
- RF-07: `outbox.Row` herda `Event` (já)
- RF-08: `NewEvent` valida UUID se presente; log warn + métrica se ausente e `!isSystemEvent`
- RF-09: `Pack` inclui `aggregate_user_id` JSON top-level com `omitempty`
- RF-10: métrica `outbox_events_inserted_total{has_user_id}` com valores `"true"`/`"false"`
- RF-15: allowlist em `system_event_allowlist.go` (vazia no MVP)
- Sentinel error `ErrInvalidAggregateUserID = errors.New("outbox: aggregate_user_id must be a valid uuid")`
- Zero comentário em `.go` exceto cabeçalho de política em `system_event_allowlist.go` (exceção justificada — ADR-004 hard rule de governance documenta política inline)
- Sem nova dep
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar campo `AggregateUserID string` em `outbox.Event` struct (`internal/platform/outbox/outbox.go`).
- [ ] 2.2 Adicionar campo `AggregateUserID string` em `outbox.EventInput` struct.
- [ ] 2.3 Definir `ErrInvalidAggregateUserID = errors.New(...)`.
- [ ] 2.4 Em `NewEvent`: após validações existentes, se `input.AggregateUserID != ""`, validar com `uuid.Parse`; erro retorna `ErrInvalidAggregateUserID`. Se vazio e `!isSystemEvent(input.Type)`, incrementar métrica com `has_user_id="false"` e log warn estruturado.
- [ ] 2.5 Criar `internal/platform/outbox/system_event_allowlist.go` com map vazio e função `isSystemEvent(eventType string) bool`. Cabeçalho explica política de adição (única exceção R-ADAPTER-001.1).
- [ ] 2.6 Atualizar `Pack` em `envelope.go` para incluir `AggregateUserID string \`json:"aggregate_user_id,omitempty"\`` no `Envelope` struct e popular em `Pack(row Row) Envelope`.
- [ ] 2.7 Métrica via `o11y.Metrics().Counter("outbox_events_inserted_total", ...)` — investigar onde registrar (pacote `outbox` recebe `o11y` no construtor de algum componente; provavelmente `NewEventStorage` ou similar; ler o pacote para definir). Se exigir injeção de o11y em `NewEvent`, refatorar mínimo possível.
- [ ] 2.8 Testes table-driven em `outbox_test.go` cobrindo: válido, inválido (não-UUID), vazio + não-sistema (warning), vazio + sistema (silent).
- [ ] 2.9 Testes em `envelope_test.go` cobrindo `Pack` com e sem campo (`omitempty` funciona).
- [ ] 2.10 Regenerar mocks `task mocks` (alguns mocks de Publisher podem ter copia da struct).

## Detalhes de Implementação

Ver techspec seção "Interfaces Chave" + ADR-001 (validação opcional) + ADR-004 (allowlist). Cabeçalho do `system_event_allowlist.go` documenta política inline — única exceção R-ADAPTER-001.1 justificada em ADR-004.

## Critérios de Sucesso

- `go test -count=1 ./internal/platform/outbox/...` PASS com novos casos.
- `go build ./...` PASS (todos os callers atuais compilam — `AggregateUserID` é campo opcional na struct).
- `Pack(Row{AggregateUserID: ""})` produz JSON sem `aggregate_user_id` (omitempty OK).
- `Pack(Row{AggregateUserID: "uuid"})` produz JSON com `aggregate_user_id`.
- `grep` R-ADAPTER-001.1 sobre `outbox.go`, `envelope.go` retorna vazio.
- `system_event_allowlist.go` tem cabeçalho de política (exceção declarada).

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Tabela `NewEvent` 4+ casos
- [ ] Tabela `Pack` 2 casos (com/sem)
- [ ] Mocks regenerados sem erro

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/outbox/outbox.go` (modificado)
- `internal/platform/outbox/envelope.go` (modificado)
- `internal/platform/outbox/system_event_allowlist.go` (novo)
- `internal/platform/outbox/outbox_test.go` (modificado)
- `internal/platform/outbox/envelope_test.go` (modificado)
- `internal/platform/outbox/mocks/` (regenerado)
