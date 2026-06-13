# ADR-002 — Rotação de secret current/next via env duplicada

## Metadados

- **Título:** Rotação manual de `IDENTITY_GATEWAY_SHARED_SECRET` via dupla `_CURRENT` + `_NEXT`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-05, RF-06; [techspec](techspec.md) seção Config; precedente em `internal/platform/whatsapp/signature/hmac.go` e `internal/billing/.../middleware/hmac_signature.go`

## Contexto

O secret compartilhado entre LLM e API precisa de processo de rotação sem downtime. Existem dois precedentes no repo:

- WhatsApp HMAC (Meta) — env duplicada `META_APP_SECRET_CURRENT` + `META_APP_SECRET_NEXT`, ambos aceitos.
- Kiwify HMAC — mesmo padrão.

O time é operador solo em VPS Hostinger. Sem KMS, sem Vault, sem SSM.

## Decisão

Manter o padrão estabelecido:

- `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` (obrigatório em production, ≥ 32 bytes hex).
- `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` (opcional; vazio = sem rotação ativa).

App aceita HMAC válido com **qualquer** dos dois secrets:
- Match com `CURRENT` → `GatewayAuthResult.Valid` + métrica `result="valid"`.
- Match com `NEXT` → `GatewayAuthResult.Rotated` + métrica `result="rotated"`.

Procedimento de rotação:

1. Operador gera novo secret: `openssl rand -hex 32`.
2. Provisiona em `IDENTITY_GATEWAY_SHARED_SECRET_NEXT` no `.env` do host.
3. Reload do container Go (graceful, com healthcheck).
4. Atualiza cliente LLM para passar a usar o novo secret.
5. Monitora métrica `result="rotated"` subir.
6. Quando `result="valid"` cair para 0 (todo tráfego usando `NEXT`), promove: copia `NEXT` para `CURRENT`, esvazia `NEXT`, reload.

## Alternativas Consideradas

1. **KMS (AWS/GCP) com derivação de chave por kid** — entrega rotação automática + audit. **Rejeitada**: extrapola escopo MVP de VPS solo; adiciona dependência externa paga e latência de rede no hot path da auth.
2. **Vault local (HashiCorp)** — secret rotation policy nativo. **Rejeitada**: adiciona um daemon a operar; sem ganho para operador solo.
3. **Secret único sem rotação** — boot reinicia para trocar. **Rejeitada**: causa downtime forçado a cada rotação; é exatamente o problema que `current/next` resolve.

## Consequências

### Benefícios Esperados

- Consistência operacional: mesmo procedimento que WhatsApp e Kiwify (operador já sabe executar).
- Zero downtime na rotação.
- Visibilidade via métrica `result="rotated"`.

### Trade-offs e Custos

- Rotação manual: depende de operador disciplinado seguindo runbook.
- Sem audit log automático de "quem trocou e quando" — fica registrado apenas no commit do `.env` (ou na ausência dele, perdido).

### Riscos e Mitigações

- **R-01**: operador esquece de promover `NEXT` para `CURRENT`. **Mitigação**: métrica `result="rotated"` constante > 7 dias dispara alerta de "rotação pendente".
- **R-02**: operador troca `CURRENT` sem provisionar `NEXT` antes — downtime imediato. **Mitigação**: runbook explícito + checklist obrigatório.

## Plano de Implementação

1. `configs/config.go` carrega `Current` e `Next` (`[]byte`) com validação em `production`.
2. `services.SecretPair{Current, Next}` consumido pelo workflow.
3. Runbook `docs/runbooks/gateway-auth-rotation.md` com checklist passo a passo.

## Monitoramento e Validação

- Métrica `identity_gateway_auth_total{result="rotated"}` visível no dashboard "Auth Module".
- Validação: simular rotação em staging antes do go-live (smoke test mensal).

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth-rotation.md` (novo).
- `.env.example`: documentar ambas as envs com nota "rotação dupla".

## Revisão Futura

Revisar quando:
- Equipe crescer além do operador solo (provavelmente migrar para Vault).
- Incidente real envolvendo vazamento de secret — avaliar se rotação manual é rápida o suficiente.
- Data sugerida: 2027-06-12.
