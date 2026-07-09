# Tarefa 5.0: Wiring module.go + tryBudgetCreation + mensagem específica + observabilidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar o fluxo de criação de orçamento ao runtime: registrar a tool `create_budget`, construir o workflow/engine, instanciar o `BudgetCreationContinuer`, seu reaper e job, e inseri-lo na cadeia `try*` do `WhatsAppInboundConsumer` **antes** do agente. Garantir mensagem específica de indisponibilidade (distinta do `fallbackReply`) e run auditável com erro persistido.

<requirements>
- RF-25: capacidade só oferecida quando há fluxo que a execute (tool registrada + continuer ativo).
- RF-26: falha de execução → mensagem específica de indisponibilidade, distinta de "não entendi".
- RF-27: run auditável com status/outcome fechados (thread_id, run_id, workflow, tool, status, duration, erro).
- RF-29: métricas com cardinalidade controlada (proibido user_id/competence como label); série `create_budget` passa a existir.
- RF-30: erro do use case persistido de forma auditável (Snapshot.LastError/run.error não-vazios).
</requirements>

## Subtarefas

- [ ] 5.1 `module.go`: registrar `BuildCreateBudgetTool(engine, def)` em `buildFinancialTools()` (24 tools); construir `BuildBudgetCreationWorkflow(meControlaAgent, planner)` + `Engine[BudgetCreationState]`.
- [ ] 5.2 `module.go`: instanciar `BudgetCreationContinuer`, reaper (`staleAfter` 35min) + job; injetar continuer no consumer. Confirmar que `create_budget` NÃO entra em `WithWriteToolSet`.
- [ ] 5.3 `whatsapp_inbound_consumer.go`: `tryBudgetCreation(ctx, span, p)` antes do agente (por `resourceId`), com exclusão mútua vs pending-entry/confirm.
- [ ] 5.4 Mensagem específica de indisponibilidade na falha de persistência (retornada `handled=true` pelo continuer), distinta do `fallbackReply`; outcome do consumer distingue `budget_creation_error`.
- [ ] 5.5 Garantir persistência do erro (Snapshot.LastError/run.error) e labels de métrica permitidos (`workflow`, `tool`, `status`, `outcome`, `agent_id`, `channel`).
- [ ] 5.6 Testes de integração da precedência `try*` e da mensagem específica.

## Detalhes de Implementação

Ver techspec.md → "Arquitetura > Modificados", "Monitoramento e Observabilidade" e ADR-005. Precedência espelha `PendingEntryContinuer`/`DestructiveConfirmContinuer`. Não redesenhar o `fallbackReply` global (fora de escopo) — a distinção fica contida no caminho de orçamento.

## Critérios de Sucesso

- `go build`, `go vet`, `go test -race`, lint verdes no módulo agents.
- Série `agent_tool_invocations_total{tool="create_budget"}` passa a existir; nenhum label de alta cardinalidade.
- Falha de persistência devolve mensagem específica (≠ fallbackReply) e run falho com `error` não-vazio.
- Adapter/consumer finos: sem regra/SQL/branching de domínio; zero comentários.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring do runtime (registry/workflow/continuer/reaper), cadeia `try*` do consumer inbound e Run auditável com estados fechados e cardinalidade controlada.

## Testes da Tarefa

- [ ] Testes unitários do consumer (precedência `try*`, mensagem específica vs fallback).
- [ ] Testes de integração (`//go:build integration`) da precedência e persistência do erro.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go` (modificado)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (modificado)
- `internal/platform/agent/runtime.go` (referência — outcome/erro do run)
