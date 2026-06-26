# Tarefa 4.0: Migração isDestructiveKind via RequiresConfirmation

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fazer `isDestructiveKind` derivar de `catalog.Lookup(kind).RequiresConfirmation` (D-06/ADR-003), injetando o catálogo no `DailyLedgerAgent`. O mapa `intentToOperationKind` **permanece** como tradução `intent.Kind → confirmation.OperationKind` (tipo fechado). Adicionar teste de consistência catálogo↔mapa e garantir não-regressão dos gates HITL — nenhuma operação destrutiva pode escapar da confirmação.

<requirements>
- RF-12: `isDestructiveKind` deriva de `RequiresConfirmation`; `OperationKind`/`intentToOperationKind` preservados; sem novo `case intent.Kind` de domínio; sem regressão HITL.
</requirements>

## Subtarefas

- [ ] 4.1 Injetar `*capability.Catalog` no `DailyLedgerAgent` (campo + wiring em `module.go`, reutilizando o catálogo de 3.0).
- [ ] 4.2 Reescrever `isDestructiveKind(kind)` para consultar `a.catalog.Lookup(kind).RequiresConfirmation`.
- [ ] 4.3 Manter `intentToOperationKind`/`resolveOperationKind` inalterados (tradução para `OperationKind`).
- [ ] 4.4 Teste de consistência: para todo kind em `intentToOperationKind`, `catalog.Lookup(kind).RequiresConfirmation == true`.
- [ ] 4.5 Executar a suíte de confirmação/HITL existente garantindo não-regressão dos 5 fluxos destrutivos.

## Detalhes de Implementação

Ver `techspec.md` → "Pontos de Integração" (Confirmation engine/HITL) e ADR-003. `isDestructiveKind` é consumido em `daily_ledger_agent.go:347` e `:519`; a troca de fonte deve preservar o comportamento exato. Conformidade R-AGENT-WF-001.1 (sem novo `case`), R-AGENT-WF-001.7-A (estado de espera/gate HITL), R-ADAPTER-001.1 (zero comentários).

## Critérios de Sucesso

- `isDestructiveKind` retorna idêntico ao mapa atual para os 5 kinds destrutivos.
- Teste de consistência catálogo↔`intentToOperationKind` verde.
- Suíte HITL verde; nenhuma operação destrutiva executa sem confirmação explícita.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera classificação de operação destrutiva/sensível e gate HITL do `internal/agent`; gatilho da skill acionado.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/daily_ledger_agent.go` (modificado: `isDestructiveKind` via catálogo)
- `internal/agent/.../module.go` (injeção do catálogo no agent)
- `internal/agent/application/services/daily_ledger_agent_test.go` (consistência + não-regressão HITL)
