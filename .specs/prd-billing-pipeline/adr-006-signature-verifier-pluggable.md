# ADR-006 — `BillingProvider.VerifySignature` hexagonal com token → HMAC sem mudança de RF

## Metadados

- **Título:** Verificação de assinatura do webhook Kiwify
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Equipe de segurança + plataforma
- **Relacionados:** `prd-billing-pipeline/prd.md` (RF-02, RF-28, D-01), `techspec.md` §Adapter Kiwify

## Contexto

PRD D-01 decidiu implementação defensiva: documentação oficial Kiwify confirma existência de token por webhook (visível em `GET /v1/webhooks/{id}`) mas não publica o mecanismo de verificação. Convenções comunitárias (n8n, Zapier, Pluga) usam comparação de token em header — mas Kiwify pode evoluir para HMAC.

Restrição: a troca de mecanismo (token → HMAC) não pode mudar contratos public RF nem o use case `IngestKiwifyWebhookUseCase`.

## Decisão

`BillingProvider` declara método **independente de mecanismo**:

```go
VerifySignature(payload []byte, headers map[string]string) error
```

Aceita `payload []byte` (preserva raw bytes — exigência de HMAC) e `headers map[string]string` (transporte de quaisquer headers). Sem parâmetro `secret`: secret é dependência injetada no construtor do adapter Kiwify (`KiwifyConfig.WebhookSecret`).

Implementações concretas isoladas em `internal/billing/infrastructure/http/client/kiwify/`:
- `TokenSignatureVerifier` (atual MVP) — comparação `subtle.ConstantTimeCompare` de token em header configurável.
- `HMACSignatureVerifier` (futuro, sem deploy de novo RF) — `hmac.New(sha256.New, secret)` sobre `payload`, compare com header `X-Kiwify-Signature`.

Adapter Kiwify recebe `SignatureVerifier` interface no construtor (injeção). Troca de impl é uma única linha em `billing_subsystem.go`.

```go
type SignatureVerifier interface {
    Verify(payload []byte, headers map[string]string) error
}
```

## Alternativas Consideradas

### Hardcode de mecanismo no adapter Kiwify

- Vantagem: simples.
- Desvantagem: troca requer deploy + alterações em testes.
- Rejeitada por inflexibilidade.

### Configuração por env (`KIWIFY_SIGNATURE_MECHANISM=token|hmac`) com switch interno

- Vantagem: troca sem deploy.
- Desvantagem: dois caminhos sempre testados; código morto se HMAC nunca for usado.
- Rejeitada por excesso para o MVP.

## Consequências

### Benefícios Esperados

- Mecanismo seguro por default (`constant-time compare`).
- Plug de HMAC sem alterar use case, RF, ou contrato externo.
- Testabilidade: cada impl tem suite própria.

### Trade-offs e Custos

- Indireção via interface — overhead negligível (1 chamada virtual por webhook).

### Riscos e Mitigações

- **Risco:** mecanismo real ser nem token nem HMAC (e.g., JWT, OAuth Bearer). **Mitigação:** nova impl do `SignatureVerifier` cobre qualquer mecanismo HTTP padrão. Caso patológico: validação via call externo (improvável).

## Plano de Implementação

1. Criar `SignatureVerifier` interface em `infrastructure/http/client/kiwify/signature_verifier.go`.
2. Implementar `TokenSignatureVerifier` com `subtle.ConstantTimeCompare`.
3. Adapter `KiwifyAdapter.VerifySignature` delega para o verifier injetado.
4. Wire em `billing_subsystem.go`: `tokenVerifier := kiwify.NewTokenSignatureVerifier(cfg.WebhookSecret, cfg.WebhookTokenHeader)`.
5. Teste: tabela com header presente/ausente/case-insensitive/valor wrong-by-1-char.

## Monitoramento e Validação

- Métrica `billing_webhook_received_total{outcome="rejected_signature"}` cresce se assinatura falha.
- Log de auditoria (sem expor secret): `slog.WarnContext(ctx, "kiwify webhook signature failed", slog.String("header_present", strconv.FormatBool(received != "")))`.
- Alerta em taxa > 1% (sinal de ataque ou config wrong).

## Impacto em Documentação e Operação

- Runbook: ao migrar para HMAC, trocar wire em `billing_subsystem.go` e atualizar painel Kiwify para emitir header `X-Kiwify-Signature`.

## Revisão Futura

- Migrar para HMAC assim que Kiwify documentar publicamente ou após validação empírica em sandbox revelar suporte. Atualizar este ADR como `Substituída por ADR-NN`.
