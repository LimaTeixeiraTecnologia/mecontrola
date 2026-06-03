# DECISION BRIEF

## Problema
O projeto MeControla tem três discoveries production-ready (`docs/discoveries/discovery-billing-hotmart-kiwify.md`, `discovery-identity-entitlement.md`, `discovery-onboarding-flow.md`) que descrevem como operar cobrança recorrente via Kiwify, identidade mínima sem RBAC e onboarding via magic token. Em paralelo, o codebase tem `internal/identity/` e `internal/finance/` apenas com `doc.go` placeholders. O `doc.go` do módulo `identity/domain` declara responsabilidades de "JWT/refresh, RBAC e audit de acesso", em conflito direto com a discovery (que rejeita explicitamente RBAC e JWT no MVP). A landing page `mecontrola.app.br` já está construída prometendo planos Mensal/Trimestral/Anual e canal 100% WhatsApp, com CTAs ainda apontando para `#` placeholder — mas o backend ainda não tem nem o agregado `User`. Sem consolidar, qualquer PRD que entrar agora vai cristalizar decisões erradas no código (ex: nascer com `roles[]` por inércia do README do módulo) ou inventar normalizações divergentes de telefone, cache de entitlement e idempotência de webhook em cada módulo.

## Objetivo
Produzir um dossiê de arquitetura inegociável que (a) elimine o drift entre o scaffold dos módulos e as decisões das discoveries, (b) materialize os contratos Go canônicos (assinaturas de portas e VOs) que cada PRD subsequente deve respeitar e (c) entregue um roadmap ordenado de épicos com critério de pronto explícito. O sucesso é medido pela capacidade de abrir os próximos PRDs (E1 Identity, E2 Billing, E3 Onboarding) sem rodada nova de discovery e sem reabrir decisões consolidadas aqui.

## Escopo Inicial
Inclui:
- Consolidação end-to-end de Identity + Billing + Onboarding (núcleo dos 3 discoveries).
- Definição de fronteiras de módulo: criação de `internal/billing/` separado, manutenção de `internal/identity/` e `internal/finance/`, definição de `internal/onboarding/` como módulo independente.
- Eliminação de drift no scaffold: correção do `doc.go`, do `README.md` e do `AGENTS.md` de `internal/identity/` para refletir a decisão "sem RBAC, sem JWT, canal autentica".
- Contratos Go canônicos (apenas assinaturas, não implementação): `User`, `WhatsAppNumber` (VO), `Email` (VO), `Subscription`, `SignupToken`, `BillingProvider`, `EntitlementService`, `UserRepository`, função pura `IsEntitled`.
- Regras inegociáveis sobre idempotência de webhook, TTL de cache de entitlement, normalização E.164, fluxo de ativação, whitelist de comandos sem entitlement, mapeamento Kiwify → estado canônico, política de uso entre `outbox` e `events.Bus`.
- Roadmap de 3 épicos paralelizáveis após Identity, com Épico 4 (hardening de reconciliação) já parcialmente coberto como backlog pós-MVP.

Exclui:
- Implementação concreta de qualquer módulo (código real fica para os PRDs/techspecs downstream).
- Migração para Asaas/Pagar.me/Stripe (provider único Kiwify no MVP, abstração desenhada para multi-provider mas não implementada).
- Painel administrativo web (admin opera via CLI/script no MVP; `is_admin bool` suporta isso).
- LLM gate detalhado e router de mensagens WhatsApp (citado como referência, mas não é núcleo da consolidação).
- Plano família/equipe e qualquer forma de RBAC.
- Trial nativo (landing não promete trial; fica como flag opcional para experimentos).
- Sweep diário completo de reconciliação (entra como Épico 4 pós-MVP; só reconciliação horária é inegociável no MVP).

