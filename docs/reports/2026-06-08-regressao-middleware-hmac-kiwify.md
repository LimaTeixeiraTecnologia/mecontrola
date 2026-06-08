# Relatório — Regressão crítica no middleware HMAC Kiwify após Passos 1 e 2

> **Data:** 2026-06-08
> **Reportado por:** AI assistant (Claude) durante sessão de implementação dos Passos 1 e 2
> **Severidade:** **CRÍTICA** — se essa versão for promovida, 0% dos webhooks Kiwify reais são aceitos em produção
> **Status:** **Não corrigida** — aguardando autorização explícita do PO antes de qualquer ação
> **Audiência:** PO (jailton), tech lead, qualquer pessoa que precise entender o estado real do working tree

---

## 1. Contexto

Esta sessão entregou dois passos do plano de hardening do webhook Kiwify, ambos com `go build`, `go vet` e `go test ./internal/billing/...` verdes ao final de cada passo:

### Passo 1 — Cobertura completa de triggers Kiwify

- Capturados **9 webhooks reais** via "Testar Webhook" do painel Kiwify (mesmo secret `9ch0bpzogu9`) + 2 webhooks de produto real.
- **6 triggers do MVP confirmados** empiricamente: `order_approved`, `subscription_renewed`, `subscription_late`, `subscription_canceled`, `order_refunded`, `chargeback`.
- **4 triggers extras descobertos** (`billet_created`, `pix_created`, `order_rejected`, `abandoned_cart`) — implementados como **no-op auditado (HTTP 202 + persistido em `billing_kiwify_events`)** em vez de 422 silencioso ou drift mascarado.
- Carrinho abandonado (payload sem `webhook_event_type`) detectado via fallback no `id` top-level + `status: "abandoned"`.
- `TestHMACSignature_RealKiwifyVectors` virou table-driven com **3 vetores reais byte-exact** (`order_approved`, `billet_created`, `pix_created`).

### Passo 2 — Telemetria inegociável (gaps do ADR-002b/ADR-004)

- Métrica `billing_webhooks_received_total{signature_status}` injetada via DI no `ProcessKiwifyWebhook`.
- Métrica `billing_kiwify_tracking_carrier_total{carrier}` (labels: `sck|s1|src|none`).
- Log `info kiwify.tracking.legacy_carrier_seen{carrier, envelope_id}` emitido quando `carrier ∈ {s1, src}`.
- Log `warn billing.webhook.signature_invalid{envelope_id, event_type}` emitido em rejeição.
- Decisão consciente: substituí `request_id` (efêmero) por `envelope_id` (durável, correlaciona com `billing_kiwify_events`).

Snapshot final do Passo 2 (entregue ao PO):
- `go build -tags integration ./...` → exit 0
- `go vet ./...` → exit 0
- 10/10 packages de `internal/billing/` verdes
- 0 comentários no código novo (regra explícita do PO)
- 0 `init()`, 0 `panic` em produção, 0 `interface{}`

---

## 2. Reversão detectada

Após o Passo 2 ter sido reportado como concluído, system-reminders subsequentes mostraram que **o arquivo `internal/billing/infrastructure/http/server/middleware/hmac_signature.go` foi modificado por um linter/IDE/hook não identificado e voltou exatamente ao estado pré-fix** — anulando a correção que decorreu da validação empírica do sandbox.

Diff esperado vs estado atual no working tree:

| Local | Estado correto (pós-fix, ADR-002b vigente) | Estado atual no working tree (reverso) |
|---|---|---|
| `import` (linha 5–7) | `crypto/sha1`, `encoding/hex` | `crypto/sha256`, `encoding/base64` |
| Leitura da assinatura (linha 28–31) | `r.URL.Query().Get("signature")` primário; header `X-Kiwify-Signature` apenas fallback | `r.Header.Get("X-Kiwify-Signature")` primário; query como fallback |
| Algoritmo HMAC (linha 60) | `sha1.New` | `sha256.New` |
| Encoding (linha 62) | `hex.EncodeToString(...)` | `base64.StdEncoding.EncodeToString(...)` |

### Trecho atual do middleware (cópia verbatim do system-reminder)

