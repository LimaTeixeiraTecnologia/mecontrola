# Tarefa 2.0: Onboarding — cartão em bullets + regra de 💳 + selo de sucesso + objetivo único no resumo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

No `internal/agents/application/workflows/onboarding_workflow.go`: reorganizar as mensagens de cartão em bullets, aplicar a regra de 💳 (só na 1ª mensagem de cartão do fluxo + selo de sucesso), introduzir o selo de sucesso pós-cadastro, e remover a repetição do objetivo na frase de conclusão (mantendo-o só no cabeçalho do resumo).

<requirements>
- RF-07 (parte onboarding): 💳 apenas na 1ª mensagem de cartão (convite inicial, 1ª menção) e no selo de sucesso; proibido em reprompts, convite ao próximo, seção de cartões do resumo.
- RF-09: convite inicial, reprompts e convite ao próximo em blocos no estilo bullets.
- RF-10: preservar fragmentos obrigatórios não-emoji (exemplo com e sem apelido, "dia 1"/"dia primeiro", apelido herda banco).
- RF-11: mensagem pós-`CreateCard` = selo "💳 Cartão registrado com sucesso ✅" + "Quer registrar algum outro?".
- RF-12: selo só após cadastro na sessão; convite inicial mantém texto/contagem vigentes ajustado à regra de emoji.
- RF-13: objetivo aparece 1× no cabeçalho do "Resumo de Onboarding".
- RF-14: frase de conclusão não repete o objetivo; preserva a CTA; restante do resumo inalterado (exceto emoji da seção de cartões).
</requirements>

## Subtarefas

- [x] 2.1 Reescrever `cardsPrompt` (existing==0 e existing>0) em bullets, 💳 só na 1ª menção; preservar contagem e `**outro**` no ramo existing>0.
- [x] 2.2 Reescrever `cardsReprompt`/`cardsRepromptMissingName`/`cardsRepromptMissingDueDay`/`cardsRepromptMissingBoth` em bullets, sem 💳, preservando fragmentos obrigatórios.
- [x] 2.3 Adicionar `cardCreatedSuccessOnboarding` e usá-la no pós-`CreateCard` (linha 888), no lugar de `cardsPrompt(len(existingCards))`.
- [x] 2.4 Remover 💳 da seção de cartões do resumo: `conclusionSummaryMessage` ("Cartões:") e `renderCardsSummary` ("Nenhum cartão cadastrado.").
- [x] 2.5 Ajustar `conclusionFinalMessage` para não mencionar o objetivo (remover parâmetros; preservar CTA) e atualizar o caller (linha 706).
- [x] 2.6 Atualizar asserts unit/integration afetados (1958, 1952-1988, 2175-2176, 2178, 2294-2306; `replies[6]/[7]` e `replies[8]` na journey de integração).

## Detalhes de Implementação

Ver techspec.md, seções "Strings Concretas" (Item 3, Item 4, Item 5) e "Abordagem de Testes". As strings exatas de cada mensagem (bullets, selo, resumo, conclusão) estão na techspec; seguir sem duplicar.

## Critérios de Sucesso

- `cardsPrompt(0)` e `cardsPrompt(>0)` em bullets, com 💳 só na 1ª menção; reprompts sem 💳; fragmentos obrigatórios preservados.
- Pós-cadastro emite exatamente "💳 Cartão registrado com sucesso ✅\nQuer registrar algum outro?".
- Seção de cartões do resumo sem 💳; objetivo aparece 1× (cabeçalho); conclusão sem repetir objetivo, CTA preservada.
- Journey de integração reverificada (💳 e "Resumo de Onboarding"); `go build`/`vet`/`test -race`/lint verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — montagem de mensagem no workflow de onboarding do consumidor de agente sobre o substrato Mastra.
- `design-patterns-mandatory` — gate de desenho ao introduzir o selo de sucesso e reorganizar as funções de montagem; confirma "não aplicar padrão" (ADR-003).

## Testes da Tarefa

- [x] Testes unitários
- [x] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` (produção)
- `internal/agents/application/workflows/onboarding_workflow_test.go` (asserts)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go` (journey)
