# ADR-002 — Autenticação de webhook Kiwify via HMAC-SHA256 sobre raw_body (suposição material)

## Metadados

- **Título:** HMAC-SHA256 com token compartilhado para validar webhooks Kiwify
- **Data:** 2026-06-05
- **Status:** **SUBSTITUÍDA** em 2026-06-08 pela ADR-002b (`adr-002b-hmac-sha1-hex-webhook-query-signature.md`). Evidência empírica em `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md` mostrou que a Kiwify usa **HMAC-SHA1 em hex via query string `?signature=`** — não HMAC-SHA256 base64 via header `X-Kiwify-Signature`. A suposição material desta ADR falhou. **NÃO seguir esta ADR para implementação.**
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-billing-pipeline/techspec.md` §8.1, [ADR-001](./adr-001-kiwify-public-api-vs-banking.md)

## Contexto

A Kiwify Public API permite criar webhook (`POST /v1/webhooks`) com um campo `token` no recurso. Auditoria das páginas oficiais (`/api-reference/webhooks/create`, `/api-reference/webhooks/single`, `/api-reference/webhooks/list`, `/api-reference/webhooks/edit`) **não documenta** o algoritmo de assinatura nem o header onde a assinatura chega. O Banking API (que não é usado por este PRD — ADR-001) usa Ed25519 prehashed + `x-kiwify-digital-signature`; isso **não se aplica** à Public API.

O PRD (Restrições Técnicas e RF-11) exige garantia de autenticidade do webhook — não pode aceitar payload de origem não verificada. A ausência de doc oficial cria risco material.

## Decisão

Adotar **HMAC-SHA256** sobre o `raw_body` da requisição, com o `token` cadastrado no recurso webhook como secret compartilhado, e validar contra o header `X-Kiwify-Signature` (preferencial) com fallback para query string `signature`. Comparação em tempo constante (`hmac.Equal`). Suportar rotação via `KIWIFY_WEBHOOK_SECRET` + `KIWIFY_WEBHOOK_SECRET_NEXT` (ambos aceitos durante janela).

A escolha está alinhada a (a) prática mais comum em webhooks de gateways brasileiros e (b) observações empíricas relatadas por integradores Kiwify (não consta na doc oficial, mas é o padrão recorrente).

**Marcação explícita:** esta decisão é uma **suposição material**. Antes do início da execução das tarefas derivadas, a equipe **deve** confirmar empiricamente em sandbox Kiwify:

1. Header exato em que a assinatura chega.
2. Encoding (base64 vs hex).
3. Conteúdo assinado (raw body sozinho vs raw body + timestamp vs path + body, como no Banking).
4. Eventual presença de header `X-Kiwify-Timestamp` e janela de tolerância.

Se a confirmação revelar protocolo diferente, esta ADR é substituída antes da execução.

## Alternativas Consideradas

1. **Apenas comparar `token` em header/query (sem HMAC).** Recusada — token em texto claro vaza facilmente em log/proxy/misconf; sem proteção contra replay.
2. **Ed25519 prehashed (Banking-style).** Recusada — alta probabilidade de rejeitar 100% dos webhooks reais; não há evidência de que a Public API use este protocolo.
3. **Bloquear techspec até suporte Kiwify confirmar.** Recusada — bloqueia desenho sem necessidade; HMAC-SHA256 é trivial de trocar se descobrirmos divergência.
4. **HMAC-SHA1.** Recusada — SHA-1 colidível; SHA-256 é o mínimo aceitável modernamente.

## Consequências

### Benefícios Esperados

- Proteção contra forjamento por terceiros que não possuam o `token`.
- Proteção parcial contra replay (combinado com idempotência por `event_key` em ADR-005, replay vira no-op).
- Implementação trivial (`crypto/hmac` + `crypto/sha256` da stdlib).
- Suporte a rotação sem downtime.

### Trade-offs e Custos

- **Suposição material:** se a Kiwify validar de outra forma, 100% dos webhooks reais são rejeitados em produção até trocar. Mitigado pela validação obrigatória pré-execução.
- Não cobre replay sem coordenação com timestamp (se Kiwify enviar timestamp, adicionar verificação de janela 5min — não bloqueia o MVP).
- Token compartilhado precisa estar sincronizado entre o recurso webhook na Kiwify e a aplicação.

### Riscos e Mitigações

- **R:** Algoritmo divergente do real. **M:** Validação empírica em sandbox (curl simulando webhook real); ADR substituída antes da execução.
- **R:** Token vazado em log. **M:** Logger configurado para nunca imprimir headers de auth; sigil `KIWIFY_WEBHOOK_SECRET` em `Safe()` apenas indica presença booleana.
- **R:** Rotação mal coordenada. **M:** Janela de aceitação de dois secrets (`_CURRENT` e `_NEXT`); operador troca em ordem (popular `_NEXT` → trocar na Kiwify → mover `_NEXT` para `_CURRENT` e limpar `_NEXT`).

## Plano de Implementação

1. **Pré-execução:** validar empiricamente o protocolo em sandbox Kiwify; documentar no PR de implementação.
2. Middleware `hmac_signature` em `internal/billing/infrastructure/http/server/middleware/`.
3. Middleware `raw_body_buffer` armazena body em context antes do parser.
4. Teste de unit com fixture conhecida (vector de teste documentado).
5. Teste de integração: webhook simulado com assinatura válida → 202; com assinatura inválida → 401.

## Monitoramento e Validação

- Métrica `billing_webhooks_received_total{signature_status}` (valid/invalid/rotated).
- Alerta: `rate(billing_webhooks_received_total{signature_status="invalid"}[5m]) > 0.1` por 5min → investigar (ataque ou rotação errada).
- Log: cada rejeição inclui `request_id` e razão (sem o conteúdo do body).

## Impacto em Documentação e Operação

- Runbook §9.4 inclui ação para `signature_status='invalid'` em massa.
- Procedimento operacional de rotação documentado no README do módulo billing.

## Revisão Futura

- Reabrir se a Kiwify publicar oficialmente o algoritmo de assinatura da Public API (substituir esta ADR pela implementação oficial — provavelmente sem mudança de código se for HMAC-SHA256; mudança maior se for Ed25519).
- Reabrir se houver incidente de assinatura inválida em massa pós-deploy.