```go
package middleware

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "net/http"
)

// ...
func HMACSignature(secretCurrent, secretNext string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            raw, ok := RawBodyFromContext(r)
            if !ok {
                http.Error(w, `{"message":"raw body unavailable"}`, http.StatusInternalServerError)
                return
            }

            header := r.Header.Get("X-Kiwify-Signature")
            if header == "" {
                header = r.URL.Query().Get("signature")
            }

            status := computeSignatureStatus(raw, header, secretCurrent, secretNext)
            // ...
        })
    }
}

func matchHMAC(raw []byte, header, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(raw)
    expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(header))
}
```

### Trecho que deveria estar lá (referência ADR-002b)

```go
package middleware

import (
    "context"
    "crypto/hmac"
    "crypto/sha1"
    "encoding/hex"
    "net/http"
)

// ...
func HMACSignature(secretCurrent, secretNext string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            raw, ok := RawBodyFromContext(r)
            if !ok {
                http.Error(w, `{"message":"raw body unavailable"}`, http.StatusInternalServerError)
                return
            }

            received := r.URL.Query().Get("signature")
            if received == "" {
                received = r.Header.Get("X-Kiwify-Signature")
            }

            status := computeSignatureStatus(raw, received, secretCurrent, secretNext)
            // ...
        })
    }
}

func matchHMAC(raw []byte, received, secret string) bool {
    mac := hmac.New(sha1.New, []byte(secret))
    mac.Write(raw)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(received))
}
```

---

## 3. O que está intacto (não foi revertido)

- **`process_kiwify_webhook.go`** — foi modificado nesta sessão (envelope flat + PascalCase + 4 triggers no-op + 2 counters + 2 logs). System-reminder mais recente confirma modificação mas não mostra diff; presumido intacto.
- **`hmac_signature_test.go`** — `buildSignature` mantém `sha1 + hex`. Tests viraram **table-driven** (R4 da skill go-implementation) com 6 cenários consolidados em `TestHMACSignature` + 3 vetores reais em `TestHMACSignature_RealKiwifyVectors`. **Como `buildSignature` está em sha1/hex mas o middleware voltou para sha256/base64, todos esses tests devem falhar agora.**
- **`kiwify_webhook_handler_test.go`** — reescrito para usar mocks geradas via mockery (`ucmocks.NewProcessSaleApproved(s.T())`, etc) com `mock.EXPECT()...Return()`. Melhoria real de qualidade — aderente a R3 (`mockery.yml`) e R4 (`testify/suite`). Os tests também usam `signPayload` em sha1/hex, então mismatch idêntico vs middleware revertido.
- **Outros tests do projeto** que tinham bug do linter (`suite.Run(s.T(), ...)`) foram corrigidos para `suite.Run(t, ...)` em 10 arquivos.

---

## 4. Consequência prevista

- **`go test ./internal/billing/infrastructure/http/server/middleware/...`** deve falhar com `SignatureStatusInvalid` em **todos** os cenários onde a assinatura era válida em sha1/hex — porque o middleware agora calcula sha256/base64 e nunca bate.
- **`go test ./internal/billing/infrastructure/http/server/handlers/...`** deve falhar de forma análoga em todos os cenários 202 (cenários 401/415/413 podem continuar passando porque não dependem de assinatura válida).
- **`TestHMACSignature_RealKiwifyVectors`** com os 3 vetores reais (`order_approved`, `billet_created`, `pix_created`) — falha em todos.
- **Em produção real**: 100% dos webhooks Kiwify rejeitados com HTTP 401 (`ErrInvalidSignature`), persistidos em `billing_kiwify_events` com `signature_status=invalid`. Cliente paga, sistema audita, mas nenhum efeito downstream — nem ativação no E3, nem entitlement, nem outbox.

---

## 5. Hipóteses para a causa da reversão

Apenas hipóteses — não investigadas para não acionar nada sem autorização:

1. **Hook de pre-commit ou pre-write** com regra "código de produção deve usar SHA-256" que não conhece a evidência empírica (ADR-002 estava aceita; ADR-002b veio depois).
2. **IDE auto-revert** de mudanças por conflito com cache de gopls/staticcheck.
3. **Linter custom** (depguard, custom analyzer) que rejeita `crypto/sha1` como "weak crypto" e auto-aplica fix para `sha256`.
4. **Outra branch / outro turno paralelo** sobrescreveu o arquivo via `git checkout` ou rebase.
5. **Lint Go padrão (`govet`/`staticcheck`)** **não** faria isso sozinho — então não é causa default.

