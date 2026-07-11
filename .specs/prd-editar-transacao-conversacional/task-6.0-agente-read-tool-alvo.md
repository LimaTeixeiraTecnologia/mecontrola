# Tarefa 6.0: Agente — read tool `get_last_entry` + porta `ListRecentEntries` no ledger + testes

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar uma read tool fina para resolução determinística do alvo da edição ("último lançamento" / recentes), retornando `id`, `version` e resumo, delegando a uma nova porta de leitura no ledger (ADR-004).

<requirements>
- RF-01: iniciar edição referenciando "último lançamento" ou atributos.
- RF-02: múltiplas correspondências → lista numerada e aguarda escolha (desambiguação).
- Tool fina (adapter): sem regra/SQL/branching; `exec` delega à porta; zero comentários (R-ADAPTER-001, R-AGENT-WF-001.2).
- `version` retornada alimenta `TargetVersion` (optimistic lock, ADR-003).
- Desempate estável na leitura (ex.: `occurred_at`, `created_at`, `id`).
</requirements>

## Subtarefas

- [ ] 6.1 Porta `ListRecentEntries(ctx, limit)` no `transactionsLedgerAdapter` delegando a usecase/repo de leitura no ledger com ordenação estável.
- [ ] 6.2 Tool `get_last_entry`/`list_recent_entries` (`internal/agents/application/tools/`) com schema fino (id, version, descrição, valor, categoria, data) via `tool.NewTool`.
- [ ] 6.3 Registrar a tool em `internal/agents/module.go`.
- [ ] 6.4 Testes da tool + porta; atualizar `.mockery.yml` se necessário.

## Detalhes de Implementação

Ver `techspec.md` (Interfaces Chave — read tool) e `adr-004`. Reusar `SearchTransactions` para desambiguação por atributos; o "último" usa a nova leitura ordenada.

## Critérios de Sucesso

- Resolução determinística do "último lançamento"; desambiguação por atributos funcional.
- Gates R-ADAPTER-001 e R-AGENT-WF-001.2 verdes.
- `go build`, `go vet`, `go test -race`, lint do módulo verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — nova tool do agente sobre o substrato (schema, `tool.NewTool`, binding→usecase, registry).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/get_last_entry.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/module.go`
- `internal/agents/application/interfaces/transactions_ledger.go`
