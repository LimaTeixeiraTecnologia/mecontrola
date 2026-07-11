# Tarefa 1.0: Ajustar prompts de onboarding: saudação + objetivo e categorias

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ajustar o workflow de onboarding para que a primeira resposta ao usuário já combine boas-vindas e pergunta de objetivo financeiro numa única mensagem outbound, e atualizar a copy de apresentação das 5 categorias canônicas antes do orçamento mensal.

<requirements>
- RF-01: primeira resposta após "Ativar o meu plano" combina saudação e pergunta de objetivo numa única mensagem outbound.
- RF-02: usuário não precisa enviar "Oi" para avançar da saudação para o objetivo.
- RF-03: primeira mensagem usa exatamente a copy funcional com quebras de linha e emojis.
- RF-04: mensagem de orçamento apresenta as 5 categorias em linhas separadas com emoji e descrição curta.
- RF-05: pergunta do orçamento mensal aparece depois da explicação das 5 categorias.
- RF-06: mensagem de orçamento usa a copy funcional exata.
- RF-07: toda mensagem sobre cartão usa o emoji 💳.
- RF-08: não introduzir outro emoji para representar cartão.
</requirements>

## Subtarefas

- [ ] 1.1 Alterar `BuildWelcomeStep` para completar sem suspender quando `ResumeText == ""`.
- [ ] 1.2 Alterar `BuildGoalStep` para suspender com a mensagem combinada exata do RF-03 quando `ResumeText == ""`.
- [ ] 1.3 Atualizar `monthlyBudgetPrompt` para a copy funcional do RF-06, com as 5 categorias em linhas separadas.
- [ ] 1.4 Garantir que `wrapStepWithMessages` registre uma única mensagem assistente para a primeira resposta combinada.
- [ ] 1.5 Preservar compatibilidade com runs legados suspensos em `welcome`.

## Detalhes de Implementação

Ver `techspec.md` — seção **Design de Implementação / Componentes Modificados** e **Sequenciamento de Desenvolvimento**. O passo `welcome` deve continuar existindo para compatibilidade com definição e runs, mas não deve suspender isoladamente; o passo `goal` assume a responsabilidade de suspender com a copy combinada.

## Critérios de Sucesso

- Teste unitário confirma que `BuildWelcomeStep` não suspende com `ResumeText == ""`.
- Teste unitário confirma que `BuildGoalStep` suspende com a mensagem combinada exata do RF-03.
- Teste unitário confirma que `monthlyBudgetPrompt` contém a copy exata do RF-06, incluindo emojis e linhas das 5 categorias.
- Teste de integração do consumer confirma que a primeira resposta outbound é uma única mensagem.
- `go test -race -count=1 ./internal/agents/application/workflows/...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração no workflow de onboarding e prompts do consumidor agentivo.

## Testes da Tarefa

- [ ] Testes unitários de `BuildWelcomeStep`, `BuildGoalStep` e `monthlyBudgetPrompt`.
- [ ] Teste de integração do consumer WhatsApp para primeira resposta combinada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