## Restrições
- Stack obrigatória: Go (versão de `go.mod`), Postgres, Redis, WhatsApp Business API, Kiwify como provider único de pagamento.
- Arquitetura hexagonal canônica do MeControla (`domain/` puro, `application/` com ports, `infrastructure/` com adapters) imposta pelo `CLAUDE.md`, `AGENTS.md` e `depguard` em `.golangci.yml`.
- Comunicação cross-module só por interface declarada em `application/` do consumidor ou por Domain Event publicado em `internal/platform/events` (`Bus` volátil) ou via `internal/platform/outbox` (persistente, transacional).
- Idempotência por `event_id` é obrigatória em todo consumidor de `outbox.Publisher` (regra do AGENTS.md seção "Outbox vs events.Bus").
- Planos contratados pela landing impõem cadências mensal (30d), trimestral (90d) e anual (365d): `Subscription` carrega `period_length` por plano, não constante hardcoded.
- Modais de pagamento prometidos pela landing: PIX e cartão (Kiwify cobre nativamente).
- Tensão tempo × robustez: roadmap precisa ser enxuto e priorizado, mas sem barganha em idempotência, reconciliação e auditoria.
- Cerco externo dominante: Kiwify (qualidade da API e propagação de `?s={token}`) e WhatsApp Business (templates pré-aprovados para outreach).

## Hipóteses
- H1 (confirmada): Discoveries de billing, identity e onboarding já refletem a direção desejada de produto — usuário citou as 3 como fonte canônica no prompt `docs/prompts/consolidar-arquitetura-core.md`.
- H2 (confirmada): Scaffold dos módulos `internal/identity/` e `internal/finance/` está virgem — `ls` mostra apenas `doc.go` em cada subpasta; não há código herdado bloqueando reescrita das fronteiras.
- H3 (confirmada): `internal/platform/outbox` está pronto para uso transacional — commit `4b7149e feat(outbox): implemente fundacao transacional de eventos` e arquivos em `platform/outbox` + `runtime/outbox_subsystem.go` confirmam.
- H4 (confirmada): `internal/platform/events.Bus` é volátil, sem persistência — escolha entre Bus e Outbox é responsabilidade do consumidor.
- H5 (confirmada): Há drift entre scaffold de identity e discovery — `internal/identity/domain/doc.go:3-4` declara "JWT/refresh, RBAC e audit de acesso"; a discovery rejeita ambos no MVP. Consolidação precisa corrigir doc.go + README + AGENTS do módulo.
- H6 (confirmada): Landing promete planos Mensal/Trimestral/Anual sem plano família/equipe — `src/lib/content.ts` lista `plans = [mensal R$29,90, trimestral R$80,73, anual R$297,80]` com copy 100% individual. CHECKOUT_URL ainda `#` (placeholder), janela curta para consolidar antes de publicar links reais.
- H7 (não validada): Kiwify propaga query param `?s={token}` no webhook via custom field ou UTM — precisa de prova com compra real de R$ 1 (Pix) no produto sandbox antes de E2 fechar.
- H8 (não validada): Volume MVP esperado é < 1k assinantes em 6 meses — confirmar com produto; se errado, Redis Streams e cache de entitlement podem precisar de revisão.
- H9 (não validada): Time de implementação é de 1 a 2 devs — confirmar nas próximas rodadas de PRD; impacta tamanho dos PRDs e paralelismo real.

## Alternativas Avaliadas
### Alternativa 1 - α Dossiê monolítico + roadmap único
Resumo:
Um único `decision-brief.md` com tudo: regras inegociáveis, contratos Go, máquina de estados unificada, roadmap em um único épico `epic-fundacao-saas` dividido em ~10 stories sequenciais. Billing vive em `internal/finance/billing/` para evitar criar módulo novo.

Viabilidade:
Técnica: alta — uma única estrutura facilita revisão. Operacional: bloqueio sequencial inteiro; se 1 story atrasa, todo roadmap atrasa. Financeira: baixa em custo de governança (1 PRD), mas drift semântico (billing dentro de finance, sendo controle pessoal != cobrança SaaS) gera retrabalho de longo prazo.

### Alternativa 2 - β Dossiê modular + 3 épicos paralelizáveis com internal/billing/ separado
Resumo:
`decision-brief.md` com 3 blocos de Arquitetura Inegociável (Identity, Billing, Onboarding) + 3 épicos independentes com interfaces inter-módulo definidas. Cria `internal/billing/` como módulo separado (billing é serviço de plataforma de cobrança, distinto de `finance/` que é controle pessoal). Identity é bloqueador dos dois outros. Reconciliação completa fica como Épico 4 pós-MVP.

