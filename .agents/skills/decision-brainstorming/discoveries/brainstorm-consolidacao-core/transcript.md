# Transcript do Brainstorming Decisório

## Contexto Inicial

- **Tema:** Consolidação de Arquitetura Core — Identity, Billing & Onboarding.
- **Prompt de origem:** `docs/prompts/consolidar-arquitetura-core.md`.
- **Discoveries consumidas:**
  - `docs/discoveries/discovery-billing-hotmart-kiwify.md`
  - `docs/discoveries/discovery-identity-entitlement.md`
  - `docs/discoveries/discovery-onboarding-flow.md`
- **Codebase inspecionado:**
  - `internal/identity/` — apenas `doc.go` placeholders. `domain/doc.go` declara responsabilidade "JWT/refresh, RBAC e audit de acesso", conflitando com a discovery (sem RBAC, sem JWT no MVP).
  - `internal/finance/` — apenas `doc.go` placeholders. Sem aggregates, sem ports.
  - `internal/platform/outbox` — implementado (commit `4b7149e`). Fundação transacional pronta.
  - `internal/platform/events` — `bus.go` (volátil) já em uso.
- **Drift estrutural confirmado:** scaffold dos domínios declara RBAC/JWT; discoveries explicitamente rejeitam. Qualquer PRD escrito hoje nasce ambíguo.
- **Stack alvo:** Go + Postgres + Redis + WhatsApp Business API + Kiwify (provider único no MVP, com abstração `BillingProvider` desde dia 1).
- **Landing page (`mecontrola.app.br`, repo `LimaTeixeiraTecnologia/mecontrola-landingpage`):** não inspecionada diretamente nesta rodada; tratada como fonte de promessa de marketing que precisa ser confrontada com capacidade técnica em rodadas posteriores.

## Rodada 1 - Entendimento do Problema

### Pergunta 1.1 — Problema central
- A) Eliminar drift entre scaffold e discoveries.
- B) Materializar contratos production-proof entre Identity/Billing/Onboarding.
- C) Validar viabilidade do funil ponta-a-ponta.
- D) Os três simultaneamente (drift + contratos + validação do funil).

**Resposta:** D — consolidar drift, contratos e funil em um só dossiê com seção "Arquitetura Inegociável" abrangente.

### Pergunta 1.2 — Urgência
- A) Próximo PRD será de billing/onboarding; sem consolidação ele cristaliza decisões erradas.
- B) Landing no ar já promete o produto; risco de chegar cliente sem backend pronto.
- C) Outbox implementado mas ocioso; consumidor real ainda não existe.
- D) Risco regulatório/LGPD acumulando.

**Resposta:** A — o gatilho é o próximo PRD prestes a entrar; sem consolidar, ele nasce com RBAC/JWT por inércia do scaffold.

### Pergunta 1.3 — Resultado mínimo de sucesso
- A) Decisões inegociáveis + handoff para techspec.
- B) Decisões + esqueleto de contratos Go (assinaturas de interfaces).
- C) Decisões + sequência ordenada de épicos com critério de pronto.
- D) Decisões + contratos + sequência de épicos (pacote completo).

**Resposta:** TODOS (equivale a D ampliado) — o dossiê precisa entregar simultaneamente: regras inegociáveis, contratos Go canônicos e um épico/roadmap de implementação ordenado e priorizado. Próximo passo já deve permitir abrir épicos sem rodada extra.

### Pergunta 1.4 — Risco de adiar
- A) Risco arquitetural (3 normalizações divergentes em 3 meses).
- B) Risco de produto/negócio (MRR zero, caixa queimado).
- C) Risco de retrabalho (PRDs nascem errados e precisam ser refeitos).
- D) Risco LGPD/segurança.

**Resposta:** A + C — risco arquitetural (divergência de implementações) e risco de retrabalho (PRDs nascendo com premissas erradas) são os dois eixos dominantes para o adiamento.

## Rodada 2 - Escopo e Restrições

### Pergunta 2.1 — Escopo de entrada
- A) Só Identity + Entitlement (núcleo).
- B) Identity + Billing pipeline (webhook → estado).
- C) Identity + Billing + Onboarding end-to-end.
- D) C + LLM gate e WhatsApp routing.

**Resposta:** C — os três módulos consolidados ponta-a-ponta. LLM/router fica como referência, não como núcleo do dossiê.

### Pergunta 2.2 — Fora de escopo explícito
- A) Implementação concreta (código real).
- B) Migração para Asaas/Pagar.me/Stripe.
- C) Painel admin web.
- D) Todos os anteriores (A+B+C).

**Resposta:** B + C + D (equivale a D = "todos") — fora desta consolidação: código real concreto, multi-provider implementado e painel admin web. Esta skill produz **dossiê apenas** (regras inegociáveis + contratos Go canônicos + roadmap de épicos).

### Pergunta 2.3 — Restrição dominante
- A) Tempo de entrega.
- B) Robustez production-proof.
- C) Custo operacional baixo.
- D) Capacidade do time.

