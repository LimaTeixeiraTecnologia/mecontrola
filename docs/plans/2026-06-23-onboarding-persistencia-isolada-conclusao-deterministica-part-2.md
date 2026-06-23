# Plano: Persistência Isolada e Conclusão Determinística do Onboarding — Parte 2

- **Data**: 2026-06-23
- **Módulos afetados**: `internal/onboarding`, `internal/agent`
- **Skills obrigatórias**: `$go-implementation` (Etapas 1–5), `$mastra`
- **Prioridade**: Alta

---

## Objetivo

Definir a estratégia de persistência do onboarding para um MVP robusto, eficiente e production-ready, sem lacunas, sem ambiguidade e sem falso positivo na marcação de conclusão.

A regra central é:

- conclusão do onboarding é **fato de domínio persistido**, nunca inferência textual;
- histórico do onboarding fica **isolado** do storage compartilhado do agente principal;
- retomada depende apenas de **estado tipado persistido**;
- o outro agente só enxerga onboarding concluído após **commit inequívoco** do domínio.

---

## Problema Atual

Hoje o codebase separa parcialmente as responsabilidades, mas ainda existe um risco estrutural:

- `internal/agent` persiste `Thread` e `Run`, o que está correto para auditoria operacional;
- `internal/onboarding` já possui `onboarding_sessions`, que guarda o estado funcional do onboarding;
- porém o onboarding LLM também usa `mecontrola.agent_sessions` para `recent_turns` e `pending_action`.

Isso cria um ponto de colisão porque `agent_sessions` já é compartilhado com outros fluxos do agente principal:

- histórico conversacional geral;
- clarificação de categoria;
- budget session e outros estados transitórios.

Consequência: onboarding e agente principal podem disputar a mesma sessão por `(user_id, channel)`.

---

## Decisão Arquitetural

### 1. Fonte canônica do onboarding

A única fonte de verdade sobre lifecycle do onboarding deve ser `mecontrola.onboarding_sessions`.

Regras:

- `state != active` significa onboarding ainda não concluído;
- `state = active` significa onboarding concluído;
- `completed_at` deve existir quando `state = active`.

`Thread` e `Run` permanecem em `internal/agent`, mas apenas como trilha operacional e auditável. Eles **não** definem estado de negócio.

### 2. Histórico isolado

O onboarding **não deve usar** `mecontrola.agent_sessions`.

O histórico necessário para o `RunOnboardingTurn` deve ficar em `onboarding_sessions.payload`, junto do estado do onboarding.

Campos novos recomendados no payload:

- `recent_turns`
- `welcome_sent_at`
- `completed_at`

`recent_turns` deve ser curto, bounded e exclusivo do onboarding.

### 3. Conclusão determinística

A conclusão do onboarding só pode acontecer quando o domínio confirmar todos os critérios:

- objetivo preenchido;
- renda válida;
- distribuição válida;
- primeira transação registrada.

A promoção para concluído deve ocorrer via `CompleteOnboardingSession`, no mesmo write transacional que:

- marca `state = active`;
- grava `completed_at`;
- publica `OnboardingCompleted`.

### 4. Handoff para o outro agente

O outro agente não deve inferir onboarding concluído por:

- texto da conversa;
- phase;
- ausência de mensagens;
- working memory parcial.

O handoff só deve ocorrer por um destes sinais confiáveis:

- `onboarding_sessions.state = active`;
- `completed_at` preenchido;
- evento `OnboardingCompleted`;
- working memory sintetizada **apenas depois** da conclusão.

---

## Mudanças Necessárias

### Fase 1 — Tornar `onboarding_sessions` autossuficiente

Expandir `OnboardingSessionPayload` para incluir:

- `RecentTurns []OnboardingTurn`
- `WelcomeSentAt`
- `CompletedAt`

`OnboardingTurn` deve ser tipado e mínimo:

- `Role`
- `Text`
- `OccurredAt`

O objetivo é permitir retomada e contexto curto do onboarding sem depender de `agent_sessions`.

### Fase 2 — Remover dependência do onboarding em `agent_sessions`

Substituir no fluxo do onboarding:

- leitura e escrita de `recent_turns`;
- uso de `pending_action` como estado transitório do onboarding

por um gateway próprio sobre `onboarding_sessions`.

O agente principal continua usando `agent_sessions` para os fluxos já existentes, sem contaminação do onboarding.

### Fase 3 — Endurecer a conclusão

Atualizar `CompleteOnboardingSession` para:

- persistir `completed_at` junto de `state=active`;
- tratar `state=active` sem `completed_at` como drift inválido;
- publicar evento apenas após persistência bem-sucedida.

### Fase 4 — Working memory somente após conclusão

A síntese de working memory deve continuar existindo, mas somente depois da conclusão inequívoca.

Isso evita que o agente principal opere sobre:

- objetivo parcial;
- renda provisória;
- split ainda não confirmado;
- cartões incompletos.

---

## Estrutura de Persistência Recomendada

### `onboarding_sessions.payload`

Campos obrigatórios para o MVP:

- `objective`
- `income_cents`
- `cards`
- `custom_split`
- `first_tx_recorded`
- `phase`
- `recent_turns`
- `welcome_sent_at`
- `completed_at`

### `agent_sessions`

Permanece exclusivo para:

- `pending_action` do agente principal;
- `recent_turns` do conversacional geral;
- drafts e suspensões que pertencem ao `internal/agent`.

Regra: onboarding não grava nem lê daqui.

---

## Regras Operacionais Inegociáveis

1. O onboarding nunca será marcado como concluído por heurística de texto.
2. O outro agente nunca decidirá "onboarding finalizado" com base em memória conversacional.
3. `state=active` sem `completed_at` é inconsistência e deve ser tratada como drift explícito.
4. Histórico do onboarding deve ser isolado do histórico conversacional geral.
5. O caminho de conclusão deve ser determinístico, transacional e auditável.
6. É obrigatório usar `$go-implementation` e `$mastra` na implementação desta parte 2.

---

## Definition of Done (DoD)

- [ ] `mecontrola.onboarding_sessions` é a única fonte de verdade do lifecycle do onboarding.
- [ ] O onboarding não lê nem grava `recent_turns` em `mecontrola.agent_sessions`.
- [ ] O onboarding não lê nem grava estado transitório próprio em `mecontrola.agent_sessions`.
- [ ] `OnboardingSessionPayload` persiste os campos mínimos necessários para retomada e auditoria funcional:
  - [ ] `phase`
  - [ ] `objective`
  - [ ] `income_cents`
  - [ ] `cards`
  - [ ] `custom_split`
  - [ ] `first_tx_recorded`
  - [ ] `recent_turns`
  - [ ] `welcome_sent_at`
  - [ ] `completed_at`
- [ ] O histórico persistido do onboarding é bounded e isolado, mantendo no máximo a janela definida para contexto do `RunOnboardingTurn`.
- [ ] `CompleteOnboardingSession` persiste `state=active` e `completed_at` no mesmo fluxo transacional.
- [ ] `CompleteOnboardingSession` nunca conclui onboarding sem todos os pré-requisitos de domínio satisfeitos.
- [ ] O evento `OnboardingCompleted` só é publicado após persistência bem-sucedida do estado concluído.
- [ ] O agente principal não depende de heurística textual, ausência de mensagens ou memória compartilhada para inferir onboarding concluído.
- [ ] O handoff para o outro agente ocorre apenas por sinal determinístico:
  - [ ] `state=active`
  - [ ] `completed_at` preenchido
  - [ ] evento `OnboardingCompleted`
  - [ ] working memory sintetizada apenas após conclusão
- [ ] Retrying do greeting proativo não duplica saudação quando `welcome_sent_at` já estiver preenchido.
- [ ] Existe cobertura automatizada para isolamento, retomada, conclusão determinística e retry idempotente.
- [ ] A implementação segue obrigatoriamente `$go-implementation` e `$mastra`.

---

## Critérios de Aceite

1. **Isolamento de persistência**
   Após executar o onboarding conversacional, `mecontrola.agent_sessions` permanece sem histórico ou estado do onboarding, e `mecontrola.onboarding_sessions` contém sozinho o contexto necessário para retomar o fluxo.

2. **Retomada sem lacuna**
   Se o usuário parar no meio do onboarding e voltar depois, o sistema retoma exatamente da `phase` persistida, com os dados já coletados preservados, sem reiniciar etapas já confirmadas.

3. **Conclusão sem falso positivo**
   O onboarding não é marcado como concluído enquanto faltar qualquer requisito obrigatório de domínio, incluindo a primeira transação registrada.

4. **Conclusão determinística**
   Quando o usuário registra a primeira transação com todos os demais dados válidos já persistidos, o sistema:
   - promove `state` para `active`
   - preenche `completed_at`
   - publica `OnboardingCompleted`
   Tudo isso sem depender de interpretação textual posterior.

5. **Pós-conclusão sem reabertura indevida**
   Depois de concluído, uma nova mensagem do usuário não reabre onboarding e é tratada pelo fluxo normal do agente principal.

6. **Idempotência do greeting proativo**
   Se o evento assíncrono que dispara a primeira saudação for reprocessado, o usuário não recebe saudação duplicada quando `welcome_sent_at` já estiver persistido.

7. **Handoff seguro para o outro agente**
   O outro agente só passa a considerar o onboarding encerrado após encontrar `state=active` com `completed_at` preenchido, ou após consumir `OnboardingCompleted`. Não é aceito inferir conclusão por texto, phase ou histórico conversacional.

8. **Drift explicitado**
   Se existir registro com `state=active` e `completed_at` ausente, o sistema não trata isso silenciosamente como sucesso; deve registrar drift ou inconsistência de forma explícita.

9. **Falha de LLM sem corrupção de estado**
   Falhas no LLM durante onboarding não promovem conclusão, não apagam progresso já válido e não contaminam o agente principal.

10. **Validação final de implementação**
    A tarefa só é considerada concluída quando houver evidência de testes cobrindo:
    - isolamento entre `onboarding_sessions` e `agent_sessions`
    - retomada no meio do fluxo
    - conclusão determinística
    - não reabertura após conclusão
    - retry idempotente do greeting

---

## Critério Final

Para este MVP, "0 falso positivo" significa:

- jamais marcar onboarding concluído sem que o domínio tenha confirmado, de forma persistida e transacional, todos os pré-requisitos.

Essa é a referência obrigatória de produção para a parte 2.