Viabilidade:
Técnica: alta — fronteiras semânticas corretas e contratos imutáveis; depguard impede regressão. Operacional: permite paralelismo após Identity; PRDs por módulo são curtos; rollback granular. Financeira: zero infra nova (Postgres + Redis + outbox já existentes); custo é só desenvolvimento, e quando vier Asaas/Pagar.me mexe só em `internal/billing/`.

### Alternativa 3 - γ Dossiê thin + walking skeleton primeiro
Resumo:
Dossiê enxuto focado em fronteiras. Antes dos 3 épicos, um épico zero entrega walking skeleton end-to-end com a fatia mais fina possível: `User` cru sem soft delete, `Subscription` só com status `ACTIVE`, webhook que só persiste raw, ativação que só atualiza `whatsapp_real`. Depois cada épico endurece uma camada (idempotência, máquina completa, fallback, reconciliação).

Viabilidade:
Técnica: aprende rápido com produção real. Operacional: viola restrição 2.3=B (production-proof); abre mão de idempotência e auditoria de saída, o que tipicamente custa caro para introduzir depois — webhook duplicado vira inconsistência em produção. Financeira: rápido para validar funil, mas custo total de hardening posterior é maior que entregar production-proof na fatia.

### Alternativa 4 - δ Identity + módulo subscription unificado englobando billing + onboarding + entitlement
Resumo:
Identity isolado (`User`, `WhatsAppNumber`). Cria `internal/subscription/` que engloba billing pipeline + onboarding magic token + entitlement service, todos no mesmo módulo como subagregados internos. Argumento: signup token, subscription, transação, entitlement e webhook são o mesmo bounded context ("ciclo de vida da assinatura"); separar gera comunicação cross-module artificial.

Viabilidade:
Técnica: menos cross-module, menos boilerplate de eventos. Operacional: módulo `subscription` fica grande; testes ficam mais lentos; depguard precisa de refinamento para separar webhook (público) de máquina de estados (interno). Risco real de criar god-module com responsabilidades misturadas. Financeira: aparente economia de PRDs, mas custo de manutenção e onboarding de novo dev é maior.

## Trade-offs
- Alt β recomendada vs Alt α: aceitamos mais arquivos e mais overhead de governança (3 PRDs em vez de 1) em troca de fronteiras semânticas corretas, paralelismo após Identity e manutenção barata quando vierem Asaas/Pagar.me ou plano família.
- Reconciliação horária + sweep diário pós-MVP (Resposta 4.1=D): aceitamos não ter sweep diário completo no MVP em troca de chegar mais rápido ao primeiro cliente pagante; mitigado pela reconciliação horária + idempotência + outbox; alerta operacional fica obrigatório.
- TTL inteligente + negative cache 5min (Resposta 4.2=D): aceitamos ter cache negativo que pode atrasar até 5min a primeira ativação de um number novo, em troca de proteger Postgres contra spam de números aleatórios.
- 1 user = 1 subscription ativa (Resposta 4.3=D): aceitamos não suportar plano família/equipe no MVP nem no roadmap; qualquer demanda futura abre PRD novo precedido de brainstorm próprio.
- Gate único antes do LLM com whitelist administrativa (Resposta 4.4=B): aceitamos não gatear comandos individuais nem ter rate limit por usuário no MVP, em troca de implementação simples; rate limit fica como item de backlog explícito.
- Value Object `WhatsAppNumber` (Resposta 4.5=B): aceitamos um tipo de domínio adicional (vs string crua) em troca de garantia em tempo de compilação de que nenhum número não normalizado entra em port, repo ou evento.
- Outbox para tudo que precisa de durabilidade (decisão derivada): aceitamos custo de coordenação transacional (publicar dentro da mesma tx do Postgres) em troca de idempotência e replay garantidos para webhooks Kiwify, mudanças de estado de subscription e invalidação de cache.
- Reconciliação respeita máquina de estados única (decisão derivada da discovery): mesmo na reconciliação, divergência dispara evento sintético pelo mesmo `BillingEventProcessor`; nunca atualiza estado direto fora dele.

