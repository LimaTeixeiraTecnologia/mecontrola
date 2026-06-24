# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Contrato de confirmação HITL — estado fechado, semântica estrita + TTL + re-prompt único
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante (produto/eng) + plataforma
- **Relacionados:** `prd.md` (RF-08..RF-13), `techspec.md`, ADR-001, ADR-002,
  `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.7), `.claude/rules/governance.md` (DMMF)

## Contexto

O gate HITL precisa ser robusto contra erro do usuário, replay de mensagem e abandono. O fluxo é
conversacional (uma resposta livre por turno) e single-turn determinístico — sem LLM no meio
(R-AGENT-WF-001.4). É preciso um estado de espera tipado (DMMF state-as-type) e regras puras para
interpretar a resposta de confirmação.

## Decisão

1. **Estado de espera como tipo fechado:** `AwaitingApproval` (`AwaitingNone`, `AwaitingConfirm`),
   com `String()`/`IsValid()`/`Parse*`. Nunca `string` livre em assinatura pública. O passo
   `confirm` é o único call-site que transita esse estado.
2. **Semântica estrita + re-prompt único + TTL** (decisão do solicitante):
   - Aceitar **apenas** confirmação explícita (`sim`/`confirmar`/`ok`/`pode`) ou cancelamento
     explícito (`não`/`cancelar`) — reusando os matchers determinísticos existentes
     (`matchesExpenseConfirmation`/`matchesExpenseCancellation`).
   - Resposta **ambígua** re-pergunta **uma vez** (`RepromptCount` 0→1, re-suspende com o mesmo
     prompt); na **segunda** ambiguidade, **cancela** sem efeito.
   - **TTL:** o gate expira por tempo. `SuspendedAt` é gravado em UTC inline (`time.Now().UTC()`) ao
     suspender; no resume, se `now - SuspendedAt > TTL`, o gate **expira**: completa como cancelado
     **sem efeito** e devolve `handled=false`, deixando o texto novo seguir para o `ParseInbound`.
   - **Idempotência:** confirmação repetida (mesmo `messageID`) é servida por replay
     (`OutcomeReplay`), sem segunda mutação — reuso do passo `replay` da guarda.
3. **Limpeza determinística:** após efetivar/cancelar/expirar, o run **completa** (não fica
   suspenso); o housekeeping do kernel purga runs concluídos. Nenhum draft órfão.
4. **Addendum de governança:** `R-AGENT-WF-001.7` é estendida para cobrir o estado `AwaitingApproval`
   além de `AwaitingKind` (categoria), exigindo persistência do estado de espera antes de retornar
   uma clarificação/confirmação.

## Alternativas Consideradas

- **Ambíguo cancela imediatamente (sem re-prompt):** mais conservador, menos amigável. **Rejeitada**
  em favor de uma tentativa extra antes de cancelar.
- **Sem TTL (gate aberto até responder):** mais simples, porém drafts/estado pendente indefinido e
  risco operacional de runs suspensos eternos. **Rejeitada.**
- **Confirmação fuzzy/LLM:** violaria R-AGENT-WF-001.4 (LLM só no parse) e o determinismo.
  **Rejeitada.**

## Consequências

### Benefícios Esperados

- Determinístico e auditável; sem LLM no gate.
- Robusto a erro (re-prompt único), abandono (TTL) e replay (idempotência por `messageID`).
- Estado fechado evita drift de string (DMMF).

### Trade-offs e Custos

- Parametrização de TTL precisa de valor padrão sensato (parâmetro de techspec/config).
- Re-prompt único adiciona um caminho de estado (`RepromptCount`) a testar.

### Riscos e Mitigações

- **Risco:** TTL muito curto cancela confirmações legítimas; muito longo deixa estado pendente.
  **Mitigação:** valor padrão conservador (ex.: alinhado ao TTL de sessão existente) e configurável;
  métrica de expiração acompanhada.
- **Risco:** matcher estrito rejeita variações regionais de "sim/não". **Mitigação:** reuso dos
  matchers já validados em produção; re-prompt único cobre o engano.

## Plano de Implementação

1. Tipos fechados `AwaitingApproval` (+ `OperationKind`, ADR-002).
2. `confirm_gate` com as transições (confirma/cancela/ambíguo-1/ambíguo-2/expira) + testes unitários
   de cada caminho.
3. Reuso de `replay` para idempotência por `messageID`.
4. Addendum em `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.7).
5. Adoção concluída quando os 5 caminhos do gate têm teste verde e o gate de governança passa.

## Monitoramento e Validação

- `agent.hitl.{suspended,confirmed,cancelled,expired,reprompt}` com `operation`.
- Critério de sucesso: 0 efeito sob cancelamento/expiração; 0 duplicação sob replay; re-prompt único
  observável.
- Revisar TTL/semântica se a taxa de expiração/cancelamento acidental for alta.

## Impacto em Documentação e Operação

- `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.7 estendida).
- Runbook do agent: tabela de transições do gate e diagnóstico.

## Revisão Futura

- Revisitar se a Fase A introduzir gates HITL encadeados num plano multi-tool (semântica de TTL por
  passo vs por plano).
