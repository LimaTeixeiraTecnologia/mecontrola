# Tarefa 6.0: Propagar núcleo compartilhado ao budget_creation sem regressão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fazer o fluxo de criação de orçamento retroativo (`budget_creation_workflow`) consumir a nova decisão de saldo, ganhando as melhorias de over/under, tolerância e valores por extenso, sem introduzir o modo personalizar e sem regressão. Atualiza os testes de baseline impactados.

<requirements>
- RF-15: melhorias do núcleo compartilhado (saldo, tolerância, extenso) valem para budget_creation; personalizar e aviso de zero permanecem onboarding-only.
</requirements>

## Subtarefas

- [x] 6.1 Ajustar `handleBudgetDistributionSlot` para chamar `DecideDistributionBalance` antes de `DecideAllocationsBP` e renderizar a mensagem de saldo (over/under) no seu próprio reprompt (`budgetDistributionReprompt`).
- [x] 6.2 Manter `DecideBudgetDistribution` (sum=10000) como rede de segurança após a conversão.
- [x] 6.3 NÃO introduzir intenção `personalize` nem aviso de zero neste fluxo (mantém o schema/prompt compartilhado).
- [x] 6.4 Atualizar os testes de baseline impactados do budget_creation, mantendo prefixos de reprompt e o invariante 10000.

## Detalhes de Implementação

Ver `techspec.md` seção "Garantia de Não-Regressão" (NR-03) e ADR-005 (política do núcleo compartilhado, testes enumerados). Adapter fino; sem regra de domínio no kernel. Zero comentários. Esta tarefa é paralelizável com a 3.0 (arquivos distintos, ambas após 2.0).

## Critérios de Sucesso

- NR-03: nenhum caminho de aceite/valores válidos do budget_creation regride; over/under passam a informar delta correto; `DecideBudgetDistribution` continua garantindo 10000.
- Testes impactados atualizados e verdes: `budget_creation_workflow_test.go:163,183,235,254`, `budget_creation_decisions_test.go:88,111`, `budget_creation_workflow_integration_test.go:181`.
- budget_creation permanece sem modo personalizar e sem aviso de zero (RF-15).
- `go test ./internal/agents/... -race` verde no pacote afetado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — o fluxo budget_creation consome o núcleo compartilhado no substrato, preservando suspend/resume e estados fechados.
- `domain-modeling-production` — garantir que a decisão de saldo permaneça pura e compartilhada sem duplicação (DRY/DMMF).

## Testes da Tarefa

- [ ] Testes unitários: atualizar os testes de baseline do budget_creation para o novo texto de saldo, preservando prefixos de reprompt e o invariante 10000; whitebox testify/suite.
- [ ] Testes de integração: `budget_creation_workflow_integration_test.go` (distribuição parcial continua não ativando).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_creation_workflow.go` (`handleBudgetDistributionSlot`, `budgetDistributionReprompt`)
- `internal/agents/application/workflows/budget_creation_workflow_test.go`, `budget_creation_decisions_test.go`, `budget_creation_workflow_integration_test.go` (testes)