## Riscos
- Risco: Kiwify não propagar `?s={token}` no webhook (H7 não validada).
  Impacto: alto — magic token quebra, onboarding cai no fallback E.164 que tem janela de erro.
  Probabilidade: média — depende da configuração de produto + custom field na Kiwify.
  Mitigação: prova de propagação com compra real R$ 1 Pix em sandbox antes de fechar Épico E2; suporte a `?s=` via UTM ou custom field; fallback de match por E.164 já especificado na discovery.
- Risco: Webhook duplicado ou fora de ordem chegando ao `BillingEventProcessor`.
  Impacto: alto — pode regressar estado da subscription, liberar acesso a usuário cancelado, ou cobrar duas vezes.
  Probabilidade: alta — Kiwify e Hotmart retentam; rede falha.
  Mitigação: dedup `(provider, external_event_id)` com `INSERT ... ON CONFLICT DO NOTHING`; processador ignora evento se `occurred_at` representa regressão; outbox garante exactly-once por `event_id`.
- Risco: Cache de entitlement servir decisão stale durante mudança de estado (cancelamento, refund).
  Impacto: alto — usuário cancelado continua usando; risco financeiro e LGPD.
  Probabilidade: baixa após implementar invalidação ativa no fim do `BillingEventProcessor`, mas existe janela.
  Mitigação: TTL `min(period_end - now, 1h)` cap evita stale longo; invalidação síncrona após commit no Postgres; reconciliação horária pega o resto.
- Risco: Drift entre o `doc.go` do `identity` e a discovery não corrigido antes do primeiro PRD.
  Impacto: médio — PRD novo nasce com RBAC por inércia; precisa ser refeito.
  Probabilidade: alta se a consolidação não bloquear o próximo PRD.
  Mitigação: Épico E1 inclui obrigatoriamente correção de `doc.go`, `README.md` e `AGENTS.md` de `internal/identity/`; criação dos mesmos artefatos para `internal/billing/` e `internal/onboarding/` desde o início.
- Risco: Volume MVP maior que H8 (1k subs em 6 meses).
  Impacto: médio — Redis Streams e cache de entitlement podem precisar de revisão; reconciliação horária pode estourar rate limit de 100 req/min da Kiwify.
  Probabilidade: baixa no MVP, média no longo prazo.
  Mitigação: roadmap inclui revisão de capacidade em 5k subs; reconciliação em batch com rate limit; outbox absorve picos.
- Risco: Promoção pública da landing antes de E2 + E3 estarem prontos.
  Impacto: alto — promessas de marketing sem backend pronto = caixa não capturada.
  Probabilidade: média — CHECKOUT_URL ainda placeholder, mas pode mudar a qualquer momento.
  Mitigação: criar gate explícito entre Marketing e Backend; landing só substitui `#` por URL real quando E2 + E3 estiverem em staging com smoke test passando.

## Custos
Estimativa relativa:
média

Drivers de custo:
- Desenvolvimento: ~4 semanas com 1 dev ou ~2-3 semanas com 2 devs para os 3 épicos (E1 + E2 || E3), exclui hardening de reconciliação que entra como Épico 4 pós-MVP.
- Infraestrutura: zero infra nova — Postgres + Redis + outbox + events.Bus já existem; sem Kafka, sem NATS, sem novo serviço.
- Custo por venda (Kiwify): 8,99% + R$ 2,49 por venda padrão (ticket Mensal R$29,90 ≈ R$24,62 líquido); avaliar plano pago da Kiwify (mensalidade + taxa zero) quando ticket médio < R$29 ou MRR > ~R$30k.
- WhatsApp Business API: custo por template de outreach (notification messages); estimar 1-2 templates por subscription PAID não consumida nas primeiras 48h.
- LLM: tokens gateados pelo entitlement; gasto zero para usuário sem subscription (decisão 4.4=B); custo segue volume de mensagens de pagantes.
- Manutenção: baixa após estabilização; depguard impede regressão de fronteira; cada módulo evolui isoladamente.

