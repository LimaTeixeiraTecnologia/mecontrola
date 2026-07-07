# Tarefa 1.0: Extensão aditiva de `get_transaction` (`subcategoryNameSnapshot`)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Expor o campo `subcategoryNameSnapshot` na saída da tool `get_transaction`. O dado já existe no
domínio (`interfaces.Entry.SubcategoryNameSnapshot`, `interfaces/types.go:181-198`) e é atualmente
descartado pela projeção da tool. É a base de dados para C5 renderizar `Categoria > Subcategoria`.
Mudança **puramente aditiva** (ADR-002 / D-09): sem nova tool, sem lógica de domínio, sem branching,
sem SQL, sem mexer em use case/binding/`module.go`.

<requirements>
- RF-06 (parte de dados): C5 deve poder exibir a subcategoria da última transação.
- RF-35: extensão aditiva única permitida; nada além do campo `subcategoryNameSnapshot`.
- ADR-002: adapter permanece fino (R-ADAPTER-001.2); zero comentários (R-ADAPTER-001.1).
- Schema `Strict: true`: novo campo entra em `properties` e em `required`; `exec` sempre preenche.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `SubcategoryNameSnapshot string \`json:"subcategoryNameSnapshot"\`` à struct `GetTransactionOutput` em `internal/agents/application/tools/get_transaction.go`.
- [ ] 1.2 Adicionar `"subcategoryNameSnapshot": map[string]any{"type": "string"}` em `properties` do schema `out` e incluir `"subcategoryNameSnapshot"` na lista `required`.
- [ ] 1.3 Mapear `SubcategoryNameSnapshot: entry.SubcategoryNameSnapshot` no retorno do `exec`.
- [ ] 1.4 Estender `TestGetTransactionTool_Success` (`read_tools_test.go:304`) para asseverar o campo com valor (`"Supermercado"`) e cobrir o caso de subcategoria vazia (string vazia, sem quebrar o schema).

## Detalhes de Implementação

Ver techspec.md, seção "Modelos de Dados" (struct completa) e "Sequenciamento de Desenvolvimento"
item 1. Não duplicar aqui — a fonte é a techspec e o ADR-002.

## Critérios de Sucesso

- `GetTransactionOutput` expõe `subcategoryNameSnapshot`; `exec` mapeia o campo do `Entry`.
- Schema estrito consistente (campo em `properties` e `required`).
- `go build ./internal/agents/...` e `go vet ./internal/agents/...` verdes.
- `go test -race -count=1 ./internal/agents/application/tools/...` verde, incluindo o assert novo.
- `golangci-lint run ./internal/agents/...` sem novos achados; zero comentários nos arquivos tocados.
- Nenhuma outra tool, use case, binding ou `module.go` alterado.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração em tool do consumidor agentivo (`tool.NewTool[I,O]`, schema estrito, adapter fino sobre `internal/platform/tool`).

## Testes da Tarefa

- [ ] Testes unitários (`TestGetTransactionTool_Success` estendido; caso com e sem subcategoria)
- [ ] Testes de integração (não aplicável — coberto pelos E2E existentes; sem nova fronteira de IO)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/tools/get_transaction.go` (struct + schema + exec — modificado)
- `internal/agents/application/tools/read_tools_test.go` (`TestGetTransactionTool_Success` — modificado)
- `internal/agents/application/interfaces/types.go` (referência: `Entry.SubcategoryNameSnapshot` — não modificado)
