# Tarefa 5.0: Repositórios Postgres, VersionReader e testes de integração

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os repositórios Postgres (`CategoryRepository`, `DictionaryRepository`) e o `VersionReader`, com queries otimizadas para listagem hierárquica, busca exata normalizada, paginação cursor-based e leitura de versão editorial. Incluir testes de integração com Postgres real.

<requirements>
- RF-08–RF-14a: listagem com filtros, ordenação PT-BR, paginação cursor
- RF-11: ordenação alfabética PT-BR nas queries
- RF-15a: busca ignora entradas com `deprecated_at`
- RF-15–RF-16a: busca exata com normalização, rejeição de q curto/vazio
- RF-20: usar `term_normalized = lower(unaccent($1))` no servidor
- RF-37: itens descontinuados ignorados por padrão
- ADR-002: VersionReader lê `category_editorial_version`
- ADR-003: cursor opaco com `term_normalized` + `id`
- RT-07: testes de integração com Postgres real
</requirements>

## Subtarefas

- [ ] 5.1 Implementar `CategoryRepository.List` com filtros `kind`, `parent_id`, `include_deprecated`
- [ ] 5.2 Implementar `CategoryRepository.GetByID`
- [ ] 5.3 Implementar `DictionaryRepository.List` com paginação cursor-based
- [ ] 5.4 Implementar `DictionaryRepository.Search` com correspondência exata normalizada
- [ ] 5.5 Implementar `VersionReader.Current`
- [ ] 5.6 Testes de integração para todos os métodos com `testcontainers-go`

## Detalhes de Implementação

Ver techspec.md seções **Interfaces Chave** e **Modelos de Dados**.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/persistence.md` e `references/testing.md`
- Carregar `references/examples-infrastructure.md` para paginação cursor-based
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- `CategoryRepository.List` sem `parent_id`: retorna todas as categorias do `kind` solicitado; use case monta árvore.
- `CategoryRepository.List` com `parent_id`: retorna apenas subcategorias diretas.
- Ordenação: `ORDER BY name COLLATE "pt_BR"` em todas as queries de listagem.
- `DictionaryRepository.List`: cursor decodifica `term_normalized|id`; query usa `WHERE (term_normalized, id) > ($term, $id) ORDER BY term_normalized COLLATE "pt_BR", id LIMIT $pageSize`.
- `DictionaryRepository.Search`: `SELECT ... WHERE kind = $1 AND term_normalized = lower(unaccent($2)) AND deprecated_at IS NULL`.
- `VersionReader.Current`: `SELECT version FROM mecontrola.category_editorial_version LIMIT 1`.
- Não computar normalização em Go; delegar 100% ao Postgres.

## Critérios de Sucesso

- [ ] `CategoryRepository.List` retorna árvore hierárquica correta para `kind=expense`
- [ ] `DictionaryRepository.List` percorre 3 páginas sem gaps nem duplicatas
- [ ] `DictionaryRepository.Search` encontra "água" com `q=agua` e vice-versa
- [ ] `VersionReader.Current` retorna versão correta após baseline
- [ ] Testes de integração com build tag `//go:build integration` passam
- [ ] Gate R0-R7 passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration test: `CategoryRepository` — listagem hierárquica, filtro por `parent_id`, ordenação PT-BR, inclusão de deprecated
- [ ] Integration test: `DictionaryRepository` — paginação cursor, busca exata, normalização unaccent
- [ ] Integration test: `VersionReader` — leitura consistente

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/infrastructure/repositories/postgres/category_repository.go`
- `internal/categories/infrastructure/repositories/postgres/dictionary_repository.go`
- `internal/categories/infrastructure/repositories/postgres/version_reader.go`
- Arquivos `_integration_test.go` correspondentes
- `mockery.yml`
