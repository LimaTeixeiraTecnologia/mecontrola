# Tarefa 3.0: Step fundido `budget_review` (submáquina) + step `activation`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fundir metodologia + distribuição + resumo num único step durável `budget_review` com submáquina
interna fechada (`reviewAwaitKind`), permitindo que "não" no resumo reabra a distribuição (D-09).
Extrair a ativação para um step próprio `activation`, que só ocorre após a confirmação (RF-22).

<requirements>
- RF-16: sugestão de distribuição usa o orçamento mensal como total.
- RF-17: distribuição aceita 3 modos (aceitar padrão / reais / percentual).
- RF-18: soma em reais que fecha → basis points; que não fecha → pede correção, sem ativar parcial.
- RF-19: distribuição resultante contém as 5 categorias, sem ausência/negativo.
- RF-20, RF-21: resumo exibe objetivo, orçamento mensal e distribuição; sem "renda mensal" e sem cartão.
- RF-22: ativação só após confirmação explícita.
- RF-23: "não"/ambíguo no resumo reabre a distribuição e volta ao resumo, sem ativar parcial (D-09).
</requirements>

## Subtarefas

- [ ] 3.1 `BuildBudgetReviewStep`: submáquina `reviewAwaitDistribution`↔`reviewAwaitConfirm`; primeira entrada → `SuggestAllocation(default)` → suspend(sugestão).
- [ ] 3.2 Resume em `reviewAwaitDistribution`: extrair alocação (`allocation_input`) → `DecideAllocationKind`/`DecideAllocationsBP`; erro → reprompt (mesmo sub-estado); sucesso → `applyDraftBudget` → `SuggestAllocation(atual)` → suspend(resumo), `reviewAwaitConfirm`.
- [ ] 3.3 Resume em `reviewAwaitConfirm`: "sim" → completa; "não"/ambíguo → volta a `reviewAwaitDistribution` (suspend sugestão) — D-09.
- [ ] 3.4 Helper `applyDraftBudget` (recria draft): `GetMonthlySummary`→se draft existe `DeleteDraftBudget`→`CreateBudget` com `TotalCents = MonthlyBudgetCents`.
- [ ] 3.5 `BuildActivationStep`: `ActivateBudget(competence)` idempotente (tolera `ErrBudgetAlreadyActive`); competência calculada inline (D-13, status quo).
- [ ] 3.6 Remover `BuildMethodologyStep`/`BuildDistributionStep`/`BuildSummaryStep` isolados; migrar prompts para o `budget_review`, eliminando TODA menção a "renda" (M-02 amplo): `methodologyPrompt` ("com base na sua renda…"), `summaryPrompt` ("Renda mensal…", e remover a linha de cartão), e `allocationInputSystemPrompt` ("…soma se aproxima da renda mensal…" / "…soma dos números se aproximar da renda…") passam a usar "orçamento mensal".
- [ ] 3.7 Reestruturar testes (`TestBuildMethodologyStep`/`Distribution`/`Summary` → testes do `budget_review` cobrindo o loop) e ativação idempotente.

## Detalhes de Implementação

Ver `techspec.md` seções "Semântica dos steps → budget_review" e ADR-003/ADR-004. A submáquina vive
num único cursor (o kernel não permite voltar a step anterior). `applyDraftBudget` espelha a lógica
atual de `BuildDistributionStep`.

## Critérios de Sucesso

- `go build ./... && go vet ./...` verdes.
- Teste de step: aceitar sugestão; enviar reais (soma = orçamento) → basis points; soma que não fecha
  → reprompt sem ativar; "não" no resumo → nova distribuição → resumo → "sim" → completa; ativação
  idempotente com `ErrBudgetAlreadyActive`.
- `grep -rniE "\brenda\b" internal/agents/application/workflows/onboarding_workflow.go` vazio; resumo sem linha de cartão (RF-21).
- Ativação só ocorre após confirmação (nenhum `ActivateBudget` antes do "sim").

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — step de workflow durável com submáquina, criação/ativação de budget via portas do consumidor `internal/agents`.
- `domain-modeling-production` — sub-estado `reviewAwaitKind` como tipo fechado (DMMF) governando as transições distribuição↔confirmação.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `BuildBudgetReviewStep`, `applyDraftBudget`, `BuildActivationStep`.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes do loop de review e ativação.
