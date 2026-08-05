# Tarefa 1.0: Configuração e dry-run seguro de thresholds

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar configuração production-safe para dry-run, quiet hours e validação de modo de threshold alerts, sem migration e sem envio real novo.

<requirements>
- Cobrir RF-10, RF-16, RF-17, REQ-11, REQ-13, REQ-19, REQ-20 e REQ-22.
- `BUDGETS_THRESHOLD_ALERTS_DRY_RUN` deve impedir outbox, dedup e chamada Meta quando ativo.
- Modo inválido de threshold alerts deve falhar em validação de config, não desligar fluxo silenciosamente.
- Quiet hours default: 20:00-08:00.
- Timezone fallback: `America/Sao_Paulo`.
</requirements>

## Subtarefas

- [ ] 1.1 Mapear `configs.BudgetsConfig` e testes existentes.
- [ ] 1.2 Adicionar campos de dry-run, quiet hours e timezone fallback.
- [ ] 1.3 Validar modo inválido de threshold alerts.
- [ ] 1.4 Injetar dry-run no wiring de `EvaluateThresholdAlerts`.
- [ ] 1.5 Adicionar testes de config e use case garantindo ausência de outbox/dedup em dry-run.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Configuração`, `Modelos de Domínio e Aplicação` e `Sequenciamento de Desenvolvimento`.

## Critérios de Sucesso

- Dry-run ativo não publica outbox, não insere sent e não chama gateway.
- Modo inválido falha cedo na validação.
- Defaults são explícitos e cobertos por teste.
- Nenhum comportamento de envio existente muda quando dry-run está desativado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — valida regras de dry-run, quiet hours e supressão como decisões de domínio.
- `design-patterns-mandatory` — confirma `não aplicar padrão` para flags e validação direta.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./configs/...`
- [ ] `go test -race -count=1 ./internal/budgets/application/usecases/...`
- [ ] `go vet ./configs/... ./internal/budgets/application/usecases/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `configs/config_test.go`
- `internal/budgets/module.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts_test.go`
