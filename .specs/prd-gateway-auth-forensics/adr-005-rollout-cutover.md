# ADR-005 — Rollout cutover atômico Caddy + LLM + app

## Metadados

- **Título:** Cutover atômico sem soft-launch parcial
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) seção "Plano de Rollout", RF-22; plano-fonte seção 9

## Contexto

Introduzir um gate de autenticação em produção tem dois modos de rollout:
- **Soft-launch**: feature flag liga/desliga enforcement; permite "modo shadow" onde middleware valida mas não rejeita.
- **Cutover atômico**: deploy único que liga tudo de uma vez.

Soft-launch parece mais seguro mas introduz risco de "deploy passou mas regra ficou frouxa" — o sintoma é silencioso e pode persistir indefinidamente.

## Decisão

**Cutover atômico em ordem fixa:**

1. **Pré-deploy** — provisionar `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT` no host. Operador da LLM atualiza cliente em modo "shadow" (assina e envia headers, mas servidor ainda não enforça).
2. **Deploy do app Go** com `RequireGatewayAuth` em modo **enforce**. Janela dupla (`CURRENT` + `NEXT`) ativa desde o boot, mas `NEXT` vazio na primeira ativação.
3. **Caddy** atualizado no mesmo deploy para strip dos headers externos.
4. **Validação E2E** imediata: smoke test do plano-fonte seção 9, itens 1 e 13 — curl externo deve retornar 401; curl interno (via gateway) deve passar.
5. **Rollback** se algo falhar: revert do deploy (app + Caddy) na mesma janela. LLM continua enviando headers (idempotente; servidor antigo ignora).

Sem feature flag de "enforce on/off" no código. Sem env var de "modo dry-run".

## Alternativas Consideradas

1. **Soft-launch com env `GATEWAY_AUTH_ENFORCE=false`** — middleware valida e loga falhas mas não retorna 401. **Rejeitada**: cria estado intermediário onde "deploy passou mas auth está aberta"; risco de esquecer de virar para `true` em produção. Sintoma silencioso.
2. **Modo shadow controlado por header** — `X-Dry-Run: 1` força bypass. **Rejeitada**: header controlado pelo client; em prática é "any client pode skipar o gate" — bypass trivial.
3. **Rollout em dois deploys** (primeiro o app aceita ambos modos, segundo enforça). **Rejeitada**: complexifica diff entre PRs e risco de drift entre os dois deploys; sem benefício real.

## Consequências

### Benefícios Esperados

- Sem estado intermediário inseguro.
- Diff do deploy é mínimo e auditável.
- Rollback simétrico (revert do mesmo commit).

### Trade-offs e Custos

- Janela curta (~minutos) onde uma falha de coordenação entre LLM cliente e servidor causa 100% de 401. Mitigação: smoke test imediato; rollback rápido.
- Exige operador disponível durante deploy.

### Riscos e Mitigações

- **R-01**: cliente LLM ainda não foi atualizado quando o app sobe enforce. **Mitigação**: passo 1 do rollout (cliente em shadow assinando) garante que cliente já está pronto antes do deploy.
- **R-02**: cliente LLM canoniza errado mas testes locais passaram. **Mitigação**: vetor fixo de teste cross-lang (ADR-001) executado pré-deploy em staging.
- **R-03**: Caddy não strip-a headers e algum cliente externo consegue passar. **Mitigação**: smoke test seção 9 item 1 do plano-fonte (curl externo → 401) **antes** de declarar deploy estável.

## Plano de Implementação

1. Runbook `docs/runbooks/gateway-auth-rollout.md` com checklist passo a passo + critérios de rollback.
2. Smoke test scriptado em `deployment/scripts/gateway-auth-smoke.sh` executável remotamente após deploy.
3. Janela de deploy em horário de baixa carga (acordado entre operador e usuários).

## Monitoramento e Validação

- Painel "Auth Module" no Grafana com `identity_gateway_auth_total{result}` filtrado pelas últimas 15 min do deploy.
- Critério "deploy ok": ≥ 1 request com `result="valid"` e 0 requests com `result ∈ {missing_header, invalid_signature}` nos 5 min seguintes.
- Critério de rollback: ≥ 10 requests com `result="invalid_signature"` ou `result="missing_header"` em 5 min — revert imediato.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth-rollout.md`.
- `deployment/scripts/gateway-auth-smoke.sh`.
- Comunicação com operador da LLM antes do deploy (acordo de janela).

## Revisão Futura

Revisar quando:
- Múltiplas integrações além da LLM (vai exigir rollout por etapas).
- Migração para JWT (rollout fica mais complexo por causa do refresh).
- Data sugerida: após o primeiro rollout real (registrar lessons learned aqui).
