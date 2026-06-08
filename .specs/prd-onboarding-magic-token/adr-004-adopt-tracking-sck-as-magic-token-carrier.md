# ADR-004 — Adoção de `tracking.sck` como carrier oficial do magic token + migração de `s1`/`src` em E2

## Metadados

- **Título:** Campo Kiwify usado para propagar o magic token entre landing → checkout → webhook
- **Data:** 2026-06-06
- **Status:** **Confirmada empiricamente** em 2026-06-08 (captura sandbox via `webhook.site`). `TrackingParameters.sck` chega intacto no payload do webhook produto da Kiwify (`webhook_event_type: order_approved`). Evidência em `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md`. **Atenção:** o campo no payload é `TrackingParameters` (PascalCase), não `tracking` (lowercase) — o struct tag do parser em `internal/billing/application/usecases/process_kiwify_webhook.go:52` precisa ser corrigido (bug descoberto pela mesma validação, ver relatório). Ordem `sck > s1 > src` continua válida.
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §"Evidências oficiais", RF-01, RF-03, S-01; `.specs/prd-billing-pipeline/techspec.md` §6.3, L-03 do E2

## Contexto

O PRD (RF-01) descreve o carrier como `?s={token}`. O nome `s` é placeholder semântico; não é um campo nativo da Kiwify. A documentação oficial confirma:

- Webhook produto (Notion oficial) — `TrackingParameters{src, sck, utm_source, utm_medium, utm_campaign, utm_content, utm_term}`. **Não existe `s`.**
- Public API `GET /v1/sales/{id}` (consumida pela reconciliação de E2) — `tracking{utm_*, sck, src, s1..s3}`. **Existem `s1..s3`** (campos auxiliares) e `sck` (subscriber/identifier customizado).

E2 hoje:
- `internal/billing/application/usecases/funnel_token.go` lê `tracking.s1` primeiro, fallback `tracking.src`.
- `s1` **não consta** na doc oficial do webhook produto (só na Public API de sales). Há risco material: a Kiwify pode não propagar `s1` no payload do webhook, e E2 vai cair em `src` (que é o campo de "fonte de tráfego" — pode ter UTM-like e não o token).

Candidatos canônicos para carrier:
1. `sck` — Subscriber Custom Key, semanticamente correto para "identificador externo do comprador no nosso sistema".
2. `src` — Source/origem; valor semântico é tracking de origem (orgânico, anúncio); usar para token mistura responsabilidades.
3. `utm_content` — sub-tracking de criativo; uso interno é tolerável mas perde semântica.
4. `s1..s3` — campos auxiliares Kiwify Public API; não confirmados no webhook produto.

## Decisão

**Adotar `sck` como carrier oficial do magic token.** Tanto a landing quanto o painel Kiwify devem usar `?sck={token}` em todos os pontos: link de checkout, URL de redirect pós-pagamento, payload do webhook (propagado nativamente em `TrackingParameters.sck`).

**Migração compatível em E2:**
1. `funnel_token.go` passa a ler **primeiro** `tracking.sck`, depois `tracking.s1`, depois `tracking.src` (ordem de prioridade).
2. Quando o token vem de `s1` ou `src` (legado), emitir log info `kiwify.tracking.legacy_carrier_seen` com label `carrier=<s1|src>`.
3. Métrica Prometheus `billing_kiwify_tracking_carrier_total{carrier}` para acompanhar a transição.
4. Janela de coexistência: 30 dias OU até 7 dias consecutivos sem nenhuma ocorrência de `s1`/`src` (o que ocorrer antes).
5. Após janela: remover suporte a `s1`/`src`; manter apenas `sck`.

**Operação:**
- Landing (`mecontrola-landingpage`) usa `POST /v1/onboarding/checkout` e recebe URL Kiwify já com `?sck={token}` apensado por `CheckoutURLBuilder` no backend Go.
- Painel Kiwify: configurar URL de redirect pós-pagamento para `https://www.mecontrola.app.br/obrigado/{tracking_sck}` (se suporta placeholders). Se não suportar: fallback descrito em techspec §6.5 (Pages Function aceita `?s={token}` na query string como retaguarda).

## Alternativas Consideradas

1. **Manter `s1`/`src` (não migrar).** Recusada — `s1` não consta no webhook produto oficial; risco de quebra silenciosa em mudanças futuras da Kiwify.
2. **Adotar `utm_content`.** Recusada — semântica errada; mistura tracking de marketing com identidade técnica; pode ser sobrescrito por UTM de campanha.
3. **Custom field oculto preenchido via query string.** Recusada — Kiwify não documenta custom fields no payload do webhook produto; risco igual ou maior que `s1`.
4. **Aceitar `sck` E `src` indefinidamente.** Recusada — código de extração permanece ambíguo; debt persiste.

## Consequências

### Benefícios
- Carrier alinhado à doc oficial (webhook produto + Public API).
- Semântica correta para "identificador do comprador no nosso sistema".
- Telemetria explícita da transição.

### Trade-offs
- Requer alteração em E2 (já em produção). Risco baixo (aditivo + ordem de prioridade).
- Requer coordenação operacional com painel Kiwify (config de URL de redirect).
- Janela de coexistência adiciona complexidade temporária.

### Riscos e Mitigações
- **R:** Painel Kiwify não suporta placeholder `{tracking_sck}` na URL de redirect. **M:** Fallback Pages Function aceita `?s={token}` via query (techspec §6.5).
- **R:** Kiwify não propaga `sck` no payload do webhook produto (apenas na Public API de sales). **M:** Janela de coexistência detecta empiricamente; se `sck` nunca aparece no webhook produto, ADR é revisada.
- **R:** Cliente clica em link antigo (cache) que ainda usa `s1`. **M:** Janela de 30d cobre cache de browser e bookmarks razoáveis.

## Plano de Implementação
1. Alterar `internal/billing/application/usecases/funnel_token.go` para ler `sck` primeiro.
2. Adicionar campo `Sck string \`json:"sck"\`` em `trackingData` em `process_kiwify_webhook.go`.
3. Emitir métrica `billing_kiwify_tracking_carrier_total{carrier}` em cada extração.
4. Atualizar `CheckoutURLBuilder` (novo, E3) para apensar `?sck={token}` em URLs Kiwify.
5. Test unitário em `funnel_token_test.go` cobrindo prioridade `sck > s1 > src`.
6. Coordenar PR no painel Kiwify (operacional).

## Monitoramento
- `billing_kiwify_tracking_carrier_total{carrier}` por 30d para decidir remoção do legado.
- Dashboard simples: distribuição de carriers ao longo do tempo.

## Revisão Futura
Após janela de coexistência, abrir issue para remover branches de `s1`/`src` em `funnel_token.go`. Atualizar este ADR para status "Substituída" se padrão de carrier mudar.
