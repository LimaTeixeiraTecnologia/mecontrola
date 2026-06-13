# ADR-007 — Tabela de rotas que pulam o gateway

## Metadados

- **Título:** Quais rotas HTTP DEVEM e quais NÃO DEVEM aplicar `RequireGatewayAuth`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-12; [techspec](techspec.md) seção Visão Geral; gate de revisão M-09

## Contexto

Aplicar `RequireGatewayAuth` cegamente em todas as rotas quebra:
- Webhooks externos (WhatsApp, Kiwify) — esses NÃO vêm da LLM e têm HMAC próprio.
- Endpoints de saúde/métricas — esses são consumidos por Caddy/Prometheus internos, não pela LLM.

Inversamente, **não** aplicar onde deveria deixa bypass aberto. A regra precisa ser explícita e mecanicamente verificável.

## Decisão

A tabela canônica de aplicação do gateway, atualizada para o estado de `main` em 2026-06-12:

| Rota | Gateway HMAC? | Mecanismo de auth atual | Motivo |
|---|---|---|---|
| `/api/v1/cards*` | **SIM** | `InjectPrincipalFromHeader` + `RequireUser` | Única rota hoje que consome o injetor |
| Futuras rotas com `InjectPrincipalFromHeader` | **SIM** | idem | Regra geral |
| `/api/v1/whatsapp/inbound` | NÃO | HMAC-SHA256 Meta próprio | Webhook externo |
| `/api/v1/whatsapp/verify` | NÃO | Verify token Meta | Webhook externo |
| `/api/v1/kiwify/*` | NÃO | HMAC-SHA1 Kiwify próprio | Webhook externo |
| `/api/v1/onboarding/*` | NÃO | Rate-limit IP + token mágico | Fluxo público de ativação |
| `/api/v1/budgets*` | (ver nota) | atualmente sem `InjectPrincipalFromHeader` | Reavaliar quando PRD for ativado |
| `/api/v1/categories*` | (ver nota) | atualmente sem `InjectPrincipalFromHeader` | idem |
| `/api/v1/transactions*` | (ver nota) | atualmente sem `InjectPrincipalFromHeader` | idem |
| `/healthz`, `/readyz` | NÃO | nenhum | Liveness/readiness probes |
| `/metrics` | NÃO (deve ser bloqueado externamente pelo Caddy) | nenhum | Prometheus interno |
| `/debug/pprof*` | NÃO (bloqueado pelo Caddy) | nenhum | Debug interno |

**Regra geral (gate M-09):**

> Toda rota que monta `InjectPrincipalFromHeader` ou `InjectPrincipalFromHeaderWithO11y` no chain DEVE ter `RequireGatewayAuth` posicionado imediatamente antes dela no mesmo bloco. Sem exceção.

**Nota sobre budgets/categories/transactions**: hoje esses routers **não** usam o injetor (verificado por `grep`). Quando adotarem (provavelmente no mesmo PR que migra para `auth.Principal` consistente), a regra geral se aplica automaticamente e o gate M-09 garante.

## Alternativas Consideradas

1. **Aplicar gateway globalmente como middleware do servidor** — todas as rotas. **Rejeitada**: quebra webhooks (Meta não envia `X-Gateway-Auth`) e healthchecks. Inviável.
2. **Aplicar gateway apenas via opt-in explícito por handler** — handler decora-se com tag. **Rejeitada**: fácil esquecer; oposto do "fail-secure".
3. **Pular gate quando `X-User-ID` ausente** — modo permissivo. **Rejeitada**: trivial bypass (atacante apenas omite o header).

## Consequências

### Benefícios Esperados

- Tabela explícita e auditável.
- Gate M-09 verifica mecanicamente.
- Adicionar uma nova rota autenticada é zero esforço cognitivo: usar `InjectPrincipalFromHeader` → gate exige `RequireGatewayAuth`.

### Trade-offs e Custos

- Manutenção da tabela: nova rota com `InjectPrincipal` precisa ser refletida aqui também (apenas se houver desvio do padrão).
- Gate M-09 é um script shell — quebra silenciosa se script tem bug. Mitigação: teste do próprio script.

### Riscos e Mitigações

- **R-01**: PR adiciona rota autenticada mas esquece de plugar `RequireGatewayAuth`. **Mitigação**: gate M-09 quebra o CI.
- **R-02**: `/metrics` é exposto externamente por erro de Caddy. **Mitigação**: item B3 do plano-fonte (Caddyfile hardening).

## Plano de Implementação

1. Script `deployment/scripts/lint-auth-bypass.sh`:
   ```bash
   #!/usr/bin/env bash
   set -euo pipefail
   violations=$(
     grep -rn --include="*.go" -E "InjectPrincipalFromHeader(WithO11y)?" \
       internal/*/infrastructure/http/server/ \
       | grep -v "_test.go" \
       | while IFS= read -r line; do
         file="${line%%:*}"
         lineno=$(echo "$line" | awk -F: '{print $2}')
         if ! awk -v ln="$lineno" 'NR < ln && NR >= ln-3 { print }' "$file" \
              | grep -q "RequireGatewayAuth"; then
           echo "$line"
         fi
       done
   )
   if [[ -n "$violations" ]]; then
     echo "FAIL: rotas com InjectPrincipalFromHeader sem RequireGatewayAuth imediatamente antes:"
     echo "$violations"
     exit 1
   fi
   ```
2. Adicionar `task lint:auth-bypass` em `Taskfile.yml`.
3. Adicionar ao job `lint` do CI.

## Monitoramento e Validação

- Gate roda no CI a cada PR.
- Falha de gate é bloqueante para merge.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: copy da tabela.
- `Taskfile.yml`: adicionar receita.
- README do módulo `internal/identity`: nota sobre a regra.

## Revisão Futura

Revisar quando:
- Adoção de `auth.Principal` por budgets/categories/transactions — atualizar a tabela.
- Surgir um terceiro tipo de boundary (e.g. app móvel direto via JWT) que não usa `InjectPrincipalFromHeader`.
- Data sugerida: 2027-06-12.