## Impactos Operacionais
- Suporte ganha runbook por módulo desde o Épico 1: como dar entitlement manual (`entitlement_overrides`), como reprocessar webhook (`webhook_events.status = 'RECEIVED'`), como ativar manualmente (`signup_tokens FOR UPDATE`), como anonimizar dados de usuário (soft delete + cleanup).
- Deploy granular: cada módulo pode ser deployado independentemente (Identity é prerequisito de Billing e Onboarding em runtime apenas porque expõe `UserRepository` via interface).
- Rollback é por módulo: rollback de Onboarding não afeta Billing; rollback de Billing pausa novas assinaturas mas preserva entitlement em cache até `period_end`.
- Operação rotineira: monitorar `webhook_events.status = 'RECEIVED' AND received_at < now() - 5min` (alerta de processador parado), `pending_paid_tokens > 10` (alerta de fluxo de ativação quebrado), filas/streams de outbox sem consumidor.
- Time pequeno (H9): documentação curta por módulo facilita onboarding de novo dev; depguard ensina fronteiras automaticamente.
- LGPD: política de mascaramento de PII em logs precisa ser implementada na primeira feature de cada módulo, não depois; soft delete obrigatório em users; runbook de "exclusão de dados" desde Épico E1.

## Segurança
- Autenticação do canal WhatsApp é responsabilidade do Meta (header `from` confiável); backend não faz autenticação adicional na rota de mensagens; admin web futuro usa magic link por email.
- Webhook ingress verifica assinatura/token antes de qualquer parse pesado (`X-Kiwify-Signature` ou `?token=`); falha → 401 imediato.
- Sem RBAC, sem JWT, sem sessões no MVP: `is_admin bool` em users resolve admin com 10 linhas de middleware; quando RBAC virar necessário, migração isolada documentada.
- PII confinada por módulo: `whatsapp_number`, `email`, `whatsapp_input` mascaradas em logs (`+5511***88887777` → `+5511***888***777`); valor cru só em queries seguras.
- Soft delete obrigatório em `users` com `deleted_at`; todas as queries filtram `WHERE deleted_at IS NULL`; janela de 30 dias para anonimização efetiva (LGPD).
- Rate limit em `/api/checkout-session` (10/min por IP) para anti-abuse.
- Tokens (`Kiwify webhook secret`, `WhatsApp Business token`) em vault/secret manager, nunca em env var commitado.
- Webhook URL distinta por ambiente (dev/staging/prod) com secrets distintos para evitar replay cross-ambiente.
- Auditoria: `webhook_events` é event store imutável (JSONB); `entitlement_overrides` registra quem deu acesso manual e por quê.
- LGPD: mecanismo de "deletion request" mapeado desde E1; PII mascarada em logs; `whatsapp_input` do checkout tratado como input não-verificado.

## Observabilidade
- Métricas por módulo (Prometheus):
  - `identity_user_created_total{path="onboarding|admin"}`
  - `identity_user_soft_deleted_total`
  - `billing_webhook_received_total{provider, event_type}`
  - `billing_webhook_failed_total{provider, reason}`
  - `billing_event_processed_total{event_type, outcome}`
  - `billing_subscription_state_total{status}` (gauge)
  - `entitlement_check_total{decision="granted|denied", reason}`
  - `entitlement_cache_hit_ratio`
  - `onboarding_checkout_session_created_total`
  - `onboarding_token_paid_total`
  - `onboarding_activation_consumed_total{path="direct|fallback_match|outreach"}`
  - `onboarding_pending_paid_tokens` (gauge)
- Logs estruturados: cada decisão de entitlement gera evento JSON com `user_id`, `decision`, `reason`, `latency_ms`; PII mascarada.
- Traces: spans para webhook → outbox → processor → invalidação cache; correlação por `event_id`.
- Alertas inegociáveis: webhook falhando 3x consecutivas, fila/stream de outbox crescendo, reconciliação divergindo, `pending_paid_tokens > N` (threshold definido no Épico E3).
- Funil de onboarding (alvo do produto, não SRE): `checkout_session_paid / checkout_session_created > 70%`; `activation_consumed / checkout_session_paid > 90%`; `first_message_processed / activation_consumed > 80%`.
- Dashboard MRR/churn entra no Épico E4 (pós-MVP).

