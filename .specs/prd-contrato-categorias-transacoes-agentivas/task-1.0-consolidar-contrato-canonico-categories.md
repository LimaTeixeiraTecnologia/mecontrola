# Tarefa 1.0: Consolidar Contrato Canonico de Categories

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Consolidar `internal/categories` como autoridade canonica de catalogo, dicionario, candidatos, outcome e versao editorial. A entrega deve manter `SearchDictionary` como calculo canonico rico e criar `ResolveCategoryForWrite` para validar IDs concretos sem acoplar `categories` a `transactions`.

<requirements>
RF-01, RF-02, RF-03, RF-05, RF-06, RF-07, RF-08, RF-09, RF-10, RF-11, RF-14, RF-15, RF-16, RF-18, RF-20, RF-26, RF-32, RF-33, RF-34, RF-35.
RNF-01, RNF-04, RNF-05.
CA-01, CA-02, CA-03, CA-04, CA-05, CA-06, CA-07, CA-08, CA-13, CA-14, CA-15.
</requirements>

## Subtarefas

- [ ] 1.1 Expor internamente `Outcome`, `Version`, `HasMore`, `SignalType`, `Confidence`, `MatchQuality`, `MatchedTerm` e `MatchReason` em `SearchDictionary` sem perder compatibilidade com consumidores existentes.
- [ ] 1.2 Criar DTOs de entrada/saida de `ResolveCategoryForWrite` com root, leaf, kind e versao esperada.
- [ ] 1.3 Implementar `ResolveCategoryForWrite` usando repositorios reais de categorias e version reader existentes.
- [ ] 1.4 Retornar erros funcionais diferenciados para root inexistente, leaf inexistente, root sem folha, leaf fora da raiz, deprecated, kind divergente e version drift.
- [ ] 1.5 Atualizar `internal/categories/module.go` para expor o novo use case apenas como dependencia real.
- [ ] 1.6 Fechar testes unitarios e de integracao de `categories` contra mocks permissivos.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Contratos de categories", "Estado Real das Migrations de Categorias" e "Abordagem de Testes". Aplicar `go-implementation` e DMMF: outcomes e kinds como tipos fechados, zero value invalido quando houver invariante, smart constructors para novos objetos com regra, pipeline deterministico `parse -> validate -> decide -> return`.

## Critérios de Sucesso

- `SearchDictionary` preserva todos os campos calculados por `CandidateResolver`.
- `ResolveCategoryForWrite` prova raiz, subcategoria folha direta, kind, active/deprecated, path, nomes, slugs e versao editorial.
- Nenhum consumidor precisa inferir autoridade por primeira categoria ou string livre.
- Erros funcionais permitem diagnostico diferenciado sem depender de mensagem textual.
- Nao ha repository novo se `GetByID`, `ListByIDs` e `VersionReader.Current` forem suficientes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/categories/...`
- [ ] Testes unitarios para exact, alias, token, fuzzy, no match, multi-candidato e baixa evidencia.
- [ ] Testes unitarios para `ResolveCategoryForWrite`: aceite, root inexistente, leaf inexistente, root deprecated, leaf deprecated, leaf de outra raiz, root sem leaf, kind divergente e version drift.
- [ ] Testes de integracao com Postgres quando o pacote ja tiver infraestrutura de `testcontainers` habilitada.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/application/usecases/validate_subcategory.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
- `internal/categories/application/dtos/input/resolve_category_for_write_input.go`
- `internal/categories/application/dtos/output/dictionary_search_output.go`
- `internal/categories/application/dtos/output/resolve_category_for_write_output.go`
- `internal/categories/domain/services/candidate_resolver.go`
- `internal/categories/domain/valueobjects/search_outcome.go`
- `internal/categories/domain/valueobjects/match_score.go`
- `internal/categories/module.go`
