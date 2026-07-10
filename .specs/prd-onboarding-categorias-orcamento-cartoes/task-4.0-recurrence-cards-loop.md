# Tarefa 4.0: Step `recurrence` + step `cards` em loop um-por-vez

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Extrair a pergunta de recorrência para um step próprio (logo após a ativação) e transformar o cadastro
de cartão único num loop um-por-vez que repete até a cliente recusar, sem limite e sem tocar o
orçamento ativado.

<requirements>
- RF-24: após a ativação e antes dos cartões, perguntar se deseja repetir o orçamento por 12 meses (D-02).
- RF-25: "sim" cria recorrência; "não"/ambíguo segue sem recorrência (D-11); nenhuma resposta desfaz o orçamento ativado.
- RF-26: cadastro de cartões ocorre após distribuição, ativação e recorrência.
- RF-27, RF-28, RF-29: perguntar por cartão; criar quando apelido/banco/vencimento válidos; após criar, perguntar por outro; repetir até "não".
- RF-30: cartão incompleto/inválido pede só o que falta, não cria parcial, não desfaz o orçamento.
- RF-31, RF-31a: um cartão por mensagem; loop sem limite máximo (D-05).
- RF-32: recusa marca a etapa de cartões concluída.
</requirements>

## Subtarefas

- [ ] 4.1 `BuildRecurrenceStep`: suspend com `conclusionRecurrencePrompt`; resume → "sim" `CreateRecurrence(...,12)`; "não"/ambíguo → segue sem recorrência (D-11); não desfazer orçamento.
- [ ] 4.2 Reescrever `BuildCardsStep` como loop: primeira entrada → `ListCards` (contagem) → suspend(`cardsPrompt`).
- [ ] 4.3 Resume: `wantsCard=false` → `CardsDone=true` → completa; inválido → suspend(`cardsReprompt`) sem criar parcial; válido → `CreateCard` → re-suspend(`cardsPrompt`) [mesmo cursor].
- [ ] 4.4 Testes: recorrência sim/não/ambíguo; recusa imediata de cartão; um cartão; dois cartões (loop); cartão inválido mantém loop sem criar parcial; orçamento ativado intacto.

## Detalhes de Implementação

Ver `techspec.md` "Semântica dos steps → cards" e ADR-004/ADR-005. O loop usa a re-suspensão no mesmo
cursor do kernel. `ListCards` é a fonte de verdade da contagem para o prompt.

## Critérios de Sucesso

- `go build ./... && go vet ./...` verdes.
- Teste de step: recorrência ambígua não cria recorrência; loop cadastra 2+ cartões e encerra com
  "não"; cartão inválido não é criado e não altera o orçamento; `CardsDone` marcado ao recusar.
- Nenhum caminho ativa/desfaz orçamento no step de cartões (RF-30).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — steps de workflow durável com loop por re-suspensão e criação de cartão/recorrência via portas do consumidor `internal/agents`.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `BuildRecurrenceStep`, `BuildCardsStep` (loop).
- `internal/agents/application/workflows/onboarding_workflow_test.go` — testes de recorrência e loop de cartões.