---

## 6. Por que NÃO foi corrigida automaticamente

O PO instruiu explicitamente nesta sessão:
> "Quando terminar realmente, sem falso positivo, me diga os próximos passos, não comece nada sem me perguntar se pode!"

Tocar o middleware agora violaria essa diretriz. O relatório é a forma autorizada de comunicação até autorização explícita do PO.

---

## 7. Opções de ação (todas requerem autorização explícita)

### Opção A — Confirmar a regressão com diagnóstico
- Rodar `go test ./internal/billing/... -count=1 -run '^TestHMAC|^TestKiwifyWebhookHandler|^TestHMACSignature_RealKiwifyVectors'`.
- Sem mudança em código. Read-only de fato. Tempo: < 1 min.

### Opção B — Reaplicar a fix do middleware
- Reescrever `hmac_signature.go` para o estado correto (sha1 + hex + query primária).
- Tempo: < 5 min. Risco: nenhum se a Opção C não for executada.

### Opção C — Investigar a causa raiz
- Listar hooks ativos: `cat .git/hooks/* .pre-commit-config.yaml`.
- Verificar `.claude/settings.json`, `Taskfile.yml` e `golangci.yml` por regras automáticas contra `crypto/sha1`.
- Verificar `git reflog` para identificar quem/quando reverteu o arquivo.
- Tempo: 10–20 min. **Sem isso, qualquer fix da Opção B pode ser revertida de novo.**

### Opção D — Combinar B + C + commit
- Aplicar fix do middleware (B) + investigar e neutralizar o gatilho (C) + commit com mensagem explicativa para travar o estado.
- Recomendada se PO quer deixar o working tree em estado verde e travado antes de fim de sessão.

### Opção E — Outras
- Aguardar e investigar manualmente em outro momento.
- Reverter os Passos 1 e 2 inteiros e refazer do zero em branch limpa.

---

## 8. O que NÃO está em risco

- **PRDs, techspecs, ADRs e relatórios em `docs/runs/`** — são markdown puro, não foram tocados pela reversão.
- **`process_kiwify_webhook.go`** — provavelmente intacto (não foi mostrado revertido). Estrutura nova de envelope + triggers + telemetria deve estar lá.
- **Trigger map (envelope parser)** — não depende do middleware HMAC.
- **`billing_kiwify_events` persistência** — funciona independentemente de assinatura ser válida.

A regressão é **localizada ao middleware HMAC**, mas como ele é o portão de entrada do webhook, basta ele estar errado para tornar o pipeline inteiro inútil em produção.

---

## 9. Referências cruzadas

- ADR vigente: `.specs/prd-billing-pipeline/adr-002b-hmac-sha1-hex-webhook-query-signature.md` (status: Implementada — atualmente em **divergência** com o working tree).
- ADR substituída: `.specs/prd-billing-pipeline/adr-002-hmac-sha256-webhook-auth.md` (status: SUBSTITUÍDA).
- Relatório de validação empírica: `docs/runs/2026-06-08-validacao-webhook-kiwify-sandbox.md`.
- Runbook do sandbox: `docs/runbooks/validacao-webhook-kiwify-sandbox.md`.
- Vetor real anchor: `internal/billing/infrastructure/http/server/middleware/hmac_signature_test.go::TestHMACSignature_RealKiwifyVectors` (3 vetores byte-exact com sigs capturadas).

---

## 10. Resumo executivo para decisão do PO

| Item | Estado |
|---|---|
| Passo 1 (4 novos triggers) | Código entregue, tests escritos, ADR-002b registrada |
| Passo 2 (4 gaps de telemetria) | Código entregue (2 counters + 2 logs), ADR atualizada |
| Middleware HMAC | **REVERTIDO PARA O ESTADO BUGADO** por hook/linter desconhecido |
| Tests do middleware | Estão na versão correta (sha1+hex) — atualmente devem estar quebrados |
| Tests do handler | Migrados para mockery + suite — atualmente devem estar quebrados |
| Production-ready inegociável | **NÃO** enquanto o middleware estiver revertido |
| Próximo passo recomendado | Opção D — corrigir + investigar causa + commit |
| Próximo passo só com autorização | qualquer das opções A–E acima |

---

**FIM DO RELATÓRIO. Nenhuma ação corretiva foi tomada. Aguardando direção explícita do PO.**
