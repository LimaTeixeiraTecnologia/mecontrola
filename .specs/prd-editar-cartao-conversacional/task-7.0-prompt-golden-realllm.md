# Tarefa 7.0: Prompt do agente e cobertura golden real-LLM

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o comportamento conversacional da edição: alinhar a instrução de `update_card` no prompt ao padrão de `create_card` (repasse verbatim, no-false-success, sem termos de infraestrutura) e adicionar cobertura golden/real-LLM de edição com gate ≥ 0,90 por categoria.

<requirements>
- O agente repassa `clarifyPrompt`/`confirmationPrompt` de `update_card` verbatim e nunca afirma atualização sem retorno do sistema.
- O agente identifica o cartão (resolve/list/get) antes de editar e, quando não encontra, oferece a lista.
- Casos golden de edição cobrindo apelido, banco, vencimento, cancelamento e banco não reconhecido.
- Schema da capture tool `update_card` no harness atualizado (sem `version`).
- Cobre RF-02, RF-03, RF-25, RF-26, RF-32.
</requirements>

## Subtarefas

- [ ] 7.1 Atualizar a instrução de `update_card` em `internal/agents/application/agents/mecontrola_agent.go` (slot/verbatim/no-false-success/no-infra-terms), espelhando as regras de `create_card`.
- [ ] 7.2 Adicionar casos em `internal/agents/application/golden/cases_card.go` (`CategoryCard`): editar apelido (resolve → update_card); alterar vencimento (resolve → update_card, resposta contém aviso de impacto); apelido não encontrado em edição (oferecer lista); cancelar edição (prior turn + "não" → sem tool, resposta de cancelamento); banco não reconhecido em edição pede fechamento.
- [ ] 7.3 Atualizar o schema da capture tool `update_card` no `harness_realllm_test.go` para `{cardId, nickname, bank, dueDay, closingDay}` (sem `version`).
- [ ] 7.4 Rodar o gate real-LLM (`RUN_REAL_LLM=1`, `OPENROUTER_API_KEY`) e confirmar razão ≥ 0,90 na `CategoryCard`.

## Detalhes de Implementação

Ver `techspec.md` seção "Abordagem de Testes / Testes E2E". Não usar `ExpectedOutcome` inexistente no enum `agent.ToolOutcome`; asserir seleção de tool (`ExpectedTools`) e propriedade de resposta (`ResponseProperty`). Depende de 5.0 e 6.0.

## Critérios de Sucesso

- Prompt não permite afirmação de sucesso sem retorno do sistema nem vaza termos de infraestrutura.
- Casos golden de edição passam com razão ≥ 0,90 na categoria de cartão.
- Casos dirigem por seleção de tool e propriedade semântica (sem strings frágeis).
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — instrução do agente e avaliação golden/evals do substrato; asserção comportamental por seleção de tool e propriedade de resposta com gate real-LLM.

## Testes da Tarefa

- [ ] Testes unitários: não obrigatórios (mudança de prompt validada por golden).
- [ ] Testes de integração: gate golden real-LLM (`CategoryCard` ≥ 0,90) com OpenRouter real.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent.go`
- `internal/agents/application/golden/cases_card.go`
- `internal/agents/application/golden/harness_realllm_test.go`
