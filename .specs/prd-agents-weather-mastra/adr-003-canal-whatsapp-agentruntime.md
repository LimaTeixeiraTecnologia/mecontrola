# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Mapeamento do canal WhatsApp → `AgentRuntime` (agente direto como caminho primário)
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma; solicitante do produto
- **Relacionados:** PRD RF-11,13,20,21,22; techspec.md; ADR-004

## Contexto

A entrada do usuário no WhatsApp é texto livre ("clima em São Paulo?"). O exemplo Mastra tem dois caminhos: o **agente** (conversa com tool `get-weather` + memória) e o **workflow** `weather-workflow` (`city`→`activities`, com agent-como-step). É preciso decidir qual atende o WhatsApp e como mapear identidades de canal para chaves opacas.

## Decisão

O **agente direto** (`AgentRuntime.Execute`) é o **caminho primário** do WhatsApp: o texto livre vira `InboundRequest.Message`; o agente decide chamar a tool `get-weather` e responde, com memória thread/working/semantic. Mapeamento de identidade: `resourceId` e `threadId` são **chaves opacas** derivadas de `user_id`/`peer` do WhatsApp (sem semântica de domínio na plataforma). O `weather-workflow` (city→activities, agent-como-step via streaming) é **registrado no kernel e exercitado pela suite de conformidade** (RF-11/13), podendo ser acionado sob demanda (ex.: intenção explícita de planejar atividades) numa evolução — não é o gatilho default do inbound. A resposta sai pelo gateway WhatsApp existente (`SendTextMessage`).

## Alternativas Consideradas

- **Workflow como caminho primário do inbound**: rejeitada agora — exigiria extrair `city` do texto livre antes do workflow (parsing determinístico/NLU) e perderia a naturalidade conversacional do agente; mantém-se para conformidade e uso sob demanda.
- **Agente sem workflow**: rejeitada — RF-11/13 exigem o workflow com agent-como-step; mantido e testado.

## Consequências

### Benefícios Esperados
- Experiência conversacional natural no WhatsApp; cobre RF-20/22 com o agente.
- Workflow preservado e testado (RF-11/13) sem acoplar ao parsing de inbound.

### Trade-offs e Custos
- Dois caminhos coexistem (agente no canal, workflow em conformidade/sob demanda).

### Riscos e Mitigações
- Streaming no workflow (agent-como-step): validar fim-de-stream/structured output (`Result(ctx)` drena `Deltas()`, fix B5) com teste de >64 deltas.
- Mapeamento de identidade: garantir opacidade (sem `user_id` como label de métrica; cardinalidade controlada).

## Plano de Implementação

1. Adapter WhatsApp inbound (consumer) → `HandleInbound` → `AgentRuntime.Execute`.
2. Derivar `resourceId`/`threadId` opacos de `user_id`/`peer`.
3. Registrar `weather-workflow` no `Engine[S]`; cobrir por conformidade.
4. Resposta via gateway existente.

## Monitoramento e Validação

- `platform_runs` com Run auditável por inbound; E2E de inbound asserindo resposta + persistência.

## Impacto em Documentação e Operação

- Runbook do fluxo WhatsApp→agente; exemplos de diálogo (clima + atividades).

## Revisão Futura

- Revisar quando houver intenção explícita de "planejar atividades" para rotear ao `weather-workflow` no canal.
