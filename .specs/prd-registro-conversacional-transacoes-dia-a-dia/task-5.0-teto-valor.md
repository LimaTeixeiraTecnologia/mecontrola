# Tarefa 5.0: Guarda de teto de valor `validateEntryAmount` nos execs de registro

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar uma guarda pura de sanity de valor na borda do agente, rejeitando valores irreais com mensagem amigável, sem tocar o invariante do VO `Money` (que é compartilhado por transactions/budgets/card).

<requirements>
- RF-04: valor validado como estritamente positivo (o VO `Money` já garante; a guarda dá mensagem amigável).
- RF-05: teto de sanity configurável na camada do agente (default R$ 10.000.000,00), sem alterar `Money`.
- ADR-003: `validateEntryAmount(cents) error` + `maxEntryAmountCents` puros; chamados no exec das tools de registro, produzindo clarificação (não erro de tool).
</requirements>

## Subtarefas

- [ ] 5.1 Definir `const maxEntryAmountCents int64 = 1_000_000_000` (R$ 10.000.000,00) e `validateEntryAmount(cents int64) error` puros na camada de tools do agente.
- [ ] 5.2 Rejeitar `cents <= 0` e `cents > maxEntryAmountCents` com sentinels/erros distintos (`amount_non_positive`, `amount_above_ceiling`).
- [ ] 5.3 Chamar a guarda no exec de `register_expense` e `register_income` antes de acionar `RegisterAttempt`, retornando resposta amigável de correção (não falha de schema/tool).
- [ ] 5.4 Testes de tabela: `0`, negativo, no teto (aceita), acima do teto (rejeita), valor normal.

## Detalhes de Implementação

Ver `techspec.md` › **Funções puras novas** (guarda de valor) e **ADR-003** (por que na borda, não no VO). Não duplicar.

## Critérios de Sucesso

- Valores `<= 0` e `> maxEntryAmountCents` produzem resposta amigável de correção, sem `engine.Start`.
- O VO `Money` permanece intacto (nenhum arquivo em `internal/transactions/domain/valueobjects/money.go` alterado).
- `go build`/`go vet` limpos; zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — altera o `exec` das tools de registro do consumidor agentivo (adapter fino do stack Mastra Go).

## Testes da Tarefa

- [ ] Testes unitários (tabela `validateEntryAmount`; exec rejeita com mensagem amigável)
- [ ] Testes de integração (n/a — comportamento validado em 8.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/register_expense.go`, `register_income.go` — chamada da guarda no exec.
- `internal/transactions/domain/valueobjects/money.go` — NÃO alterar (referência do invariante).
