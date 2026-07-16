# Tarefa 8.0: Tools, wiring do módulo e prompt do agente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar as tools finas dos fluxos novos, reapontar as tools de escrita aos novos workflows, refazer o wiring de `internal/agents/module.go` e reescrever o prompt do `mecontrola_agent.go` consumindo o catálogo de mensagens (task 4.0) e o catálogo de ferramentas novo. As tools permanecem adapters finos (R-ADAPTER-001 / R-AGENT-WF-001.2): sem regra de negócio, SQL ou branching de domínio.

<requirements>
- RF-01: registro de despesa/receita alcançável via tools de escrita reapontadas ao `transaction-write`.
- RF-05: mensagens verbatim do catálogo (informacionais e de tom de voz).
- RF-12: guard `multi_item` preservado.
- RF-23, RF-24: detalhamento por categoria e leitura estática (cancelamento/suporte).
- RF-25: resumo por categoria reflete a edição (tool `category_detail`).
- RF-26: edição de objetivo financeiro (tool `edit_goal` → workflow `goal-edit`).
- RF-31: escrita exige `auth.Principal`; onboarding como pré-condição.
- Dependência: task 5.0 (`transaction-write`), task 6.0 (`budget-manage`, `card-manage`, `goal-edit`, `destructive-confirm`) e task 7.0 (dispatcher/registry).
</requirements>

## Subtarefas

- [ ] 8.1 Novas tools finas em `internal/agents/application/tools/`:
  - `edit_budget_total` → `BudgetPlanner.EditBudgetTotal`.
  - `category_detail` → detalhe por categoria (subcategoria→raiz; lançamentos com data/valor/subcategoria; planejado/gasto/disponível).
  - `cancel_plan_info` e `support_info` → leitura estática verbatim via campo `message` do catálogo, sem billing.
  - `edit_goal` → workflow `goal-edit`.
- [ ] 8.2 Estender `edit_entry.go` (categoria/subcategoria/forma de pagamento) e `EditEntryCommand` correspondentes.
- [ ] 8.3 Reapontar as tools de escrita aos novos workflows: `register_expense`/`register_income`/`create_recurrence` → `transaction-write`; `delete_entry`/`delete_recurrence`/`update_recurrence` → `destructive-confirm`; `create_card`/`update_card` → `card-manage`; `create_budget`/`adjust_allocation` → `budget-manage`.
- [ ] 8.4 Rewire de `internal/agents/module.go`: engines/defs/registry/reapers/`WithWriteToolSet`; remover o wiring legado das defs/continuers antigos; registrar os novos workflows e o `SuspendedRunIndex`.
- [ ] 8.5 Reescrever o prompt de `internal/agents/application/agents/mecontrola_agent.go` consumindo o catálogo de mensagens e o catálogo de ferramentas novo; preservar o guard `multi_item` (RF-12).
- [ ] 8.6 Testes unitários das tools + validação de wiring (build do módulo).

## Detalhes de Implementação

Ver `techspec.md` (RF-01, RF-05, RF-12, RF-23, RF-24, RF-25, RF-26, RF-31 e seção "Remoção Total do Legado" — subseção "Modificar (não deletar)", que lista tools reapontadas, o rewire de `module.go` e a reescrita do prompt) e `adr-003-message-catalog.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave:
- Tool é adapter fino (R-AGENT-WF-001.2): valida o input contra o schema, mapeia para o DTO/command do usecase, invoca binding/client e mapeia o retorno; zero regra de negócio, SQL ou branching de domínio.
- `cancel_plan_info`/`support_info` retornam texto estático verbatim do catálogo (sem tocar billing).
- Escrita exige `auth.Principal` (RF-31); onboarding é pré-condição para escrita.
- Guard `multi_item` preservado (RF-12) — invariante `TestInvariantNoFalseMultiItem`.
- Prompt consome o catálogo de mensagens (task 4.0) para tom de voz determinístico.

## Critérios de Sucesso

- Os 13 fluxos alcançáveis via tools/workflows reapontados.
- Tools informacionais retornam texto verbatim do catálogo.
- Escrita exige `auth.Principal` (RF-31); sem principal, falha.
- Módulo compila e faz wiring completo (engines/defs/registry/reapers/write tool set).
- Guard `multi_item` preservado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — tools finas, wiring do consumidor e prompt do agente.
- `design-patterns-mandatory` — gate de desenho das tools e do rewire do módulo.

## Testes da Tarefa

- [ ] Testes unitários (tools novas e `edit_entry` estendida; escrita sem `auth.Principal`)
- [ ] Testes de integração (smoke de wiring: módulo compila e resolve tools/workflows)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/*.go`
- `internal/agents/module.go`
- `internal/agents/application/agents/mecontrola_agent.go`
</content>
