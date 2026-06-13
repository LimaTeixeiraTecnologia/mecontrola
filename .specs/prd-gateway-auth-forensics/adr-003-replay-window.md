# ADR-003 — Janela de replay 60s sem cache de nonce

## Metadados

- **Título:** Aceitar replay dentro de 60s; sem cache de nonces no MVP
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-03; [techspec](techspec.md) seção Workflow; plano-fonte seção 6 (pós go-live)

## Contexto

A verificação HMAC com janela de timestamp ±60s permite que uma assinatura capturada na rede seja reenviada dentro dessa janela. Mitigações possíveis: (a) reduzir janela; (b) cache de nonce; (c) aceitar e mitigar por outras camadas.

Contexto do projeto:
- TLS termina no Caddy; rede entre Caddy e app é loopback no mesmo host.
- Mutations já são protegidas por `Idempotency-Key` em `platform_idempotency_keys`.
- Leituras (`GET /api/v1/cards`) são idempotentes por natureza.
- Sem Redis; cache in-memory aceitável mas adiciona estado mutável no hot path.

## Decisão

**Aceitar replay dentro da janela de 60s sem nenhum cache de nonce.**

Mitigações de defesa em profundidade já existentes:
- `Idempotency-Key` em todas as mutations (POST/PUT/DELETE).
- TLS na borda + Caddy strip de headers externos.
- Métrica `identity_gateway_auth_total` permite detectar burst suspeito.

Documentado como **risco residual** a ser revisitado pós go-live se observado incidente real. Cache de nonce em Redis está na lista de segunda onda do plano-fonte.

## Alternativas Consideradas

1. **Cache in-memory de `(timestamp, signature)` por 60s** — `sync.Map` com goroutine de evict. **Rejeitada para MVP**: ~300KB de memória estável (cálculo em pico 5 req/s × 60s × 1KB), goroutine extra para limpeza, complexidade vs benefício marginal em rede controlada.
2. **Reduzir janela para 10s** — limita raio mas exige relógios sincronizados (NTP rigoroso) em VPS + cliente. **Rejeitada**: gera 401 legítimo por skew normal de NTP (±100ms a ±1s); operacional ruim.
3. **Cache distribuído (Redis)** — solução correta para multi-node. **Rejeitada**: extrapola escopo MVP single-node; Redis seria daemon adicional só para isso.

## Consequências

### Benefícios Esperados

- Zero memória adicional no hot path.
- Zero código extra (sem cache, sem evict, sem locking).
- Workflow puro permanece puro (cache de nonce introduz IO ou estado global).

### Trade-offs e Custos

- Replay de leitura possível dentro de 60s — impacto operacional zero (response é idempotente).
- Replay de mutation possível dentro de 60s **se** o atacante também conhece um `Idempotency-Key` não usado antes — improvável (key é gerada pelo client, opaca, alta entropia).

### Riscos e Mitigações

- **R-01**: replay de leitura dispara métricas/billing duplicadas. **Mitigação**: dashboards não dependem de unicidade de request, apenas contam volume bruto.
- **R-02**: replay coincide com mutation idempotente quebrada. **Mitigação**: testes do PRD `prd-card-crud-mvp` cobrem idempotência.

## Plano de Implementação

Nenhuma implementação dedicada. Apenas:
1. `GatewayTimestamp` smart constructor valida janela `[now - 60s, now + 60s]`.
2. Documentar risco residual em `docs/runbooks/gateway-auth.md`.

## Monitoramento e Validação

- Métrica `identity_gateway_auth_total{result="stale_timestamp"}` em estado estacionário < 0.1% — alerta acima.
- Sem métrica específica para replay (aceito como cego).

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: seção "Riscos aceitos".
- Plano-fonte seção 6: já cita Redis para idempotência/dedup distribuído como pós go-live.

## Revisão Futura

Revisar quando:
- Volume passar de 50 req/s sustentado (replay ganha valor de exploit).
- Houver incidente real de replay documentado.
- Migração para multi-node (Redis vira obrigatório por outros motivos).
- Data sugerida: 2027-06-12.
