# Tarefa 3.0: Estado fechado e decisão pura com confirmação de-para

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Modelar o estado fechado `CardUpdateState` e a decisão pura de confirmação, incluindo a pergunta de-para (valor atual → novo) por campo alterado. Base do workflow de edição (ADR-003).

<requirements>
- `CardUpdateStatus` e `CardUpdateState` como tipos fechados (state-as-type), com valores atuais e novos.
- `DecideCardUpdateConfirmation` pura (sem IO, sem `context.Context`), determinística.
- `buildCardUpdateQuestion` monta o de-para apenas dos campos alterados e a nota de impacto quando o vencimento muda.
- Cobre RF-05, RF-10, RF-11, RF-12, RF-13, RF-14, RF-19.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `internal/agents/application/workflows/card_update_state.go`: `CardUpdateStatus` (Active/Completed/Cancelled/Expired) e `CardUpdateState` (identidade, `ExpectedVersion`, valores atuais `Current*`, valores novos `New*`, controle de confirmação).
- [ ] 3.2 Criar `internal/agents/application/workflows/card_update_decisions.go`: TTL 15min, reprompt máx 1, `DecideCardUpdateConfirmation` (Accept/Cancel/Reprompt/Expire/Replay) reusando o tipo `CardConfirmAction` existente.
- [ ] 3.3 Implementar `buildCardUpdateQuestion` com linhas de-para por campo alterado e a nota "A alteração do dia de vencimento pode impactar parcelas em aberto." quando o vencimento muda (RF-19: permite, apenas avisa).
- [ ] 3.4 Garantir que o de-para inclua apenas campos informados e diferentes do atual.

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave" e "Modelos de Dados" e ADR-003. Reutilizar `AwaitingKind` e `CardConfirmAction` já existentes.

## Critérios de Sucesso

- `DecideCardUpdateConfirmation` é pura e testável sem mock; cobre todos os ramos.
- De-para correto e determinístico; campos inalterados não aparecem.
- Estado fechado sem `string` livre em assinatura pública.
- `gofmt`/`vet`/`lint`/`test -race` verdes; zero comentários em `.go`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — estado como tipo fechado (state-as-type) e função `Decide*` pura; modelagem de estados e transições sem IO.
- `mastra` — o estado e a decisão pertencem ao padrão de workflow durável do substrato de agent; consumo correto de `AwaitingKind`/`CardConfirmAction`.

## Testes da Tarefa

- [ ] Testes unitários: `DecideCardUpdateConfirmation` (accept/cancel/reprompt/expire/replay) e `buildCardUpdateQuestion` (de-para por campo, nota de impacto), padrão testify/suite, sem mock.
- [ ] Testes de integração: não aplicável (lógica pura).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/card_update_state.go`
- `internal/agents/application/workflows/card_update_decisions.go`
- `internal/agents/application/workflows/card_create_decisions.go` (referência: `CardConfirmAction`)
