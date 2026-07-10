# Tarefa 2.0: Steps de linguagem e ordem inicial (welcome, goal, monthly_budget, conclusion)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os steps de menor complexidade e a nova linguagem: boas-vindas isolada, meta sem preâmbulo,
apresentação das categorias + orçamento mensal em mensagem única, e conclusão reduzida (WorkingMemory
+ mensagem final). Elimina completamente a expressão "renda líquida".

<requirements>
- RF-01, RF-02, RF-03, RF-03a: `BuildWelcomeStep` isolado; suspende; no resume completa ignorando o texto (D-07); gatilho de início inalterado.
- RF-04..RF-08: `BuildGoalStep` pergunta a meta sem preâmbulo; aceita valor opcional (mantém sub-fluxo); objetivo vazio → reprompt.
- RF-09..RF-12: apresentação das 5 categorias com texto exato (RF-11) + pergunta de orçamento na mesma mensagem, um único suspend (D-01), sem confirmação da apresentação.
- RF-13, RF-14, RF-15: sem renda líquida; pergunta usa "orçamento mensal"; valor inválido → reprompt específico com exemplo em reais, sem criar budget/cartão/distribuição.
- RF-33, RF-34: conclusão grava WorkingMemory (objetivo + valor quando houver) e monta `FinalMessage`; não reabre distribuição/resumo/ativação.
- RF-37: mensagens/WM não expõem termos internos (workflow, run, snapshot, correlação, plataforma, infraestrutura).
</requirements>

## Subtarefas

- [ ] 2.1 `BuildWelcomeStep`: prompt de boas-vindas isolado (reaproveitar tom/emojis, sem pergunta de meta — D-10); resume completa ignorando texto.
- [ ] 2.2 `BuildGoalStep`: remover preâmbulo de `welcomeGoalPrompt`; manter extração de objetivo + valor opcional (1x) e reprompt de objetivo.
- [ ] 2.3 `BuildMonthlyBudgetStep` (renomeia `BuildIncomeStep`): prompt = texto exato RF-11 + pergunta de orçamento; schema `income_extract`→`monthly_budget_extract`; grava `MonthlyBudgetCents`; reprompt específico em reais.
- [ ] 2.4 `BuildConclusionStep`: reduzir a upsert de WorkingMemory + `FinalMessage` (remover ativação e recorrência, que migram para 3.0/4.0). A WorkingMemory permanece apenas objetivo (`## Objetivo Financeiro`) + metadata do valor da meta — **sem linha de renda/orçamento**; assert que não contém termo de renda (RF-33/M-02).
- [ ] 2.5 Atualizar/renomear prompts (`welcomeGoalPrompt`→welcome + goal; `incomePrompt`/`incomeReprompt`→orçamento) e os testes de step correspondentes.

## Detalhes de Implementação

Ver `techspec.md` seções "Prompts" e "Semântica dos steps". Cada step preserva o padrão
`ResumeText==""` → suspend; resume → processa e limpa `ResumeText`. `BuildConclusionStep` não ativa
orçamento (isso é 3.0/4.0).

## Critérios de Sucesso

- `go build ./... && go vet ./...` verdes.
- Teste de step: welcome suspende isolado e ignora texto; goal pergunta sem preâmbulo; monthly_budget
  contém o texto exato das 5 categorias e a pergunta de orçamento; valor inválido re-suspende sem criar nada.
- `grep -rniE "\brenda\b" internal/agents/application/workflows/onboarding_workflow.go` retorna vazio (M-02 amplo).
- Zero comentários em Go de produção; sem termos internos nas mensagens (RF-37).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — steps de workflow durável do agente financeiro (suspend/resume, extração estruturada via `agent.Agent`, prompts) no consumidor `internal/agents`.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — steps welcome/goal/monthly_budget/conclusion e prompts.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes de step (renomear `TestBuildIncomeStep`).