## Escalabilidade
- Capacidade MVP esperada (H8 não validada): < 1k subs em 6 meses; Postgres único + Redis pequeno + Redis Streams para outbox = suficiente; sem necessidade de shard, sem necessidade de Kafka/NATS.
- Gargalos esperados:
  - Reconciliação horária bate em rate limit Kiwify (100 req/min) a partir de 6k subs ativas; mitigação por batch + retry com backoff.
  - Cache de entitlement domina Postgres (`SELECT FROM subscriptions WHERE user_id`); índice obrigatório em `(user_id, status)`.
  - Webhook ingress precisa retornar 200 em < 2s; outbox transacional + commit antes de 200 garante isso enquanto write rate < 100/s; revisar acima disso.
- Crescimento previsto: Identity escala linear com base; Billing escala linear com eventos; Onboarding escala linear com checkouts.
- Limites operacionais: revisar capacidade quando atingir 5k subs (Redis Streams, índices Postgres, rate limit Kiwify, custo WhatsApp Business).
- Multi-provider futuro: `BillingProvider` interface permite plugar Asaas/Pagar.me/Stripe sem mexer no `BillingEventProcessor`; reconciliação precisa de implementação por provider, mas a interface está pronta.

## Alternativa Recomendada
Alternativa 2 - β Dossiê modular + 3 épicos paralelizáveis com internal/billing/ separado, com os seguintes refinamentos confirmados na Rodada 3: (a) `internal/billing/` como módulo independente, espelhando `internal/identity/` e `internal/finance/`; (b) `EntitlementService` dividido — função pura `IsEntitled(sub, now) bool` em `internal/identity/domain` e cache + I/O em `internal/billing/application`; (c) roadmap híbrido — Identity como bloqueador, Billing e Onboarding em paralelo após Identity, reconciliação completa como Épico 4 pós-MVP.

## Justificativa
A Alt β é a única que respeita simultaneamente as cinco coisas que o usuário pediu nas Rodadas 1 e 2: (1) consolidar drift, contratos e funil em paralelo (1.1=D); (2) entregar dossiê + contratos Go + roadmap de épicos em pacote único (1.3=TODOS); (3) escopo end-to-end Identity + Billing + Onboarding sem incluir LLM como núcleo (2.1=C); (4) bundle entrega dossiê apenas, com código nos PRDs downstream (2.2=B+C+D); (5) tempo curto + robustez production-proof sem barganha (2.3=A+B). O scorecard quantitativo confirma: Alt β soma 38 pontos contra 30 (α), 29 (γ) e 32 (δ), com notas máximas (5) em Segurança, Confiabilidade e Manutenibilidade. Alt α perde em manutenibilidade por causa do drift semântico de colocar billing em finance. Alt γ viola a restrição production-proof. Alt δ tende a god-module e penaliza testes. Os refinamentos da Rodada 3 (3.2=B, 3.3=D, 3.4=D) tornam a Alt β ainda mais alinhada com a Uber Go Style (funções puras separadas de I/O) e com o contrato dual `outbox` vs `events.Bus` já em vigor no `internal/platform/`.

---

## Arquitetura Inegociável

Esta seção lista as regras que **não podem ser quebradas** pelos PRDs, techspecs e tarefas downstream. Qualquer descumprimento exige brainstorm novo precedendo o PRD que pretenda mudar a regra.

### A. Layout de módulos
- `internal/identity/` — `User` agregate, `WhatsAppNumber` VO, `Email` VO, `is_admin bool`, soft delete, port `UserRepository`, função pura `IsEntitled(sub, now) bool`.
- `internal/billing/` — módulo independente. Webhook ingress, `BillingEventProcessor`, `Subscription` agregate, máquina de estados canônica, `EntitlementService` (cache + I/O), interface `BillingProvider`, adapter Kiwify.
- `internal/onboarding/` — módulo independente. `SignupToken` agregate, `/api/checkout-session`, handler `ATIVAR`, job de outreach, fallback E.164.
- `internal/finance/` — continua como controle financeiro pessoal do usuário. **NÃO** abriga billing.
- `internal/platform/outbox` é a fonte de durabilidade para todos eventos críticos (webhooks Kiwify, mudanças de estado de subscription, invalidação de cache, token paid).
- `internal/platform/events.Bus` é volátil; usar apenas para fan-out best-effort dentro do mesmo processo.
- Comunicação cross-module só via interface declarada em `application/` do consumidor ou via Domain Event publicado por outbox/Bus. Import direto cross-module é proibido por depguard.

