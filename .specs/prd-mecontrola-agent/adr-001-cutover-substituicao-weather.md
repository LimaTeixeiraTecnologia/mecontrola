# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Cutover — substituir o weather-agent por `MeControlaAgent` reusando o molde estrutural
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma; dono do produto
- **Relacionados:** PRD `.specs/prd-mecontrola-agent/prd.md` (RF-01..RF-05, D-01), techspec.md; ADR-002..006; `.specs/prd-agents-weather-mastra/` (origem do molde)

## Contexto

`internal/agents` hoje é um port do exemplo weather do Mastra, útil como prova do substrato mas sem valor de produto. O PRD exige substituição integral por um agente financeiro conversacional, sem convivência (RF-02/RF-03). O weather deve servir apenas como **molde estrutural** (RF-04). O `internal/onboarding` (ativação de conta por magic token) é separado e deve permanecer intacto (D-01).

## Decisão

Reusar o **layout** de `internal/agents` (`application/{agents,tools,workflows,scorers,usecases,interfaces}`, `infrastructure/messaging/.../consumers`, `module.go`) e o substrato `internal/platform/*`, **substituindo o conteúdo de domínio** weather pelo financeiro. Remover, no mesmo cutover e sem resíduo: `application/agents/agent.go` (weather), tools/workflows/scorers/domain weather, `interfaces.WeatherClient`, `infrastructure/weather`, e o campo `WeatherClient` em `agents.Deps`/wiring. Manter `HandleInbound` (assinatura), o registro do `EmbeddingIndexHandler` e do `WhatsAppInboundConsumer`. Preservar `internal/onboarding` intacto.

## Alternativas Consideradas

- **Criar um novo módulo `internal/financeagent` ao lado do weather** — Vantagem: menos risco de quebra imediata. Desvantagem: viola RF-02/RF-03 (convivência), duplica wiring e deixa código morto. Rejeitada.
- **Refatorar incrementalmente o weather mantendo nomes** — Vantagem: diff menor. Desvantagem: arrasta semântica de clima e nomes enganosos; risco de resíduo. Rejeitada.

## Consequências

### Benefícios Esperados

- Padrão canônico de consumidor da plataforma preservado; menor superfície morta.
- Cutover atômico e verificável por gates de governança.

### Trade-offs e Custos

- Remoção ampla exige varredura cuidadosa de imports/wiring (server, worker, e2e).

### Riscos e Mitigações

- **Resíduo órfão** (wiring/config weather) → gate: `grep` por `weather`/`WeatherClient` retorna vazio em produção; build + e2e verdes. Rollback: o cutover é a última etapa da ordem de build; reverter o commit do cutover restaura o weather sem afetar os módulos novos.

## Plano de Implementação

1. Construir todo o conteúdo novo (ADR-002..006) com weather ainda presente.
2. Trocar o registro no `AgentRegistry` para o `MeControlaAgent`.
3. Remover artefatos weather e ajustar `Deps`/wiring/e2e.
4. Rodar gates + build + testes.

## Monitoramento e Validação

- `agents_inbound_total{outcome}` saudável pós-deploy; ausência de erros de wiring.
- Critério de sucesso: zero referência weather; uma mensagem financeira percorre o fluxo e responde.

## Impacto em Documentação e Operação

- Atualizar `docs/runs/` com o plano; runbook do agente financeiro (jornada completa) conforme preferência registrada.

## Revisão Futura

- Revisar se um segundo agente for adicionado (multi-agente no registry).
