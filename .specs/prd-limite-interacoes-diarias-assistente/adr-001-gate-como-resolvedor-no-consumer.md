# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate de limite diário como resolvedor de aplicação consultado pelo consumer
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** responsável pelo produto mecontrola, agente de engenharia
- **Relacionados:** `.specs/prd-limite-interacoes-diarias-assistente/prd.md`, `.specs/prd-limite-interacoes-diarias-assistente/techspec.md`, `adr-002-contagem-via-platform-runs-com-porta-no-consumidor.md`, `docs/us/US-003-limite-30-interacoes-diarias-assistente.md`

## Contexto

O PRD exige bloqueio da 31ª interação diária antes de qualquer chamada ao LLM e antes da abertura de Run (RF-01, RF-10), com zero regressão nos fluxos de texto, áudio, deduplicação, onboarding e confirmação destrutiva. O ponto de enforce precisa respeitar a regra dura do repositório de adapters finos (R-ADAPTER-001): o consumer `whatsapp_inbound` não pode conter regra de negócio, SQL ou branching de domínio. O fluxo atual já possui dois pontos de dispatch ao agente: o caminho de texto em `Handle` (`whatsapp_inbound_consumer.go:226-230`) e o closure de dispatch do áudio transcrito (`whatsapp_inbound_consumer.go:281-286`), ambos precedidos por `tryResume`, que trata workflows suspensos e onboarding fora da cota (RF-07).

## Decisão

O gate é um use case novo `ResolveDailyInteractionLimit` em `internal/agents/application/usecases`, injetado no consumer via `ConsumerOption` (`WithDailyLimitResolver`) e consultado por um método `tryDailyLimit` posicionado imediatamente após `tryResume` e antes de `handleAgentInbound`, nos dois pontos de dispatch. O contrato retorna `DailyLimitResult{Blocked, Message}`, espelhando `OnboardingResult` (`resolve_onboarding_or_agent.go:17-21`), e o consumer apenas envia `Message` via `sendReply` existente quando bloqueado. A decisão de permitir ou bloquear é função pura `DecideDailyInteractionLimit` dentro do use case.

## Alternativas Consideradas

- Gate dentro do use case `HandleInbound`: centralizaria a regra no fluxo canônico, mas exigiria estender o contrato `agent.Outcome` (`internal/platform/agent/ports.go:95-101`) com um estado de bloqueio ou sobrecarregar `Content` com a mensagem de limite, poluindo um contrato do substrato com regra de produto. Rejeitada por acoplar produto ao substrato e por dificultar a distinção métrica entre bloqueio e execução.
- Gate no `AgentRuntime.Execute` do platform (`internal/platform/agent/runtime.go:100-172`): centralizaria para qualquer consumidor do runtime, mas o kernel de plataforma é genérico e não pode conter regra de negócio de um consumidor (R-AGENT-WF-001 e regras de plataforma compartilhada). Rejeitada por violação de fronteira.
- Middleware HTTP ou gateway de webhook antes da fila: o inbound chega por evento outbox, não por HTTP síncrono; bloquear na borda impediria a resposta estática ao usuário, que depende do gateway de envio disponível no consumer. Rejeitada por inviabilidade de responder ao usuário e por quebrar a deduplicação existente.
- Gate antes do pipeline de áudio: economizaria o custo de transcrição de quem está acima do limite, mas exigiria consulta adicional no início de `handleAudioInbound` e mudança no pipeline de áudio, que o PRD manda preservar intacto (RF-13). Rejeitada nesta fatia, com perda registrada.

## Consequências

### Benefícios Esperados

- Consumer permanece adapter fino, passando no gate `task ci:agent-boundary` sem exceção nova.
- Padrão estrutural idêntico ao resolvedor de onboarding, já validado em produção e coberto por testes de ordenação (`TestResumeDispatcherPrecedesOnboarding`, `whatsapp_inbound_consumer_test.go:953-988`).
- Bloqueio ocorre antes de Run e de LLM, atendendo RF-01 e RF-10 com custo zero de provedor.
- Cobertura dos dois pontos de dispatch (texto e áudio transcrito) com um único resolvedor.

### Trade-offs e Custos

- Dois call sites de `tryDailyLimit` no consumer (texto e closure de áudio), mesma forma já existente de `tryResume`; custo de duplicação mínima aceito.
- Áudio acima do limite ainda paga transcrição antes do bloqueio, perda deliberada registrada no PRD.
- Mensagem de bloqueio não gera Run nem registro em `platform_runs`; a auditoria do bloqueio depende da métrica e do log, não de linha em tabela.

### Riscos e Mitigações

- Risco: ordem errada na cadeia (gate antes de `tryResume`) bloquearia respostas a workflows suspensos. Mitigação: teste de ordenação dedicado no padrão do teste de precedência existente.
- Risco: resolvedor ausente (option não configurada) em algum entrypoint. Mitigação: wiring obrigatório em `module.go` junto às options já obrigatórias (`WithOnboardingResolver`, `WithResumeDispatcher`, `module.go:418-419`); comportamento fail safe do consumer quando a option é nil deve seguir o padrão atual das demais options opcionais, documentado no teste.
- Rollback: `AGENT_DAILY_INTERACTION_LIMIT=0` desativa o gate sem deploy, decidido na rodada de clarificação.

## Plano de Implementação

1. Criar `DailyLimitResult`, `dailyInteractionCounter`, `ResolveDailyInteractionLimit` e `DecideDailyInteractionLimit` em `internal/agents/application/usecases`.
2. Adicionar `dailyLimitResolver`, `WithDailyLimitResolver` e `tryDailyLimit` ao consumer, posicionando a chamada após `tryResume` nos dois pontos de dispatch.
3. Wiring em `internal/agents/module.go` e repasse de config em `cmd/server/server.go` e `cmd/worker/worker.go`.
4. Critério de conclusão: testes de ordenação e bloqueio verdes, `task ci:agent-boundary` sem violação, zero alteração de comportamento nos cenários existentes da suite do consumer.

## Monitoramento e Validação

- Métrica `agents_daily_limit_total` com label `outcome` (`allowed` ou `blocked`), sem identificadores de usuário.
- Log `Info` por bloqueio e `Error` com wrap `%w` em falha de contagem.
- Critério de sucesso: 100% das mensagens acima do limite bloqueadas sem chamada ao LLM, auditável pela métrica e pela ausência de novos Runs; taxa de bloqueio acompanhada para calibragem futura do limite.

## Impacto em Documentação e Operação

- `.env.example` e `deployment/config/prod.env` documentam `AGENT_DAILY_INTERACTION_LIMIT` e `WA_MSG_DAILY_LIMIT_REACHED`.
- Runbook de operação do assistente deve registrar o knob de desativação por valor zero.

## Revisão Futura

- Revisar ao introduzir um segundo agente no mesmo `resource_id`, ao adotar limites por plano, ou se a taxa de bloqueio indicar limite mal calibrado.
