# Runbook — Validação empírica do webhook Kiwify em sandbox

> **Status:** Pronto para execução
> **Owner:** PO (jailton)
> **Tempo estimado:** 30–45 min
> **Pré-requisitos:** Acesso ao painel Kiwify (produto sandbox), conta `webhook.site`, `go` instalado localmente
> **Output esperado:** relatório em `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md` com ADRs confirmadas/substituídas

## Por que executar

Duas decisões aceitas como **suposição material** precisam de evidência empírica antes do go-live:

1. **ADR-002 (`hmac-sha256-webhook-auth`)** — algoritmo, header exato, encoding e janela de timestamp da assinatura do webhook Kiwify Public API. Se errar, **100% dos webhooks reais são rejeitados em produção**.
2. **ADR-004 (`adopt-tracking-sck-as-magic-token-carrier`)** — confirmar que `tracking.sck` chega no payload do webhook produto (e não apenas na Public API `GET /v1/sales/{id}`). Se não chegar, o magic token nunca volta para E3 e o onboarding fica órfão.

Este runbook coleta evidência crua suficiente para confirmar/substituir ambas as ADRs em uma única passagem de sandbox.

---

## Passo 1 — Configurar destino de captura

Use a URL `webhook.site` já provisionada:

- **Painel:** https://webhook.site/#!/view/6353c2b7-998a-4b56-94b9-9f2390a91f72
- **URL de entrega (para a Kiwify):** `https://webhook.site/6353c2b7-998a-4b56-94b9-9f2390a91f72`

> **Atenção:** não compartilhe esta URL fora do time. Ela aceita qualquer POST e pode receber PII (e-mail, telefone) durante o teste.

---

## Passo 2 — Configurar webhook na Kiwify (sandbox)

1. Painel Kiwify → produto sandbox → **Apps & Integrações** → **Webhooks** (ou **Configurações de Webhook**).
2. Criar/editar webhook:
   - **URL:** `https://webhook.site/6353c2b7-998a-4b56-94b9-9f2390a91f72`
   - **Token (secret):** anotar o valor exato gerado/usado — será necessário no Passo 5. Chamar de `KIWIFY_SANDBOX_SECRET`.
   - **Eventos:** marcar pelo menos `Compra Aprovada` (alvo principal); opcional: `Pix Gerado`, `Boleto Gerado`, `Reembolso`, `Chargeback`, `Carrinho Abandonado`, `Assinatura: Renovada`, `Assinatura: Atrasada`, `Assinatura: Cancelada` (para mapeamento futuro).
3. Salvar.

---

## Passo 3 — Disparar uma venda de teste com `?sck={token}`

A. **Gerar um token de teste** (qualquer UUID v4 serve):
```
TOKEN=$(uuidgen | tr 'A-Z' 'a-z')
echo $TOKEN
```

B. **Construir URL de checkout sandbox com `?sck=`**. Use o link público do produto sandbox e anexe:
```
https://pay.kiwify.com.br/<id-do-produto-sandbox>?sck=$TOKEN
```

C. **Fazer uma compra de teste** (cartão de teste da Kiwify, ver doc oficial; ou pedido R$ 1 real se sandbox não estiver disponível). Preencher campos com dados fictícios — *evite PII real*.

D. **Aguardar o webhook de `order_approved`** aparecer no `webhook.site` (até ~30s).

---

## Passo 4 — Coletar evidência crua

No painel do `webhook.site`, abrir a requisição recebida e copiar **literalmente**:

### 4.1 Headers (toda a sequência)

Copie a tabela `Request Headers` inteira. Em particular, anote os valores destes campos:

- [ ] `Content-Type` (esperado: `application/json`)
- [ ] **Nome exato do header de assinatura** — variações possíveis: `X-Kiwify-Signature`, `X-Signature`, `Signature`, `X-Hub-Signature`, `X-Kiwify-Webhook-Signature`. **Anote o que aparecer.**
- [ ] Header de timestamp (se existir): `X-Kiwify-Timestamp`, `X-Timestamp`, `X-Kiwify-Sent-At`
- [ ] `User-Agent`
- [ ] Quaisquer outros `X-Kiwify-*` ou `X-*` específicos

### 4.2 Query string

- [ ] A URL recebida contém `?signature=...`? Em caso afirmativo, copie o valor.
- [ ] Outros parâmetros customizados na query?

### 4.3 Body cru

Salvar **exatamente como recebido** (sem reformatar JSON, sem normalizar) em arquivo local:

```bash
mkdir -p /tmp/kiwify-evidence
# colar o body cru e fechar
pbpaste > /tmp/kiwify-evidence/order_approved.json   # macOS
# ou: xclip -selection clipboard -o > /tmp/kiwify-evidence/order_approved.json   # Linux
```

