# Tarefa 2.0: Endurecer etapa de 💳 opcional e contextual

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tornar a etapa de cartão do onboarding capaz de reconhecer cartões existentes, aceitar banco/apelido único com vencimento em linguagem natural, permitir recusa sem bloquear o onboarding e informar exatamente qual dado falta em respostas incompletas.

<requirements>
- RF-07: toda mensagem, critério, teste e copy sobre cartão usa o emoji 💳.
- RF-08: não introduzir outro emoji para cartão.
- RF-09: quando houver cartão ativo, onboarding reconhece e pergunta se deseja cadastrar OUTRO 💳.
- RF-10: resposta negativa com cartão existente não duplica cartão e prossegue com os existentes.
- RF-11: quando não houver cartão e o usuário informar banco/apelido + vencimento, criar 💳 ativo com due_day entre 1 e 31.
- RF-12: respostas como "Santander, vencimento dia 1" são válidas; banco/apelido único preenche ambos os campos.
- RF-13: após criar 💳 válido, perguntar se deseja OUTRO 💳; resposta negativa conclui a etapa.
- RF-14: usuário sem 💳 pode recusar e concluir onboarding sem cartão ativo.
- RF-15: resposta incompleta não cria cartão parcial, explica dado faltante com 💳 e mantém workflow suspenso sem marcar `cardsDone=true`.
</requirements>

## Subtarefas

- [ ] 2.1 Atualizar `cardsPrompt` para distinguir mensagem quando `existing > 0` (OUTRO 💳) e `existing == 0` (cadastro opcional).
- [ ] 2.2 Adicionar normalização pura para extração de cartão: quando `nickname` ou `bank` vier preenchido e `dueDay` válido, replicar o valor para ambos.
- [ ] 2.3 Implementar decisão tipada para erro de cartão incompleto (falta banco/apelido vs. falta vencimento).
- [ ] 2.4 Montar reprompt específico usando 💳 quando a extração estiver incompleta.
- [ ] 2.5 Garantir que recusa sem cartão marque `CardsDone=true` sem chamar `CreateCard`.

## Detalhes de Implementação

Ver `techspec.md` — seções **Modelos de Dados / Mudanças de decisão em memória** e **Testes Unitários / Onboarding**. A normalização deve ser função pura, sem IO. O handler do passo deve distinguir os dois tipos de incompletude para montar reprompt específico.

## Critérios de Sucesso

- Testes unitários cobrem `cardsPrompt(existing > 0)` e `cardsPrompt(existing == 0)`.
- Testes unitários confirmam que "Santander, vencimento dia 1", "Nubank, vencimento dia 1" e "XP, vencimento dia 1" criam `NewCard` com `Nickname` e `Bank` preenchidos e `DueDay=1`.
- Teste unitário confirma que recusa sem cartão marca `CardsDone=true` sem chamar `CreateCard`.
- Teste unitário confirma que resposta incompleta não chama `CreateCard`, não marca `CardsDone` e retorna reprompt específico do dado faltante.
- `go test -race -count=1 ./internal/agents/application/workflows/...` passa.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração no workflow de onboarding, decisões de cartão e interação com `CardManager`.

## Testes da Tarefa

- [ ] Testes unitários de prompts, parsing, criação, recusa e incompletude de cartão.
- [ ] Teste de integração do fluxo de cartão no onboarding.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go`
