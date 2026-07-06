# Tarefa 6.0: Atualizar Contratos Agentivos e Adapters Ricos

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualizar `internal/agents` para consumir o contrato rico de `categories` e propagar evidencia categorial ate `transactions` sem transformar tool, LLM ou scorer em autoridade de escrita.

<requirements>
RF-01, RF-04, RF-05, RF-06, RF-17, RF-19, RF-25, RF-26, RF-30, RF-32, RF-33, RF-35.
RNF-01, RNF-05.
CA-12, CA-16.
</requirements>

## Subtarefas

- [ ] 6.1 Alterar `CategoriesReader.SearchDictionary` para retornar `CategorySearchResult` com outcome, version, hasMore e candidatos ricos.
- [ ] 6.2 Adicionar `ResolveForWrite` como porta consumidora quando necessario para clarificacao retomada.
- [ ] 6.3 Atualizar `categories_reader_adapter` para preservar todos os campos vindos de `categories`.
- [ ] 6.4 Estender `RawTransaction`/commands agentivos para transportar evidencia ate o ledger adapter.
- [ ] 6.5 Atualizar `transactions_ledger_adapter` sem decidir persistencia dentro de `agents`.
- [ ] 6.6 Regenerar mocks e endurecer stubs para nao omitirem `Outcome=matched` e `Version>0`.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Contratos de agents", "Pontos de Integracao" e "Conformidade com Padroes". Aplicar `mastra`: tools e adapters finos, consumidor `internal/agents` usando substrato real, sem recriar primitivos de plataforma, sem LLM/scorer desbloquear write. Aplicar `go-implementation` e DMMF nos tipos fechados adicionados.

## Critérios de Sucesso

- Adapter de `agents` nao descarta `Outcome`, `Version`, `SignalType`, `Confidence`, `MatchQuality`, `MatchedTerm` nem `MatchReason`.
- Mocks nao permitem sucesso sem evidencia completa.
- Evidencia sai de `agents` e chega ao adapter de `transactions`, mas a autorizacao final continua no gate de `transactions`.
- Nenhum prompt, scorer ou resposta de LLM aparece como source de decisao autorizadora.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — Altera consumidor agentivo, tools/adapters financeiros, contrato Thread/Run indireto e escrita financeira idempotente em `internal/agents`.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/...`
- [ ] Unit test do adapter preservando todos os campos do resultado de `categories`.
- [ ] Unit test do ledger adapter provando propagacao da evidencia.
- [ ] Testes de mocks/stubs falhando quando `Outcome` ou `Version` forem omitidos em sucesso.
- [ ] Integration com `categories` real quando infraestrutura estiver disponivel.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/interfaces/categories_reader.go`
- `internal/agents/application/interfaces/types.go`
- `internal/agents/application/interfaces/mocks/categories_reader.go`
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