> **Crítico:** o body precisa ser byte-a-byte idêntico ao recebido. Whitespace e ordem de campos importam para a verificação HMAC.

### 4.4 Verificações no body

Abra o JSON em um visualizador e marque:

- [ ] Existe campo `tracking` no top-level?
- [ ] Dentro de `tracking`, qual destes campos está preenchido com o valor de `$TOKEN`?
  - [ ] `sck`
  - [ ] `s1`
  - [ ] `src`
  - [ ] `utm_source` / `utm_campaign` / `utm_content` / `utm_term` / `utm_medium`
  - [ ] outro: ______
- [ ] Existe campo `webhook_event_type` / `event` / `event_type` / `trigger`? Qual o valor?
- [ ] Existe campo `webhook_event_id` / `event_id` / `id` no envelope? Qual o formato (UUID v4? número sequencial?)?

---

## Passo 5 — Validar HMAC localmente

Protocolo Kiwify confirmado empiricamente em 2026-06-08 (ver `.specs/prd-billing-pipeline/adr-002b-hmac-sha1-hex-webhook-query-signature.md`):

- **Algoritmo:** HMAC-SHA1
- **Encoding:** hexadecimal lowercase (40 chars)
- **Veículo:** query string `?signature=<sig>` (nenhum header `X-Kiwify-*` é enviado)
- **Secret:** Token do webhook no painel Kiwify
- **Payload:** raw body, sem timestamp ou prefixo

A. **Recalcular a assinatura com openssl e comparar** com o valor da query string:

```bash
EXPECTED=$(openssl dgst -sha1 -hmac "$KIWIFY_SANDBOX_SECRET" -hex \
  /tmp/kiwify-evidence/order_approved.json | awk '{print $2}')
echo "expected=$EXPECTED"
echo "received=<valor-da-query-signature-no-webhook-site>"
[ "$EXPECTED" = "<valor-recebido>" ] && echo "MATCH" || echo "MISMATCH"
```

B. **Validar com o test de regressão do middleware** — qualquer captura nova pode ser plugada como vetor em `internal/billing/infrastructure/http/server/middleware/hmac_signature_test.go::TestHMACSignature_RealKiwifyVector`. Se o test falhar com a captura nova, o protocolo Kiwify mudou.

C. **Verificar o carrier `sck`** no body: `jq '.TrackingParameters.sck' /tmp/kiwify-evidence/order_approved.json` deve retornar o `$TOKEN` enviado no checkout.

---

## Passo 6 — Critérios de aprovação

Para considerar a validação **aprovada para go-live**, **todos** os itens abaixo devem ser verdadeiros:

- [ ] Webhook chegou no `webhook.site` em ≤ 60s do clique "Confirmar Compra".
- [ ] Body é JSON válido com top-level `TrackingParameters` contendo o token de teste em `sck`.
- [ ] `EXPECTED` (recalculado com openssl/SHA-1/hex) é igual ao valor da query `?signature=`.
- [ ] Trigger `webhook_event_type` está em `{order_approved, subscription_renewed, subscription_late, subscription_canceled, order_refunded, chargeback}`.
- [ ] `TestHMACSignature_RealKiwifyVector` do middleware continua passando.

---

## Passo 7 — Registrar resultado

Preencher o relatório em `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md` com:

1. Header de assinatura exato (nome + valor mascarado: primeiros/últimos 4 chars).
2. Encoding/protocolo HMAC vencedor.
3. Carrier vencedor.
4. Trigger/evento exato no envelope (`order_approved`, `subscription_canceled`, etc.).
5. Decisão por ADR (Confirmada / Substituída / Bloqueada).
6. Próximos passos imediatos.

**Não** versionar `/tmp/kiwify-evidence/*.json` no git — contém PII de teste e header de assinatura.

---

## Passo 8 — Limpeza

- [ ] Apagar a configuração de webhook sandbox da Kiwify (ou trocar URL de volta para a definitiva).
- [ ] Apagar arquivos em `/tmp/kiwify-evidence/`.
- [ ] Rotacionar `KIWIFY_SANDBOX_SECRET` se ele foi compartilhado em qualquer canal.

---

## Riscos durante a execução

| Risco | Mitigação |
|---|---|
| Compra de teste cobrada em cartão real | Usar cartão de teste oficial Kiwify (se sandbox suportar); reembolsar imediatamente em produção; nunca usar cartão pessoal sem combinação prévia. |
| `webhook.site` armazena PII | Usar apenas dados fictícios; deletar a sessão após teste; URL fica pública. |
| Token sandbox vazado | Rotacionar imediatamente; nunca usar o mesmo secret de produção em sandbox. |
| Webhook não chega em 60s | Verificar firewall/redirect; confirmar URL no painel; tentar `Reenviar` no histórico de webhooks da Kiwify. |
