# Tarefa 4.0: Parser de dias da semana `parseWeekday` + encaixe em `parseInputDate`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender o parser de data **puro** existente (`parseInputDate(text, now)`) com dias da semana, mantendo determinismo e pureza. Expressões de baixa precisão ("semana passada", "mês passado") não casam e caem no sentinel `""`, que já leva o fluxo a pedir data específica (RF-08). O agente apenas **repassa** o texto de data; a resolução é do parser.

<requirements>
- RF-06: datas relativas em `America/Sao_Paulo` com base em `now` (já existente; preservar).
- RF-07: dias da semana ("segunda".."domingo"; "X passada/passado" = −7); ocorrência mais recente incluindo hoje.
- RF-08: "semana passada"/"mês passado" rejeitados → pedir data específica (via sentinel `""`).
- RF-10: data resolvida em `YYYY-MM-DD`, base do `RefMonth` (não-cartão) e da data de compra (cartão).
- ADR-002: `parseWeekday(text, now) (string, bool)` pura; sem IO; `now` injetado.
</requirements>

## Subtarefas

- [ ] 4.1 Implementar `parseWeekday(text string, now time.Time) (string, bool)` pura em `pending_entry_decisions.go`, reconhecendo `segunda`/`segunda-feira`, `terca`, `quarta`, `quinta`, `sexta`, `sabado`, `domingo` (via `normalizeText`), com sufixo `passada`/`passado` = −7 dias; fuso via `now.Location()`.
- [ ] 4.2 Encaixar a chamada em `parseInputDate` antes do fallback de formato explícito; sem casar → segue o fluxo atual (retorna `""`).
- [ ] 4.3 Garantir que "semana passada"/"mês passado" não casem (não contêm nome de dia da semana) → `""`.
- [ ] 4.4 Testes de tabela com `now` fixo em vários dias da semana cobrindo cada dia, "X passada/passado", acentos e "-feira", e não-casamento de "semana/mês passado".

## Detalhes de Implementação

Ver `techspec.md` › **Funções puras novas** e **ADR-002** (contrato de "segunda" sem sufixo = ocorrência mais recente incluindo hoje; "passada" = −7). Não duplicar.

## Critérios de Sucesso

- `parseWeekday` é pura (sem `time.Now()` interno) e determinística para `(text, now)` fixos.
- Todos os casos de tabela verdes; nenhum caso de "semana/mês passado" produz data.
- `go build`/`go vet` limpos; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera a lógica de decisão pura do workflow `pending-entry` do consumidor agentivo.

## Testes da Tarefa

- [ ] Testes unitários (tabela `parseWeekday` com `now` fixo; encaixe em `parseInputDate`)
- [ ] Testes de integração (n/a — validação de comportamento em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/pending_entry_decisions.go` — `parseInputDate`, `parseWeekday`, `normalizeText`.
