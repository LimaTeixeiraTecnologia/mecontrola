# ADR-004 — Plano Determinístico Multi-Tool (1..N)

## Metadados

- **Título:** Execução composta determinística a partir de um plano de 1..N intents extraído no parse
- **Data:** 2026-06-24
- **Status:** Aceita
- **Decisores:** Solicitante + plataforma
- **Relacionados:** PRD (RF-12, RF-10, RF-06, RF-01), techspec §"Plano multi-tool", `R-AGENT-WF-001.1/.4`, ADR-003

## Contexto

O Documento Oficial e o uso real exigem que uma mensagem componha mais de uma ação (ex.: "paguei 50
no mercado e quanto gastei esse mês?" → registra + resumo). Hoje o `ParseInbound` retorna um único
`Intent`. O PRD pede composição **sem** loop de raciocínio LLM (RF-10) e **sem** novo `case
intent.Kind` no switch (RF-21).

## Decisão

O `ParseInbound` (único call-site LLM) passa a poder emitir um `IntentPlan` (lista ordenada de
`IntentStep`, 1..N) via campo opcional `plan` no Structured Output. O `PlanExecutor` é um **workflow
durável do kernel** (`Definition[PlanState]`): `PlanState` carrega os intents ordenados, um **cursor**
e as `Reply` acumuladas. Cada passo executa um intent pelo caminho existente
(`dispatchWrite`/`IntentRegistry.Resolve → Workflow → Tool`), com **short-circuit** em falha dura de
escrita e **agregação determinística** das respostas (junção ordenada das `Reply`). A condição de
parada é **função pura** sobre o estado (sem LLM). **Plano de 1 passo é idêntico ao fluxo atual**
(não-regressão, RF-01/07).

**Plano com passo destrutivo/sensível (HITL) — decisão "suspende o plano inteiro":** quando um passo
delega ao `destructive_confirm` e este suspende no `confirm_gate`, a durabilidade do `PlanState` faz a
suspensão **suspender o plano inteiro**. Ao confirmar (resume), o `Engine.Resume` retoma **a partir do
cursor** os passos restantes; ao cancelar/expirar, o plano encerra sem efeito nos passos pendentes. O
cursor e os steps ficam no snapshot (fonte única — R-WF-KERNEL-001.7), evitando plano órfão.

**Idempotência/replay por passo — decisão "chave por passo (migration)":** um plano com ≥2 escritas
vem de **um único `message_id`**, mas a idempotência atual é `UNIQUE(user_id, channel, message_id)`
(`agent_decisions`, migration 000011). Para não colidir nem gerar falso-replay, a chave de auditoria/
replay é estendida para **`(user_id, channel, message_id, step_index)`** via **nova migration 000021**
(altera o índice único e a tabela `agent_decisions` ganha `step_index INT NOT NULL DEFAULT 0`). A
entidade `Decision` carrega `step_index`; ações single (plano de 1) usam `step_index=0` — comportamento
idêntico ao atual. Replay passa a ser por (mensagem, passo), garantindo que reprocessar a mesma
mensagem não duplique nenhuma escrita e que o passo 2 não seja confundido com replay do passo 1.

### Migration 000021 (resumo — SQL completo no arquivo)

```sql
ALTER TABLE mecontrola.agent_decisions ADD COLUMN IF NOT EXISTS step_index INT NOT NULL DEFAULT 0;
DROP INDEX IF EXISTS mecontrola.agent_decisions_user_channel_message_uniq_idx;
CREATE UNIQUE INDEX agent_decisions_user_channel_message_step_uniq_idx
  ON mecontrola.agent_decisions (user_id, channel, message_id, step_index);
-- down: recria índice antigo (após garantir step_index=0 único por mensagem) e dropa a coluna
```

## Alternativas Consideradas

- **Loop agêntico com tool-calling iterativo** (LLM decide próximos passos): mais flexível, porém viola
  RF-10/`R-AGENT-WF-001.4`, aumenta alucinação/custo/imprevisibilidade. Rejeitada.
- **Concatenar duas chamadas LLM**: dobra custo/latência e reintroduz LLM no meio. Rejeitada.

## Consequências

### Benefícios Esperados

- UX de "agente que planeja e age" sem custo/risco de loop LLM; reuso total dos workflows/tools.

### Trade-offs e Custos

- Schema de parse mais rico; executor com agregação/short-circuit a testar; limite de passos do plano
  (parâmetro de techspec) para conter abuso.

### Riscos e Mitigações

- **Risco:** ordem/falha parcial confusa para o usuário. **Mitigação:** agregação determinística que
  sinaliza o que foi feito e o que não foi; short-circuit só em falha dura de escrita.
- **Risco:** plano com passos conflitantes. **Mitigação:** cada passo passa pelo WriteGuard/HITL
  normalmente; HITL suspende o plano inteiro e retoma do cursor. **Rollback:** desabilitar emissão de
  `plan` (volta a 1 intent).
- **Risco:** resume reconstruir cursor errado após HITL no meio do plano. **Mitigação:** cursor + steps
  + replies persistidos no snapshot (R-WF-KERNEL-001.7); cobertura por teste de durabilidade
  (suspend→restart→resume continua do cursor).

## Plano de Implementação

1. Estender schema de parse (`plan` opcional). 2. `IntentPlan`/`IntentStep`/`PlanExecutor`.
3. Integrar no `Handle` (plano 1 = caminho atual). 4. Testes: 1, N, short-circuit, agregação.

## Monitoramento e Validação

- `agent_plan_steps_total{outcome}` (cardinalidade controlada). Sucesso: 100% dos cenários de aceite
  compostos executam em ordem; single-intent sem regressão.

## Impacto em Documentação e Operação

- Runbook conversacional (exemplos de mensagem composta), prompts de parse.

## Revisão Futura

- Revisar limite de passos e política de short-circuit conforme uso real.