**Resposta:** A + B — tempo curto **e** production-proof simultâneos. Sem barganha em idempotência/reconciliação/auditoria, mas dossiê precisa virar PRD em janela curta. Implica que o roadmap precisa ser **enxuto e priorizado** (fatia mínima production-proof primeiro), não cobertura ampla.

### Pergunta 2.4 — Dependências externas dominantes
- A) Kiwify (qualidade da API e propagação de `?s={token}`).
- B) WhatsApp Business (templates pré-aprovados, custo).
- C) Stack interna (Postgres + Redis).
- D) Todas (A+B+C).

**Resposta:** A + B — Kiwify e WhatsApp Business são as duas dependências externas que mais restringem as decisões. Stack interna (C) é tida como fixa e suficiente, não restrição negociável.

## Rodada 3 - Alternativas

### Alternativas geradas
- **α Dossiê monolítico + roadmap único:** um único épico, 100% sequencial; billing dentro de `finance/`. Drift semântico (finance é controle pessoal, billing é cobrança SaaS).
- **β Dossiê modular + 3 épicos paralelizáveis:** `internal/billing/` separado; Identity como bloqueador; Billing e Onboarding em paralelo; reconciliação pós-MVP.
- **γ Dossiê thin + walking skeleton:** primeiro entrega fatia ponta-a-ponta sem idempotência/auditoria, depois endurece. Conflita com 2.3=B.
- **δ Identity + módulo `subscription` unificado englobando billing + onboarding token + entitlement:** menos cross-module, mas risco de god-module.

### Pergunta 3.1 — Alternativa principal
- A) α | B) β | C) γ | D) δ.

**Resposta:** B — Alt β. Dossiê modular + 3 épicos paralelos, `internal/billing/` separado, Identity como bloqueador.

### Pergunta 3.2 — Localização de billing/subscription no codebase
- A) `internal/finance/billing/` | B) `internal/billing/` separado | C) `internal/subscription/` unificado | D) `internal/platform/billing/`.

**Resposta:** B — `internal/billing/` como módulo independente, espelhando `identity/` e `finance/`. Billing é serviço de plataforma de cobrança, **distinto** de `finance/` (controle financeiro pessoal do usuário).

### Pergunta 3.3 — Onde mora EntitlementService
- A) `internal/identity/` | B) `internal/billing/` | C) `internal/platform/entitlement/` | D) Dividido (decisão pura em identity/domain, cache+I/O em billing/application).

**Resposta:** D — divisão canônica: função pura `IsEntitled(sub, now) bool` mora em `internal/identity/domain` (testável sem mock); cache Redis + I/O do estado da subscription mora em `internal/billing/application` como `EntitlementService`. Handler chama o `Service` do billing, que internamente usa a função pura. Identity expõe a regra; billing dona o estado.

### Pergunta 3.4 — Estratégia tempo × robustez no roadmap
- A) Fatia mínima production-proof completa | B) Camadas com gates explícitos | C) Por subsistema sequencial | D) Híbrido (Identity bloqueador → Billing/Onboarding paralelos → Reconciliação transversal pós-MVP).

**Resposta:** D — híbrido. Épico Identity é bloqueador (production-proof). Depois Billing e Onboarding em paralelo, cada um production-proof na sua fatia. Reconciliação como épico transversal pós-MVP (rodando, mas não bloqueia lançamento).

### Decisão consolidada da Rodada 3
- Alternativa **β refinada** com:
  - `internal/billing/` separado (3.2=B).
  - `EntitlementService` dividido entre `identity/domain` (puro) e `billing/application` (cache + I/O) (3.3=D).
  - Roadmap híbrido: Identity bloqueador → Billing + Onboarding paralelos → Reconciliação pós-MVP (3.4=D).

## Rodada 4 - Trade-offs

### Pergunta 4.1 — Idempotência + reconciliação
- A) Reconc 1h + sweep diário.
- B) Reconc só sob demanda.
- C) Reconc adaptativa.
- D) Reconc 1h apenas; sweep diário pós-MVP.

**Resposta:** D — idempotência por `(provider, external_event_id)` + reconciliação horária são inegociáveis no MVP; sweep diário full fica para backlog pós-MVP, entra quando volume ou incidente justificar.

### Pergunta 4.2 — TTL de entitlement
- A) `min(period_end - now, 1h)`.
- B) `min(period_end - now, 5min)`.
- C) `min(period_end - now, 24h)` + invalidação ativa.
- D) A + negative cache 5min para "sem subscription".

**Resposta:** D — TTL inteligente positivo (`min(period_end - now, 1h)`) **e** negative cache de 5min para users sem subscription. Inegociável. Também inegociável invalidação ativa **no fim** do `BillingEventProcessor` após commit no Postgres.

### Pergunta 4.3 — Plano família/equipe vs RBAC
- A) Sem RBAC, sem família no roadmap, `is_admin bool`.
- B) `subscription_members(subscription_id, user_id, role)` via migration futura.
- C) `Subscription.seat_count` desde já.
- D) Regra inegociável "1:1 user↔subscription"; família vira PRD novo + brainstorm novo.

