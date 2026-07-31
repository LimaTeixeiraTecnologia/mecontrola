# Tarefa 3.0: Confirmação de receita com linha Data condicional

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Exibir a linha `📅 Data` no bloco de confirmação de receita apenas quando houver data específica informada/derivada. Ajustar `IncomeConfirmationBlock` (messages/catalog.go:251) para renderizar a linha condicionalmente e `confirmSummaryIncome` (transaction_write_workflow.go:1023) para preencher `DateFormatted` via `formatConfirmDate(state.OccurredAt, now)`, espelhando `confirmSummaryExpense`. Manter o catálogo oficial como fonte de verdade (texto `Posso registrar?`), sem adotar o texto alternativo do briefing.

<requirements>
- RF-10: bloco de confirmação oficial de receita (💰 Valor / 📥 Origem / Posso registrar?) com linha 📅 Data somente quando a data for informada/derivada.
- RF-26: todas as mensagens conforme catálogo oficial; nenhuma mensagem verbatim reescrita fora do catálogo.
</requirements>

## Subtarefas

- [ ] 3.1 Ajustar `IncomeConfirmationBlock` para incluir a linha `📅 Data` só quando `ConfirmationView.DateFormatted != ""`.
- [ ] 3.2 Ajustar `confirmSummaryIncome` para preencher `DateFormatted: formatConfirmDate(state.OccurredAt, now)`.
- [ ] 3.3 Unit tests do bloco com e sem `DateFormatted`; atualizar asserts de confirmação de receita existentes para o formato condicional.

## Detalhes de Implementação

Ver `techspec.md` seção "Modelos de Dados" e ADR-002 (`adr-002-confirmacao-receita-data-condicional.md`). `formatConfirmDate` (transaction_write_workflow.go:1066) já suprime a data quando vazia ou igual a hoje — a linha aparece apenas para datas específicas. Zero comentários em Go de produção. Não introduzir forma de pagamento em receita (RF-15 permanece fora de escopo).

## Critérios de Sucesso

- Confirmação de receita exibe `📅 Data` só quando há data específica; caso comum (hoje) mantém dois campos.
- 0 regressão em testes de confirmação; catálogo oficial preservado (sem texto alternativo).
- `go build`, `go vet`, `go test -race` verdes; `gofmt -l` limpo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — mensagem de confirmação e wiring no transaction_write_workflow do consumidor internal/agents.
- `design-patterns-mandatory` — gate de desenho para o ramo condicional de formatação (evitar duplicação/abstração desnecessária).

## Testes da Tarefa

- [ ] Testes unitários (IncomeConfirmationBlock com/sem data; confirmSummaryIncome preenche DateFormatted)
- [ ] Testes de integração (não aplicável — formatação e wiring puro)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/messages/catalog.go`
- `internal/agents/application/workflows/transaction_write_workflow.go`
- `internal/agents/application/messages/catalog_test.go` (ou test correspondente)
