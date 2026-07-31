# Tarefa 2.0: Contrato de data verbatim nos schemas de register_income e edit_entry

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fixar o contrato de data LLM→tool: o LLM envia a expressão de data verbatim do usuário no campo `occurredAt`, sem computar data ISO. Acrescentar `description` explícita à propriedade `occurredAt` nos schemas de `register_income` e `edit_entry`, orientando o LLM a repassar a expressão literal (`semana passada`, `mês passado`, `dia 10`, `ontem`, ...) para que `ParseInputDate` (tarefa 1.0) faça a aritmética.

<requirements>
- RF-16: a compreensão de datas segue a arquitetura LLM-first pela ferramenta de registro; o LLM não faz aritmética de calendário — envia o token verbatim.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `description` à propriedade `occurredAt` no schema de `register_income` (tools/register_income.go), instruindo repasse verbatim da expressão de data, sem calcular ISO.
- [ ] 2.2 Adicionar/ajustar `description` da propriedade `occurredAt` no schema de `edit_entry` (tools/edit_entry.go) com a mesma instrução.

## Detalhes de Implementação

Ver `techspec.md` seção "Design de Implementação" e ADR-001. Tools permanecem finas (R-AGENT-WF-001.2 / R-ADAPTER-001): apenas mapeiam input→command; nenhuma regra de domínio, SQL ou branching. Zero comentários em Go de produção. A `description` é conteúdo de schema (string), não comentário de código.

## Critérios de Sucesso

- Os schemas de `register_income` e `edit_entry` descrevem `occurredAt` como expressão de data verbatim do usuário.
- `go build`, `go vet` verdes; `gofmt -l` limpo.
- Nenhuma regressão nos testes existentes das tools (financial_tools_test.go, new_read_write_tools_test.go).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração de schema/descrição de tool do agente (contrato LLM↔tool) sobre internal/platform/tool e internal/agents.

## Testes da Tarefa

- [ ] Testes unitários (schema das tools continua válido; snapshot/estrutura do schema)
- [ ] Testes de integração (não aplicável)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/register_income.go`
- `internal/agents/application/tools/edit_entry.go`
- `internal/agents/application/tools/financial_tools_test.go`
