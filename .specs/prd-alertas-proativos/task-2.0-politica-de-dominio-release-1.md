# Tarefa 2.0: Política de domínio Release 1

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Modelar e implementar a decisão pura de alertas Release 1: 80%, 100%, orçamento ausente e orçamento não revisado, com prioridade diária, opt-in e bloqueio explícito de 90%.

<requirements>
- Cobrir RF-01, RF-02, RF-03, RF-04, RF-06, RF-07, RF-08, RF-09, RF-18, REQ-07, REQ-08, REQ-09, REQ-10, REQ-12, REQ-16, REQ-17 e REQ-18.
- No máximo um alerta por usuário por rodada diária.
- `category_threshold_90` não pode ser emitido no Release 1.
- Alertas `MARKETING` exigem opt-in explícito antes de envio.
- Não criar tabela `proactive_alerts` nesta tarefa.
</requirements>

## Subtarefas

- [ ] 2.1 Mapear `ThresholdWorkflow` e VOs atuais de threshold/root slug.
- [ ] 2.2 Criar tipos fechados para kinds/supressões necessários ao Release 1.
- [ ] 2.3 Implementar política pura de prioridade e supressão diária.
- [ ] 2.4 Garantir que 90% fique bloqueado por teste, sem emissão acidental.
- [ ] 2.5 Cobrir orçamento ausente e não revisado no desenho de decisão sem migration.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Modelos de Domínio e Aplicação` e `Persistência`.

## Critérios de Sucesso

- A política é determinística, sem IO e sem LLM.
- Estados ilegais são bloqueados por tipos/validação/testes.
- Todos os motivos de supressão Release 1 são representáveis e testáveis.
- Nenhum alerta fora do Release 1 é emitido por acidente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modela kinds, estados, políticas, invariantes e erros de domínio.
- `design-patterns-mandatory` — executa gate de pattern para confirmar solução direta.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/budgets/domain/...`
- [ ] `go test -race -count=1 ./internal/budgets/application/usecases/...`
- [ ] `go vet ./internal/budgets/domain/... ./internal/budgets/application/usecases/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/domain/services/threshold_workflow.go`
- `internal/budgets/domain/valueobjects/threshold.go`
- `internal/budgets/domain/valueobjects/root_slug.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts.go`
- `internal/budgets/application/usecases/evaluate_threshold_alerts_test.go`
