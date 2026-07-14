# Tarefa 4.0: Handlers de distribuição e personalizar com persistência do sub-estado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar tudo no passo de distribuição: rotear a intenção em `handleReviewAwaitDistribution`, adicionar `handleReviewAwaitPersonalize`, preservando a extração de valores compartilhada e garantindo que o estado de espera seja persistido antes de qualquer pergunta e retomado antes do parse.

<requirements>
- RF-01: modo personalizar operante do início ao fim (entra, coleta valores, fecha).
- RF-12: sem regressão — aceite "sim" aplica default e avança; valores válidos aceitos; "não" no resumo reabre na sugestão padrão; soma inválida re-suspende sem ativação parcial.
- RF-13: estado de espera persistido de forma durável antes de responder ao usuário; retomado no resume antes do parse.
</requirements>

## Subtarefas

- [ ] 4.1 Reescrever `handleReviewAwaitDistribution`: executar o pré-classificador de intenção; `accept`→default→confirmar; `personalize`→`reviewAwaitPersonalize`+`personalizePrompt`; `values`→extração compartilhada→`DecideAllocationKind`→`DecideDistributionBalance`; `mixed_unit`→pedir unidade única.
- [ ] 4.2 Implementar `handleReviewAwaitPersonalize`: mesma coleta de valores, com reprompts no tom personalizar; over/under permanecem no sub-estado; recusa/ambíguo repete a orientação.
- [ ] 4.3 Rotear o novo sub-estado em `BuildBudgetReviewStep` (`switch state.ReviewAwait`), mantendo `reviewAwaitDistribution`/`reviewAwaitConfirm` inalterados.
- [ ] 4.4 Garantir que todo caminho que pede input use `suspendStep` (persiste o `Snapshot` antes de responder) e que o resume aplique merge-patch antes do parse (R-AGENT-WF-001.7).

## Detalhes de Implementação

Ver `techspec.md` seções "Design de Implementação" (assinaturas dos handlers), "Garantia de Não-Regressão" (NR-02/04/05) e "Fluxo de dados"; ADR-001/ADR-002. Adapter fino (R-ADAPTER-001.2): decisão pura fora do handler; sem SQL/branching de domínio no kernel. Zero comentários.

## Critérios de Sucesso

- NR-02: caminho `values` usa a extração compartilhada inalterada; comportamento idêntico ao atual para entradas válidas.
- NR-04: over/under continua re-suspendendo no mesmo sub-estado sem ativar; só o texto muda.
- NR-05: "não" no passo de confirmação do resumo reabre a distribuição na sugestão padrão (`methodologyPrompt`), preservando `onboarding_workflow_test.go:1386`; personalizar não é reaberto automaticamente.
- RF-13: nenhum caminho pede input sem antes suspender/persistir o estado; resume aplica merge-patch antes do parse.
- Cenários de baseline de resume (`onboarding_workflow_test.go:1247,1291,1331,1386`) permanecem verdes (atualizados apenas onde o texto de saldo mudou).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — handlers do workflow durável no consumidor; suspend/resume, roteamento por estado fechado, sem branching de domínio no kernel.
- `domain-modeling-production` — manter a decisão pura fora do handler; estado de espera como tipo fechado persistido no snapshot.

## Testes da Tarefa

- [ ] Testes unitários: roteamento por intenção (accept/personalize/values/mixed_unit), `handleReviewAwaitPersonalize` (over/under permanece, recusa repete), persistência antes de suspender. Package whitebox testify/suite com `agent.Agent` e `BudgetPlanner` mockados; atualizar os cenários de baseline impactados.
- [ ] Testes de integração: coberto na tarefa 7.0 (resume Postgres do novo sub-estado).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (`BuildBudgetReviewStep`, `handleReviewAwaitDistribution`, `handleReviewAwaitPersonalize`)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (testes de handler/baseline)
