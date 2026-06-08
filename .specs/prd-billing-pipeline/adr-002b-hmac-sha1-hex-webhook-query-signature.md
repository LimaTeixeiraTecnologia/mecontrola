# ADR-002b — Autenticação de webhook Kiwify via HMAC-SHA1 hex em query string (confirmada empiricamente)

## Metadados

- **Título:** HMAC-SHA1 em hex sobre raw_body, secret = Token do painel, assinatura em `?signature=` da URL
- **Data:** 2026-06-08
- **Status:** **Implementada** em 2026-06-08 (commit pendente). Middleware `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` ajustado para HMAC-SHA1 + hex + query primária; teste de regressão `TestHMACSignature_RealKiwifyVector` âncora o vetor real capturado em sandbox. Envelope parser, trigger map e fixtures de teste alinhados ao payload real. Ver `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md` para detalhamento.
- **Substitui:** [ADR-002](./adr-002-hmac-sha256-webhook-auth.md)
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §8.1; evidência em `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md`; harness em `scripts/validate-kiwify-webhook/main.go`

## Contexto

A ADR-002 anterior assumiu, sem evidência, que a Kiwify usaria HMAC-SHA256 com encoding base64 via header `X-Kiwify-Signature`. Captura empírica em sandbox (`webhook.site`) em 2026-06-08 demonstrou que a realidade é diferente:

- **Algoritmo:** HMAC-SHA1 (não SHA-256).
- **Encoding:** hexadecimal lowercase (40 caracteres).
- **Veículo:** query string `?signature=...` da URL (nenhum header `X-Kiwify-*` é enviado).
- **Secret:** o **Token** cadastrado no recurso de webhook no painel da Kiwify (campo visível em "Editar Webhook").
- **Payload assinado:** `raw_body` do POST, **sem timestamp** ou concatenações.

Evidência crua (todos os 7 protocolos testados pelo harness, com body byte-exact de 2685 bytes):

```
winning_protocol: "hmac-sha1-hex(raw_body)"
signature exemplar: e8c9bfc3080b49d11d026058171c9061bc5cde95   (40 chars hex)
URL recebida:       https://<destino>/...?signature=<sig>
user-agent:         axios/1.8.4
content-type:       application/json
content-length:     2685
```

## Decisão

Substituir o middleware `HMACSignature` em `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` para:

1. **Algoritmo:** `crypto/sha1` em vez de `crypto/sha256`.
2. **Encoding:** `encoding/hex` em vez de `encoding/base64`.
3. **Veículo:** ler **prioritariamente** de `r.URL.Query().Get("signature")`. Manter header `X-Kiwify-Signature` apenas como fallback secundário (defesa em profundidade caso a Kiwify mude no futuro).
4. **Comparação:** continuar `hmac.Equal` em tempo constante.
5. **Rotação:** manter o suporte a `KIWIFY_WEBHOOK_SECRET` + `KIWIFY_WEBHOOK_SECRET_NEXT`; tentar ambos antes de marcar inválida.
6. **Métricas:** continuar planejada `billing_webhooks_received_total{signature_status}` (gap conhecido).

## Trade-offs e riscos aceitos

| Item | Decisão |
|---|---|
| **SHA-1 é colidível** | Aceito. Kiwify decidiu o protocolo; não temos controle. Para forjar uma colisão útil aqui, o atacante precisaria conhecer o secret — e com secret conhecido, qualquer hash já é trivial. SHA-1 + secret compartilhado **continua sendo barreira efetiva contra forjamento por terceiros sem o secret**. |
| **Query string em logs/proxy** | Aceito como risco médio. Mitigação: configurar gateway/proxy a redatar `?signature=` em access logs; manter o secret nunca commitado. Não bloqueia MVP. |
| **Não há header `X-Kiwify-Timestamp`** | Aceito. Sem janela de timestamp, defesa contra replay depende exclusivamente da idempotência por `event_key` (ADR-005 — `order_id` é candidato natural). Replay vira no-op no use case. |
| **Mudança unilateral pela Kiwify** | Risco operacional permanente. Mitigação: métrica de `signature_status=invalid` precisa ser exposta e alertar; já é gap conhecido (gap de telemetria #1). |

## Plano de implementação

1. **Branch dedicada:** `bugfix/kiwify-webhook-real-protocol`.
2. **Middleware** `hmac_signature.go` — trocar SHA-256/base64 por SHA-1/hex; trocar prioridade header→query para query→header.
3. **Test unit** com vetor de teste real (body do `docs/runs/.../evidencia-2026-06-08.json` se não-PII; ou fixture sintética com mesmo formato).
4. **Test de integração** simulando POST com `?signature=` na URL.
5. **Métrica** `billing_webhooks_received_total{signature_status}` exposta.
6. **Documentação:** atualizar `internal/billing/README.md` (se existir) e `techspec.md` §8.1.

## Monitoramento

- `billing_webhooks_received_total{signature_status="invalid"}` — alerta se > 0.1/s por 5min.
- Log estruturado de rejeição com `request_id` + URL (com `signature` redatada).
- Painel: distribuição de `signature_status` ao longo do tempo.

## Revisão futura

- Reabrir se a Kiwify migrar para SHA-256 ou Ed25519 (acompanhar release notes do painel).
- Reabrir se incidentes de assinatura inválida em massa ocorrerem pós-deploy.
- Reabrir se a Kiwify introduzir `X-Kiwify-Timestamp` (permitiria janela contra replay sem depender de idempotência downstream).
