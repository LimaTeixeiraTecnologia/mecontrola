# Tarefa 8.0: Plano multi-tool 1..N + idempotência por passo (migration 000021)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Permitir que uma mensagem componha um plano determinístico de 1..N intents, executado pelo caminho
existente, com short-circuit e agregação determinística. Plano com passo destrutivo suspende o plano
inteiro (workflow durável do kernel) e retoma do cursor. Idempotência por passo via migration 000021
(`step_index`). Plano de 1 passo é idêntico ao fluxo atual (não-regressão).

<requirements>
- RF-09: agente mapeia a saída estruturada 1:1 para a porta de entrada (sem reinterpretar/recalcular).
- RF-10: execução determinística — proibido LLM/prompt/fallback durante a execução dos passos.
- RF-12: plano determinístico de 1..N ações; short-circuit em falha dura; agregação determinística; plano de 1 = comportamento atual.
</requirements>

## Subtarefas

- [ ] 8.1 Estender o schema de parse com `plan: [{...}]` opcional (ausência = plano de 1, idêntico ao atual); `IntentStep`/`IntentPlan` (tipos fechados).
- [ ] 8.2 `PlanExecutor` como `Definition[PlanState]` do kernel (`Steps`, `Cursor`, `Replies`); cada passo roda via `dispatchWrite`/`IntentRegistry.Resolve → Workflow → Tool`.
- [ ] 8.3 Durabilidade condicional (eficiência): `Durable=true` só quando o plano contém ≥1 passo de escrita/destrutivo (decisão pura sobre `intent.Kind.IsWrite()`); plano só-leitura executa em memória.
- [ ] 8.4 Passo destrutivo no plano: delega ao `destructive_confirm`; suspensão suspende o plano inteiro; `Resume` continua do cursor; cancelar/expirar encerra sem efeito nos pendentes.
- [ ] 8.5 Short-circuit em falha dura de escrita; agregação determinística das `Reply` na ordem; condição de parada é função pura (sem LLM).
- [ ] 8.6 Migration `000021_agent_decisions_step_index` (up/down): adicionar `step_index INT NOT NULL DEFAULT 0`; trocar índice único `(user_id,channel,message_id)` → `(user_id,channel,message_id,step_index)`; entidade `Decision` carrega `step_index` (single = 0).
- [ ] 8.7 Não-regressão: plano de 1 passo bate o comportamento single-intent atual.

## Detalhes de Implementação

Ver `adr-004-multi-tool-plan.md` (PlanState, suspensão do plano, migration 000021) e techspec §"Plano
multi-tool 1..N". DMMF: `IntentPlan`/`PlanState` tipos fechados; parada/seleção puras.

## Critérios de Sucesso

- `paguei 50 no mercado e quanto gastei?` executa 2 passos em ordem com agregação; single-intent sem regressão.
- `apaga o uber e quanto gastei?` suspende no HITL e retoma do cursor após "sim"; replay por `step_index` não duplica.
- Plano só-leitura não cria snapshot (eficiência verificada).
- `agent_plan_steps_total{outcome}` com cardinalidade controlada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — adiciona workflow executor durável e idempotência por passo no `internal/agent` (R-AGENT-WF-001, R-WF-KERNEL-001.7).

## Testes da Tarefa

- [ ] Testes unitários (PlanExecutor: plano 1 não-regressão, plano N ordem, short-circuit, agregação; durabilidade condicional por IsWrite; replay por step_index).
- [ ] Testes de integração (plano com passo destrutivo: suspend→restart→resume continua do cursor; migration 000021 up/down).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/usecases/parse_inbound.go`, `application/prompting/prompts.go` (schema plan)
- `internal/agent/application/workflow/` (PlanExecutor/PlanState), `application/services/daily_ledger_agent.go`
- `internal/agent/domain/entities` (Decision + step_index), `infrastructure/repositories/postgres/agent_decision_repository.go`
- `migrations/000021_agent_decisions_step_index.{up,down}.sql` (novos)
