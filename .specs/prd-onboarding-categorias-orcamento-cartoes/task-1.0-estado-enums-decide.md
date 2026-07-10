# Tarefa 1.0: Estado, enums fechados e `Decide*` puros (rename orçamento mensal)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estabelecer a base pura (sem IO) do refactor: renomear a semântica de renda líquida para orçamento
mensal no estado durável, redefinir os enums fechados de fase, introduzir o sub-estado fechado do step
de review e ajustar as funções `Decide*` afetadas. Nenhuma mudança de fluxo/step ainda.

<requirements>
- RF-35: modelo de estado usa semântica de orçamento mensal (rename `IncomeCents`→`MonthlyBudgetCents`, tag JSON `monthlyBudgetCents`; `DecideIncomeCents`→`DecideMonthlyBudgetCents`, mesma regra `> 0`).
- RF-36: estados/fases permanecem tipos fechados e parseáveis; nova fase não pode ser string solta.
- RF-43: rename sem shim de compatibilidade (D-06); campo `ResumeText` (tag `resumeText`) preservado.
</requirements>

## Subtarefas

- [ ] 1.1 Redefinir `OnboardingPhase` para o conjunto `PhaseWelcome, PhaseGoal, PhaseMonthlyBudget, PhaseBudgetReview, PhaseActivation, PhaseRecurrence, PhaseCards, PhaseConclusion` (`iota+1`); atualizar `String()`, `IsValid()`, `ParseOnboardingPhase`. Nota: `PhaseWelcome`/`PhaseGoal`/`PhaseCards`/`PhaseConclusion` já existem; as NOVAS são `PhaseMonthlyBudget/PhaseBudgetReview/PhaseActivation/PhaseRecurrence`; REMOVER `PhaseMonthlyIncome/PhaseMethodology/PhaseDistribution/PhaseSummary`.
- [ ] 1.2 Adicionar enum fechado `reviewAwaitKind` (`reviewAwaitDistribution`, `reviewAwaitConfirm`, `iota+1`) com `String()`/`IsValid()`/`parseReviewAwaitKind` (uso interno ao pacote).
- [ ] 1.3 Renomear campo de estado `IncomeCents`→`MonthlyBudgetCents` (tag `monthlyBudgetCents`); adicionar `ReviewAwait reviewAwaitKind` (tag `reviewAwait`); remover `CardNickname`/`CardDueDay` (mortos após remover cartão do resumo).
- [ ] 1.4 Renomear `DecideIncomeCents`→`DecideMonthlyBudgetCents` (regra `amountBRL > 0`) e atualizar **todos os leitores de `state.IncomeCents`** e parâmetros `incomeCents` das funções existentes (`DecideAllocationKind`, `DecideAllocationsBP`, e os steps ainda não removidos `BuildMethodologyStep`/`BuildDistributionStep`/`BuildSummaryStep`/`BuildConclusionStep` que leem `state.IncomeCents`) para `MonthlyBudgetCents`/`monthlyBudgetCents`, garantindo que o módulo **compile** ao final desta tarefa.
- [ ] 1.5 Reescrever as strings de "renda" internas ao domínio puro: em `DecideAllocationsBP`, as mensagens de erro "não consegui usar sua renda…" e "…precisa ser igual à sua renda…" passam a usar "orçamento mensal" (M-02).
- [ ] 1.6 Atualizar/renomear testes puros afetados (`TestDecideIncomeCents`→`TestDecideMonthlyBudgetCents`, `TestParseOnboardingPhase`) e o merge-patch test (`TestOnboardingState_MergePatch_*`) para preservar `MonthlyBudgetCents`/`ReviewAwait`.

## Detalhes de Implementação

Ver `techspec.md` seções "Modelos de Dados" e "Design de Implementação". Enums seguem DMMF
state-as-type (zero value inválido). Nenhuma chamada de IO nesta tarefa.

## Critérios de Sucesso

- `go build ./...` e `go vet ./...` verdes no módulo `internal/agents`.
- `go test ./internal/agents/... -run 'Decide|ParseOnboardingPhase|MergePatch' -count=1` verde.
- `grep -rniE "IncomeCents|\brenda\b" internal/agents/application/workflows/onboarding_workflow.go` retorna vazio (M-02 amplo: qualquer "renda", não só "renda líquida"/"renda mensal"; o módulo deve compilar ao fim da tarefa).
- Nenhum enum representado como string solta; zero comentários em Go de produção (R-ADAPTER-001.1).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — modelagem de estado como tipo fechado (DMMF state-as-type) para `OnboardingPhase` e `reviewAwaitKind` com invariantes explícitas.
- `mastra` — o estado é do workflow durável do consumidor `internal/agents`; preservar contrato de snapshot/merge-patch do substrato.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — estado, enums, `Decide*`.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes puros.
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — ajustar o campo `IncomeCents` do harness real-LLM (o arquivo de resume Postgres não referencia a chave).