### B. Identity (regras imutáveis)
- PK de `User` é UUID. Telefone muda; UUID não.
- `WhatsAppNumber` é Value Object imutável construído por `NewWhatsAppNumber(input string) (WhatsAppNumber, error)`. APIs internas trafegam `WhatsAppNumber`, **nunca** `string` cru. Construtor encapsula normalização E.164 BR.
- `Email` é Value Object imutável construído por `NewEmail(input string) (Email, error)`.
- Soft delete obrigatório em `users` (`deleted_at`); todas as queries filtram `WHERE deleted_at IS NULL`.
- Histórico de números em `user_whatsapp_history` com `(user_id, number, active, linked_at, unlinked_at, reason)`.
- `is_admin bool` resolve admin no MVP. **Sem** tabela `roles`, **sem** policy engine, **sem** JWT, **sem** sessions.
- `IsEntitled(sub *Subscription, now time.Time) bool` é função pura em `internal/identity/domain`, testável sem mock. Cobertura 100% obrigatória das 6 transições.
- Regra inegociável: **1 user = 1 subscription ativa**. Plano família/equipe = PRD novo precedido de brainstorm próprio. Se a landing começar a prometer família, isto reabre.

### C. Billing (regras imutáveis)
- Webhook ingress (`/webhooks/kiwify`) faz **três** coisas e nada mais: (1) verifica assinatura/token antes de qualquer parse pesado; (2) deduplica por `(provider, external_event_id)` via `INSERT ... ON CONFLICT DO NOTHING` em `webhook_events`; (3) persiste raw payload + publica via outbox. Retorna 200 em < 2s.
- `BillingEventProcessor` (consumidor do outbox) é o **único** ponto de mutação de estado de `Subscription`. Reconciliação dispara evento sintético no mesmo pipeline; nunca atualiza estado direto.
- Estados canônicos da `Subscription`: `TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED_PENDING`, `EXPIRED`, `REFUNDED`. Mapeamento Kiwify ⇄ canônico fica em adapter, não no processor.
- `Subscription.period_length` é por plano (mensal=30d, trimestral=90d, anual=365d). **Sem** constante hardcoded.
- Idempotência por `event_id` é obrigatória em todo handler do outbox; processador ignora evento se `occurred_at` representa regressão.
- Interface `BillingProvider` única, com adapter Kiwify hoje. Adicionar Asaas/Pagar.me/Stripe = novo adapter sem mexer no processor.
- Reconciliação horária via API Kiwify nas subscriptions `ACTIVE`/`PAST_DUE` é **inegociável** no MVP. Sweep diário full = Épico E4 pós-MVP.
- `webhook_events` é event store imutável (JSONB). Nunca apagar; usar para replay e auditoria.

### D. Entitlement (regras imutáveis)
- `EntitlementService.Check(ctx, userID)` mora em `internal/billing/application` e retorna `Decision{Entitled, Reason, ExpiresAt}`.
- Cache Redis: `TTL = min(period_end - now, 1h)`. Sem TTL fixo eterno.
- Negative cache de 5min para "sem subscription" (proteção contra spam de número aleatório).
- Invalidação **síncrona** no fim do `BillingEventProcessor`, **após** commit no Postgres: ordem fixa = `Postgres → cache → notificação WhatsApp`.
- Override manual via `entitlement_overrides(user_id, granted_until, reason, granted_by)`; `IsEntitled` consulta também essa tabela.

### E. Onboarding (regras imutáveis)
- Fluxo único: Landing → Checkout Kiwify (com `?s={token}`) → Thank-you page **própria** (não Kiwify) → deep link `wa.me` com mensagem `ATIVAR <token>` pré-preenchida → bot.
- `SignupToken` é UUID v4 opaco. TTL 7 dias. Estados: `PENDING → PAID → CONSUMED`; `EXPIRED` via job de cleanup.
- Ativação atômica: `SELECT ... FOR UPDATE` no token, upsert `users` por `whatsapp_number`, vincula `subscription.user_id`, marca `CONSUMED`. Tudo na mesma transação.
- User só é criado quando `ATIVAR` chega. Antes disso, `subscription.user_id` é nullable (ou tabela `pending_subscriptions`).
- Fallback obrigatório: (1) job horário de outreach via template WhatsApp Business para tokens `PAID` há > 2h; (2) ativação por match E.164 com `whatsapp_input` registra `fallback_reason`.
- Idempotência no handler `ATIVAR`: segundo envio do mesmo token pelo mesmo número responde "já ativado"; outro número responde "código já usado".

