# ADR-001 — Adotar Kiwify Public API (e não Banking API) para webhooks de assinatura

## Metadados

- **Título:** Adotar Kiwify Public API para o billing-pipeline
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/prd.md`, `.specs/prd-billing-pipeline/techspec.md`, prompt enriquecido em `docs/prompts/next-create-technical-specification-billing-pipeline.md`

## Contexto

O prompt enriquecido (e portanto a expectativa inicial) listava como fonte oficial obrigatória `https://docs.kiwify.com.br/api-reference/banking/webhooks` (e páginas adjacentes), incluindo o protocolo Ed25519 prehashed e o endpoint `GET /v1/webhooks-keys`. Auditoria nas docs oficiais Kiwify revelou que o domínio possui **duas APIs distintas com escopo completamente diferente**:

- **Banking API** (`conta-public-api.kiwify.com`, `/api-reference/banking/*`): exclusivamente operações da conta digital (PIX/Boleto/QR Code). Usa Ed25519 prehashed + headers `x-kiwify-digital-signature` e `x-kiwify-timestamp`, chave pública via `GET /v1/webhooks-keys`. **Não emite eventos de assinatura recorrente.**
- **Public API** (`public-api.kiwify.com/v1`, `/api-reference/{webhooks,sales,auth,...}`): cobre vendas e assinaturas recorrentes (`compra_aprovada`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `compra_reembolsada`, `chargeback`). Usa OAuth2 + `x-kiwify-account-id`.

O PRD E2 lida exclusivamente com ciclo de vida de assinatura (Mensal/Trimestral/Anual). Adotar a Banking API tornaria o desenho irrelevante — nenhum dos triggers necessários existe lá.

## Decisão

A techspec adota **apenas a Public API** como fonte de eventos e como API de reconciliação. A Banking API é referenciada apenas como inspiração para padrões (dedupe por `recurso_id + type`).

Escopo:

- Inbound: endpoint `POST /api/v1/billing/webhooks/kiwify` configurado na Kiwify Public API com os 6 triggers do MVP.
- Outbound: client OAuth em `internal/billing/infrastructure/http/client/kiwify` chamando `POST /v1/oauth/token`, `GET /v1/sales`, `GET /v1/sales/{id}`.
- Banking API: **fora de escopo** do MVP e dos épicos seguintes deste PRD.

## Alternativas Consideradas

1. **Seguir literalmente o prompt (Banking API).** Recusada — não existe `subscription_*`, `compra_*` ou `chargeback` na Banking API; o desenho ficaria desconectado do PRD.
2. **Híbrido Public + Banking.** Recusada — Banking só faria sentido para gestão da conta digital, fora do escopo do PRD; aumenta superfície sem benefício para o MVP.
3. **Public API apenas.** Escolhida.

## Consequências

### Benefícios Esperados

- Desenho alinhado à realidade dos eventos disponíveis.
- Menor superfície técnica (um único client, sem dependência de Ed25519/chave pública rotativa).
- Reconciliação implementável com endpoint real (`GET /v1/sales`).

### Trade-offs e Custos

- Lacuna oficial: a Public API **não publica** algoritmo de assinatura dos webhooks (vide ADR-002). Trade-off aceito ao custo de suposição material.
- Sem endpoint `GET /v1/subscriptions` na Public API documentada — reconciliação restrita à varredura por `GET /v1/sales` (ADR-006).

### Riscos e Mitigações

- **Risco:** Kiwify expandir Public API com novos campos/headers ou breaking changes não anunciados. **Mitigação:** parser tolerante a campos desconhecidos; tests-com-fixture de payloads reais; monitoração de `billing_webhooks_received_total` por trigger.

## Plano de Implementação

1. Documentar provisionamento manual do webhook na Kiwify (criar via dashboard ou `POST /v1/webhooks`).
2. Implementar client OAuth em `internal/billing/infrastructure/http/client/kiwify`.
3. Mover qualquer menção a Banking API para esta ADR (e não para a techspec operacional).

## Monitoramento e Validação

- Métrica `billing_kiwify_client_requests_total{endpoint,status}` para visibilidade de chamadas Public API.
- Alerta operacional se 5xx > 1% por 5min.

## Impacto em Documentação e Operação

- README operacional de billing (a criar) deve listar os 6 triggers configurados.
- Runbook §9.4 da techspec.

## Revisão Futura

- Reabrir se a Kiwify publicar oficialmente `GET /v1/subscriptions` ou unificar Banking + Public API.
- Reabrir se um épico futuro (E4+) exigir operações de conta digital (saque, PIX out).
