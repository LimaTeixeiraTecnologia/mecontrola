# Tarefa 3.0: B7 — Rate limit no webhook WhatsApp

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Plug o middleware de rate-limit existente (`internal/onboarding/.../middleware/rate_limit.go`) na chain do router WhatsApp (`composeWhatsAppWebhookRouter` em `cmd/server/server.go`), parametrizado por novos envs. Mitiga DoS contra validação HMAC (CPU-bound).

<requirements>
- RF-21: reusar middleware existente, posicionar primeiro na chain do router WhatsApp
- RF-22: envs `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` (default 600), `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST` (default 100)
- RF-23: integration test cobrindo 429 antes do burst esgotar; reset após janela
- RF-24: documentação opcional sobre whitelist de IPs Meta
- RF-32–34: skills, gates, sem nova dep
- Zero comentário em `.go`
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar campos `WhatsAppWebhookRateLimitPerMin int` e `WhatsAppWebhookRateLimitBurst int` em config (`configs/config.go` na struct apropriada).
- [ ] 3.2 Defaults via `cfg.SetDefault` ou similar pattern do projeto: 600/100.
- [ ] 3.3 Em `cmd/server/server.go` `composeWhatsAppWebhookRouter()`, injetar middleware de rate-limit ANTES do raw body buffer e HMAC validation.
- [ ] 3.4 Métrica `whatsapp_webhook_rate_limit_exceeded_total` incrementada quando 429 retorna.
- [ ] 3.5 Integration test: `httptest` + 100 requests acima do burst → 429.
- [ ] 3.6 Documentar em `docs/runbooks/whatsapp-rate-limit.md` (incluir seção "whitelist Meta IPs" como opcional pós-go-live).

## Detalhes de Implementação

Ver techspec seção "Fluxo de Dados Relevante > B7" e plano-fonte §5 B7. Reusa middleware existente — **sem reimplementar**. Se reuso exigir generalização do extractor, coordenar com tarefa 4.0 (A10) para evitar conflito de PR.

## Critérios de Sucesso

- `go test -tags=integration ./internal/platform/whatsapp/... -run "RateLimit" -v` PASS.
- Smoke local com `hey -n 1000 -c 50 http://localhost:<port>/api/v1/whatsapp/inbound` → 429 antes de 1000 completar.
- `task lint && task test && task vulncheck` PASS.
- Métrica visível em `/metrics`.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration test com httptest cobrindo burst + 429
- [ ] Smoke `hey` local
- [ ] Métrica incrementa em cenário 429

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go` (modificado)
- `cmd/server/server.go` (modificado — `composeWhatsAppWebhookRouter`)
- `docs/runbooks/whatsapp-rate-limit.md` (novo)