### F. Gate LLM
- Whitelist explícita de comandos administrativos que **não** exigem entitlement: `ATIVAR <token>`, `/ajuda`, `/cancelar`, `/contato`.
- Tudo fora da whitelist sem entitlement válido responde **copy fixo + link de pagamento**, sem invocar LLM. Custo zero de token para inadimplente.
- Rate limit por usuário = backlog explícito, **não** MVP.

### G. Plataforma
- `outbox.Publisher` é a única forma de publicar eventos com durabilidade. Idempotência por `event_id` é obrigatória no consumidor.
- `events.Bus` é volátil, best-effort, mesmo processo. Não usar para eventos cuja perda corrompa estado.
- Eventos canônicos esperados: `webhook.kiwify.received`, `billing.subscription.state_changed`, `signup_token.paid`, `signup_token.consumed`, `entitlement.invalidated`.

### H. Segurança e LGPD
- Mascaramento de PII em logs (telefone, email, CPF) implementado na primeira feature de cada módulo.
- Webhook secrets em vault, nunca em env var commitado.
- URLs de webhook distintas por ambiente (dev/staging/prod) com secrets distintos.
- Rate limit em `/api/checkout-session`: 10/min por IP.
- Runbook "exclusão de dados" obrigatório desde Épico E1 (soft delete + anonimização em 30d).

### I. Roadmap inegociável
1. **Épico E1 — identity-foundation** (bloqueador): `User`, `WhatsAppNumber` VO, `Email` VO, `is_admin`, soft delete, `UserRepository` port + impl Postgres, `IsEntitled` puro, testes 100%, correção de `doc.go`/`README.md`/`AGENTS.md` para remover RBAC/JWT.
2. **Épico E2 — billing-pipeline** (paralelo após E1): webhook ingress + outbox + `BillingEventProcessor` + máquina de estados + `Subscription` + `BillingProvider` + adapter Kiwify + `EntitlementService` + reconciliação horária.
3. **Épico E3 — onboarding-magic-token** (paralelo após E1): `SignupToken` + `/api/checkout-session` + thank-you page + handler `ATIVAR` + outreach + integração com `EntitlementService.Invalidate`.
4. **Épico E4 — reconciliation-hardening** (pós-MVP): sweep diário full, alertas refinados, dashboard MRR/churn, runbook completo, rate limit por usuário.

## Decisões Pendentes
- Confirmar com produto o tamanho do time (H9): 1 dev ou 2 devs simultâneos impactam o paralelismo real entre E2 e E3.
- Confirmar com produto o volume esperado (H8) para 6 meses: se > 5k subs, revisar capacidade de Redis Streams e índices Postgres antes de E2 fechar.
- Validar propagação de `?s={token}` no webhook Kiwify (H7) com compra real R$ 1 Pix em sandbox antes do PRD do Épico E2 entrar em implementação.
- Decidir, ao abrir o PRD do Épico E3, se a thank-you page será hospedada em `mecontrola.app.br/obrigado` (landing repo) ou em domínio da API; decisão impacta CORS e deploy.
- Confirmar com Marketing/Landing o gate de substituição do `CHECKOUT_URL_*` placeholder pelos URLs reais da Kiwify (não pode acontecer antes de E2 + E3 estarem em staging com smoke test).

## Próximo Passo Recomendado
create-prd para o Épico E1 (identity-foundation) primeiro, com objetivo de materializar `User`, `WhatsAppNumber` VO, `Email` VO, `is_admin`, soft delete, `UserRepository` port, função pura `IsEntitled` e correção de drift em `internal/identity/domain/doc.go`. Após PRD de E1 aprovado, abrir em paralelo create-prd para Épicos E2 (billing-pipeline) e E3 (onboarding-magic-token), ambos consumindo este bundle como insumo de Arquitetura Inegociável. Épico E4 (reconciliation-hardening) entra como backlog pós-MVP, sem PRD imediato.
