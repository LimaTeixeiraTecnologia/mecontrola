# Tarefa 6.0: Roteamento HITL no agent + resume antes do parse + gate de budget no commit

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Conectar o HITL ao fluxo do agent: rotear os 4 kinds destrutivos para o workflow `destructive_confirm`
via registry (sem crescer `switch`), adicionar `continuePendingApproval` (resume **antes** do
`ParseInbound`) e inserir o gate de budget no ponto de commit (ADR-004), preservando a coleta
multi-turn existente.

<requirements>
- RF-08: as 4 operações suspendem aguardando confirmação antes de efetivar.
- RF-13: resume antes do `ParseInbound`; limpeza determinística após efetivar/cancelar/expirar.
- RF-14: sem fricção em lançamentos comuns (expense/income/card purchase seguem sem gate).
- RF-21: nenhum novo `case intent.Kind` no switch de `daily_ledger_agent.go` (registry).
- RF-22: roteamento não invoca LLM nem regra/SQL/branching de domínio.
</requirements>

## Subtarefas

- [ ] 6.1 Registry keyed-by-kind dos 4 kinds destrutivos → workflow HITL; `dispatchWrite` despacha por registry (sem novo `case`).
- [ ] 6.2 `continuePendingApproval`: tentar `Engine.Resume(destructive_confirm, key, {"ResumeText": texto})` antes do parse; ordem determinística (categoria → aprovação → parse); expirado retorna `handled=false` para parsear o novo texto.
- [ ] 6.3 Gate de budget no commit: `BudgetSessionRunner` ao atingir 100% inicia o gate (`OperationBudgetCommit`) em vez de ativar direto; ativação só após confirmação.
- [ ] 6.4 Não regressão: kinds não-destrutivos e o fluxo de categoria permanecem idênticos.

## Detalhes de Implementação

Ver `techspec.md` seção "Fluxo de Dados (HITL)" e ADR-002/ADR-004. O `correlationKey` é
`"<user_id>:<channel>"`. Reusar matchers determinísticos de confirmação/cancelamento. Zero comentários.
Gate `R-AGENT-WF-001` (switch não cresce) deve retornar vazio.

## Critérios de Sucesso

- 4 kinds destrutivos suspendem na 1ª mensagem (nada efetivado) e retomam na 2ª.
- `daily_ledger_agent.go` não ganha novo `case intent.Kind` (gate R-AGENT-WF-001.1 verde).
- Budget: ao completar 100%, pede confirmação antes de `ActivateBudgetUC`; cancelar/expirar preserva o budget vigente; coleta multi-turn inalterada.
- Lançamentos comuns sem gate (não regressão).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — roteamento dos 4 kinds → HITL; `continuePendingApproval` (suspende/retoma/expira); budget gate no commit; não regressão dos kinds comuns.
- [ ] Testes de integração — não aplicável nesta tarefa (coberto em 7.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/daily_ledger_agent.go` (modificado — registry + continuePendingApproval)
- `internal/agent/application/services/agent_workflows.go` (modificado — registro do workflow HITL)
- `internal/agent/application/tools/budget_session.go` (modificado — gate no commit)
- `internal/agent/application/services/daily_ledger_agent_test.go` (estendido)
