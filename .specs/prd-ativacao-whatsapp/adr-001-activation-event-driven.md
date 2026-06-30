# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Ativação por WhatsApp event-driven via consumer do onboarding
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Plataforma / autor da feature
- **Relacionados:** PRD `.specs/prd-ativacao-whatsapp/prd.md` (RF-21..RF-29), techspec `.specs/prd-ativacao-whatsapp/techspec.md`, `.claude/rules/go-adapters.md` (R-ADAPTER-001)

## Contexto

A ativação por WhatsApp está 0% wired em produção: `dispatcher.go:141` classifica a mensagem e retorna `OutcomeNoRoute`, e `inbound_handler.go:37` descarta o resultado. O domínio (`ConsumeMagicToken`, `SubscriptionBindingService`, `WhatsAppMessageProcessor`) está pronto, mas nada o invoca.

Precisamos conectar a primeira mensagem de um número não vinculado ao consumo da Activation Session. O `dispatcher` é um adapter de plataforma (`internal/platform/whatsapp`), regido por R-ADAPTER-001 (adapter fino, sem regra de domínio, sem branching de domínio). Já existe o precedente do `agentRoute`: um callback injetado pelo módulo `agents` que publica `agents.whatsapp.inbound.v1` no outbox, consumido por `WhatsAppInboundConsumer`.

## Decisão

A ativação é **event-driven**:

1. O `dispatcher`, no ramo `ErrUnknownUser`, invoca um callback injetado `activationRoute(ctx, msg) RouteOutcome` que **apenas publica** o evento `onboarding.activation.attempted.v1` (`{peer_e164, text, message_id}`) no outbox. O dispatcher permanece genérico — não conhece onboarding.
2. Um novo `ActivationAttemptConsumer` (adapter fino em `internal/onboarding/infrastructure/messaging/database/consumers`) consome o evento e delega ao usecase `ActivateFromInbound`, que orquestra correlação por telefone → fallback por token → bind+consume → no-match (com throttle durável). A ativação publica `onboarding.subscription_bound`.
3. As **boas-vindas são desacopladas**: um `WelcomeConsumer` idempotente consome `onboarding.subscription_bound` (que carrega `peer_e164`+`user_id`) e envia as 2 mensagens com retry próprio — separa a entrega da ativação e evita boas-vindas parcial.
4. O ramo `MatchActivationCommand` (`ATIVAR <token>`) é removido do dispatcher (RF-29).

Escopo: roteamento inbound de número não vinculado e wiring do consumer. Impacta `internal/platform/whatsapp`, `internal/onboarding`, `cmd/server/whatsapp_wiring.go`.

## Alternativas Consideradas

- **Chamada síncrona no dispatcher** (`processor.HandleActivation` direto em `Route`): + menor latência, − injeta domínio do onboarding no adapter de plataforma (viola R-ADAPTER-001 e inverte o layering `platform→onboarding`), − acopla dispatcher a onboarding, − mais difícil de testar isoladamente, − sem durabilidade/retry. Rejeitada.
- **Reusar o consumer de agente** (`agents.whatsapp.inbound.v1`): − número não vinculado nunca passa por `establish` com sucesso, logo nunca chega ao agentRoute; misturaria ativação com roteamento de agente. Rejeitada.
- **Novo handler HTTP dedicado**: − duplica o webhook Meta já existente. Rejeitada.

## Consequências

### Benefícios Esperados

- Preserva R-ADAPTER-001 e o layering; dispatcher continua genérico (simétrico ao `agentRoute`).
- Durabilidade e retry pelo outbox; idempotência natural pela máquina de estado do token.
- Lógica de correlação testável em isolamento no usecase.

### Trade-offs e Custos

- Latência adicional do tick do outbox (~500ms default) entre "Oi" e boas-vindas — bem dentro dos 30s do PRD.
- Um evento e um consumer novos para manter.

### Riscos e Mitigações

- **Spam de número não vinculado** (sem rate-limit por `UserID`): dedup por WAMID antes do publish + **store durável de throttle por telefone** (1 resposta de no-match por janela, com housekeeping), que também idempotentiza a resposta sob reentrega.
- **Concorrência multi-instância**: `UpdateMarkConsumed` com guard `WHERE status='PAID'` + checagem de `RowsAffected==0` → `AlreadyActive`.
- **Reentrega at-least-once**: `BindAndConsume` idempotente (`PAID→CONSUMED`; replay → `AlreadyActive`); boas-vindas suprimidas em `AlreadyActive`.
- Rollback: remover o registro do EventHandler e o callback restaura o comportamento anterior (sem ativação), sem migration a reverter.

## Plano de Implementação

1. Definir evento `onboarding.activation.attempted.v1` (allowlist do outbox se aplicável).
2. Criar usecase `ActivateFromInbound` + consumer.
3. Injetar `activationRoute` no `dispatcher.New` e expor `WhatsAppActivationRoute` no onboarding module.
4. Registrar o EventHandler e remover o ramo `ATIVAR`.
5. Critério de conclusão: e2e "Oi → ativado → boas-vindas" verde e idempotência em reentrega comprovada.

## Monitoramento e Validação

- `onboarding_activation_attempt_total{outcome}` por resultado.
- Run auditável (`message_id`, `outcome`, `duration_ms`) sem labels de alta cardinalidade.
- Sucesso: ativações concluídas < 30s; zero ativação dupla em reentrega.
- Revisar se a taxa de no-match indicar spam acima do tolerável (gatilho para throttle por telefone).

## Impacto em Documentação e Operação

- Runbook do onboarding: novo evento e consumer, caminho de no-match e como suporte reativa.
- Dashboards: painel de funil de ativação por outcome.

## Revisão Futura

Revisitar se for introduzido rate-limit por telefone, se o volume de inbound não-vinculado crescer, ou se a latência do outbox passar a importar para a UX.
