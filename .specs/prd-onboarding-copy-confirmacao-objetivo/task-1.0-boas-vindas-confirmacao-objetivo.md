# Tarefa 1.0: Onboarding — boas-vindas (celular) + confirmação/reforço do objetivo determinístico + exemplo de valor

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Refinar as duas primeiras mensagens do onboarding em `internal/agents/application/workflows/onboarding_workflow.go`: trocar o fragmento de exemplo das boas-vindas (casa → celular), e fazer a mensagem seguinte à captura do objetivo confirmar o objetivo (eco) + reforço positivo, de forma determinística, antes da pergunta opcional de valor (com exemplo de valor alinhado a R$ 5.000,00).

<requirements>
- RF-01, RF-02: em `welcomeCombinedPrompt` (linha 527), trocar apenas "comprar uma casa, meta de R$ 400.000,00" por "comprar um celular novo, meta de R$ 5.000,00"; preservar todo o restante do texto e as substrings asseguradas ("🎉 Bem-vindo ao MeControla! 🎉", "Vamos começar? Qual é o seu principal objetivo financeiro para este mês?").
- RF-03: a mensagem após capturar o objetivo confirma o objetivo ecoando `state.Goal`, traz reforço positivo e mantém a pergunta opcional de valor.
- RF-04: confirmação+reforço geradas por função pura `goalConfirmationReprompt(goal string) string`, sem nova call-site de LLM.
- RF-05: não regredir a coleta opcional do valor da meta (`GoalValueAsked`, `DecideGoalValueCents`).
- RF-06: alinhar o exemplo de formato de valor em `goalValueReprompt` de "R$ 400.000,00"/"400 mil" para "R$ 5.000,00"/"5 mil".
</requirements>

## Subtarefas

- [x] 1.1 Trocar o fragmento de exemplo em `welcomeCombinedPrompt` (linha 527).
- [x] 1.2 Alinhar o exemplo de valor em `goalValueReprompt` (linha 531) para R$ 5.000,00 / "5 mil".
- [x] 1.3 Adicionar a função pura `goalConfirmationReprompt(goal string) string` (template com eco do objetivo + reforço + `goalValueReprompt`).
- [x] 1.4 Substituir os dois `suspendStep(state, goalValueReprompt)` (linhas 768 e 775) por `suspendStep(state, goalConfirmationReprompt(state.Goal))`.
- [x] 1.5 Atualizar/adicionar asserts unitários (817, 858, 910 → `goalConfirmationReprompt(...)`; `expected` de welcome; assert de "R$ 5.000,00"; unit test puro de `goalConfirmationReprompt`).

## Detalhes de Implementação

Ver techspec.md, seções "Strings Concretas" (Item 1 e Item 2) e "Abordagem de Testes / Testes Unitários". A string exata do reforço e o formato do template estão na techspec. Não duplicar; seguir a techspec.

## Critérios de Sucesso

- `welcomeCombinedPrompt` contém "comprar um celular novo, meta de R$ 5.000,00" e não contém "comprar uma casa"; substrings de boas-vindas preservadas.
- A 2ª mensagem contém o objetivo ecoado entre aspas + reforço + a pergunta de valor; `goalConfirmationReprompt` é puro (sem IO/context).
- `goalValueReprompt` contém "R$ 5.000,00"/"5 mil" e não contém "400.000".
- Nenhuma nova chamada de LLM; `go build`, `go vet`, `go test -race` e lint verdes no pacote afetado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a mudança vive no consumidor de agente (`internal/agents`, workflow de onboarding) sobre o substrato Mastra; sem nova call-site de LLM.
- `domain-modeling-production` — `goalConfirmationReprompt` é função pura (DMMF: sem IO/context, determinística), reforçando a pureza dos passos de decisão.
- `design-patterns-mandatory` — gate de desenho ao introduzir a nova função; confirma "não aplicar padrão" (ADR-003) em vez de abstração especulativa.

## Testes da Tarefa

- [x] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (produção)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (asserts)
