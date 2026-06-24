# Tarefa 4.0: Kernel caminho único — remover legacy + fallback morto

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tornar o kernel o caminho único de escrita/resume, removendo a coexistência legacy (flag
`kernelEnabled`, `EnableKernel`, `continuePendingExpenseConfirmationLegacy`, `parity_test`) e o
fallback determinístico inalcançável de configuração de orçamento — após satisfeitas as pré-condições
PRÉ-1..PRÉ-4 do ADR-006.

<requirements>
- RF-39: kernel é o caminho único; remover flag/branch/método e teste de paridade legacy, migrando previamente qualquer caso ainda atendido pelo legacy.
- RF-40: remover o fallback determinístico inalcançável de configuração de orçamento (branch morto), após confirmação de inacessibilidade.
</requirements>

## Subtarefas

- [ ] 4.1 PRÉ-1: remover a flag `WorkflowKernelConfig.TransactionsWriteEnabled` e o gating; tornar `attachKernel`/`attachBudgetConfigSession` não-opcionais (deps ausentes = falha de boot, nunca fallback silencioso).
- [ ] 4.2 PRÉ-3: drenar/expirar `pendingexpense.Draft` legacy em `agent_sessions` (housekeeping/TTL) antes do cutover.
- [ ] 4.3 PRÉ-4: garantir `parity_test` verde; migrar os casos de resume (`TestParity_*Resume`) para suite kernel-only antes de apagar.
- [ ] 4.4 Remover `EnableKernel` (morto), campo `kernelEnabled` + guardas (trocar por `a.kernelEngine != nil`), `continuePendingExpenseConfirmationLegacy` + helpers exclusivos + ramo `RunID==Nil` (`daily_ledger_agent.go:458-460`).
- [ ] 4.5 Remover `parity_test.go` + `runLegacy()` no mesmo PR da 4.4.
- [ ] 4.6 Remover fallback morto de budget: branch else em `budget_tools.go`, campo/param `BudgetConfigurator`, adapter `infrastructure/onboarding/budget_configurator.go` (preservar usecase `StartBudgetConfiguration`).
- [ ] 4.7 Gate de resíduo: `deadcode`/`staticcheck` sem unused nos símbolos removidos.

## Detalhes de Implementação

Ver `adr-006-kernel-single-path.md` (PRÉ-1..PRÉ-4, tabela de itens 1.1–1.5 e 3.1–3.4). Confirmar a
condição `RunID==uuid.Nil` (`engine.go:156-159`) inalcançável sob PRÉ-1/PRÉ-2 antes de remover.

## Critérios de Sucesso

- Caminho único: `grep EnableKernel|continuePending*Legacy|parity` vazio; gate switch ≤1 em `daily_ledger_agent.go`.
- Nenhuma espera de categoria órfã (resume sempre via kernel); suíte verde com suite kernel-only de resume.
- `deadcode`/`staticcheck` sem resíduo dos símbolos removidos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — altera o ciclo de resume Thread→Run, kernel e pending step do `internal/agent` (R-AGENT-WF-001, R-WF-KERNEL-001).

## Testes da Tarefa

- [ ] Testes unitários (resume kernel-only: auto_log, authz, replay, policy, ambiguous/needs_confirm, usecase_error, audit conflict/fail, missing_resolver).
- [ ] Testes de integração (suspend→resume durável de categoria via kernel; sem fallback legacy).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/daily_ledger_agent.go`
- `internal/agent/application/workflow/parity_test.go` (remover)
- `internal/agent/application/tools/budget_tools.go`
- `internal/agent/infrastructure/onboarding/budget_configurator.go` (remover)
- `internal/agent/module.go`, `configs/config.go` (remover flag)