**Resposta:** D — regra inegociável: **1 user = 1 subscription ativa**; plano família/equipe **não** existe no MVP e qualquer demanda futura abre PRD novo precedido de brainstorm próprio. `is_admin bool` em users; sem tabela de roles, sem policy engine. Migração futura para RBAC respeita caminho descrito na discovery (`text[] roles` ou tabela `roles` em migration isolada).

### Pergunta 4.4 — Economia de tokens LLM
- A) Gate único antes do LLM.
- B) Gate único + bypass de comandos administrativos.
- C) Gate em N pontos com `RequireFeature`.
- D) B + rate limit por usuário.

**Resposta:** B — gate único antes do LLM com **whitelist explícita** de comandos administrativos que não exigem entitlement: `ATIVAR <token>`, `/ajuda`, `/cancelar`, `/contato` (suporte). Tudo que não cair na whitelist e não tiver entitlement válido responde copy fixo de bloqueio + link de pagamento, sem invocar LLM. Rate limit por usuário (D) fica como item de backlog explícito.

### Pergunta 4.5 — Normalização E.164
- A) Função pura `NormalizeWhatsAppBR` em todo entry point.
- B) Value Object `WhatsAppNumber` imutável.
- C) B + multi-país.
- D) A + VO opcional.

**Resposta:** B — Value Object `identity/domain.WhatsAppNumber` imutável com construtor `NewWhatsAppNumber(input) (WhatsAppNumber, error)` que encapsula a normalização. APIs internas (ports, handlers, repos, payloads de eventos) trafegam `WhatsAppNumber`, **nunca** `string` cru. Tentativa de criar via string passa obrigatoriamente pelo construtor. Suporte só BR no MVP (assinatura simples, sem `region`).

## Rodada 5 - Seleção de Direção

### Síntese apresentada ao usuário
- Alternativa recomendada: **β refinada** (dossiê modular + 3 épicos paralelizáveis + `internal/billing/` separado + `EntitlementService` dividido + roadmap híbrido).
- 8 blocos de Arquitetura Inegociável consolidados (Layout, Identity, Billing, Entitlement, Onboarding, Gate LLM, Plataforma, Segurança+LGPD) + Roadmap E1→E2/E3 paralelos → E4 pós-MVP.
- Riscos residuais: H7 (propagação `?s=` na Kiwify), H8 (volume MVP), H9 (tamanho do time), CHECKOUT_URL placeholders.

### Pergunta 5.1 — Confirmação
- A) Confirmo Alt β exatamente como descrita.
- B) Confirmo com ajustes pontuais.
- C) Rodada extra.
- D) Comparar com outra alternativa.

**Resposta:** A — Alt β refinada confirmada sem alterações.

### Pergunta 5.2 — Skill consumidora do bundle
- A) `epic-story-discovery`.
- B) `create-prd` por épico (E1 primeiro, depois E2 e E3 em paralelo).
- C) `technical-discovery-production`.
- D) Outro caminho.

**Resposta:** B — `create-prd` por épico. Abrir PRD do Épico E1 (identity-foundation) primeiro; após aprovação, E2 e E3 em paralelo consumindo este bundle como insumo.

## Decisões Registradas

1. Adotar Alt β: dossiê modular + 3 épicos paralelizáveis (E1 Identity bloqueador, E2 Billing e E3 Onboarding em paralelo após E1, E4 hardening pós-MVP).
2. Criar `internal/billing/` como módulo independente, separado de `internal/finance/` (que permanece como controle financeiro pessoal).
3. `EntitlementService` dividido: função pura `IsEntitled(sub, now) bool` em `internal/identity/domain`; cache Redis + I/O em `internal/billing/application`.
4. Eliminar drift em `internal/identity/domain/doc.go` já no Épico E1; atualizar `README.md` e `AGENTS.md` dos módulos para refletir "sem RBAC, sem JWT".
5. `WhatsAppNumber` como Value Object imutável; construtor único; APIs internas trafegam o VO, nunca `string`.
6. Sem RBAC no MVP; `is_admin bool` em users; regra inegociável "1 user = 1 subscription ativa"; família/equipe abre PRD novo + brainstorm próprio.
7. Webhook Kiwify: idempotência por `(provider, external_event_id)` + reconciliação horária inegociáveis; sweep diário full vai para Épico E4 pós-MVP.
8. Cache de entitlement: `TTL = min(period_end - now, 1h)` + negative cache 5min; invalidação síncrona após commit no `BillingEventProcessor`.
9. Gate LLM único antes do modelo com whitelist `ATIVAR <token>`, `/ajuda`, `/cancelar`, `/contato`; rate limit por usuário fica para backlog.
10. `Subscription.period_length` por plano (30d/90d/365d); sem constante hardcoded.
11. Outbox transacional para todos eventos críticos; `events.Bus` permanece volátil para fan-out intra-processo.
12. Próximo passo: `create-prd` para Épico E1 (identity-foundation); depois `create-prd` para E2 e E3 em paralelo.
