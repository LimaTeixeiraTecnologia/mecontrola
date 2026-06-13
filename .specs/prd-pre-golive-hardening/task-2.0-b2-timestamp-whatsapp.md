# Tarefa 2.0: B2 — Timestamp WhatsApp anti-replay

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adiciona validação de timestamp da mensagem WhatsApp após dedup por wamid e antes de `EstablishPrincipal`, eliminando replay de webhooks legítimos fora da janela de 5 min. Retorna 200 OK silencioso (Meta não dispara retry) e registra `auth_events` com `reason` apropriado.

<requirements>
- RF-01: extrair `entry[].changes[].value.messages[].timestamp` do payload Meta
- RF-02: `|now - ts| > 5min` → 200 OK + `auth_events.reason="stale_webhook"`
- RF-03: timestamp ausente/inválido → 200 OK + `auth_events.reason="invalid_webhook_timestamp"`
- RF-04: `time.Now().UTC()` inline; sem `Clock` interface (regra de memória)
- RF-05: testes unitários cobrindo 5 cenários (dentro janela, +6min, -6min, ausente, inválido)
- RF-32: skill `go-implementation`
- RF-33: lint+test+vulncheck verde
- RF-34: sem nova dep
- Zero comentário em `.go` produção
- Coordenar com migration `000015` (PRD gateway-auth-forensics) para incluir os 2 novos valores no CHECK de `reason`
</requirements>

## Subtarefas

- [ ] 2.1 Localizar `inbound_handler.go` em `internal/platform/whatsapp/handlers/` e o ponto pós-dedup, pré-`EstablishPrincipal`.
- [ ] 2.2 Extrair `message.Timestamp` (string unix segundos). `strconv.ParseInt(raw, 10, 64)`. Em erro, registrar `reason="invalid_webhook_timestamp"` e retornar 200.
- [ ] 2.3 Calcular `delta := time.Now().UTC().Sub(time.Unix(ts, 0).UTC())`. Se `|delta| > 5*time.Minute`, registrar `reason="stale_webhook"` e retornar 200.
- [ ] 2.4 Atualizar CHECK constraint de `auth_events.reason` para incluir `stale_webhook` e `invalid_webhook_timestamp` — coordenar com tarefa 3.0 do PRD `prd-gateway-auth-forensics` (migration 000015) ou criar migration 000016 dedicada se aquela já tiver sido aplicada.
- [ ] 2.5 Métrica `whatsapp_stale_webhook_total{reason}` com `reason` ∈ {`stale_webhook`, `invalid_webhook_timestamp`}. Sem `user_id` em label.
- [ ] 2.6 Testes table-driven cobrindo os 5 cenários RF-05.

## Detalhes de Implementação

Ver techspec seção "Fluxo de Dados Relevante > B2" e plano-fonte seção 8.2. Skill `go-implementation` Etapa 1–5 obrigatória.

## Critérios de Sucesso

- `go test ./internal/platform/whatsapp/handlers/... -run "Timestamp" -v` PASS para 5 cenários.
- Smoke local: postar payload válido com timestamp -6min → response 200 + log `stale_webhook`.
- Métrica `whatsapp_stale_webhook_total` visível no `/metrics` interno.
- `task lint && task test && task vulncheck` PASS.
- Inspeção: 0 comentário novo em `.go`.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Tabela 5 cenários (dentro, +6min, -6min, ausente, inválido)
- [ ] Métrica incrementa em cenário stale
- [ ] Smoke local com curl + payload Meta de exemplo

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/whatsapp/handlers/inbound_handler.go` (modificado)
- `internal/platform/whatsapp/handlers/inbound_handler_test.go` (modificado)
- `migrations/000016_auth_events_stale_webhook_reasons.up.sql` (novo se 000015 já mergeado; senão coordenar)
- `migrations/000016_auth_events_stale_webhook_reasons.down.sql` (novo)
