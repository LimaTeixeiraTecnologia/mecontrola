# Categories Flows

![Categories container context](../system/mecontrola-container.svg)

## Objetivo do modulo

`internal/categories` expoe o catalogo editorial de categorias e o dicionario de classificacao, incluindo versao para ETag e integracao interna consumida por `budgets`.

## Arquivos .puml por fluxo

- [CAT-01-list-categories.puml](./CAT-01-list-categories.puml)
- [CAT-02-get-category.puml](./CAT-02-get-category.puml)
- [CAT-03-list-dictionary.puml](./CAT-03-list-dictionary.puml)
- [CAT-04-search-dictionary.puml](./CAT-04-search-dictionary.puml)
- [CAT-05-budgets-internal-reader.puml](./CAT-05-budgets-internal-reader.puml)

## Entradas, saidas e artefatos

### Endpoints

- `GET /api/v1/categories/`
- `GET /api/v1/categories/{id}`
- `GET /api/v1/category-dictionary/`
- `GET /api/v1/category-dictionary/search`

### Saidas

- Leitura em `categories`, `dictionary`, `version`
- Respostas com `ETag`
- Servicos internos expostos ao modulo `budgets`:
  - `ResolveBySlug`
  - `ValidateSubcategory`
  - `VersionReader`

## Matriz de fluxos

| ID | Origem | Tipo | Saida principal |
| --- | --- | --- | --- |
| CAT-01 | `GET /api/v1/categories/` | sync | Lista arvore/categoria com ETag |
| CAT-02 | `GET /api/v1/categories/{id}` | sync | Detalhe de categoria com ETag |
| CAT-03 | `GET /api/v1/category-dictionary/` | sync | Pagina entradas de dicionario |
| CAT-04 | `GET /api/v1/category-dictionary/search` | sync | Busca candidatos do dicionario |
| CAT-05 | chamada interna de `budgets` | sync | Resolve `root_slug`, valida subcategoria e versao de cache |

## Percurso detalhado

### CAT-01 - Listagem de categorias

Origem:
- `CategoryRouter.Register` -> `GET /api/v1/categories/`

Percurso:
1. `middleware.RequireUser` exige principal no contexto.
2. `ListCategoriesHandler.Handle` monta input de consulta.
3. O use case `ListCategories.Execute` le `CategoryRepository` e `VersionReader`.
4. O handler devolve `ETag` com a versao atual.
5. Se `If-None-Match` casar, responde `304 Not Modified`.

### CAT-02 - Detalhe de categoria

Origem:
- `GET /api/v1/categories/{id}`

Percurso:
1. `GetCategoryHandler.Handle` extrai `id`.
2. `GetCategory.Execute` consulta categoria e versao.
3. O handler reaplica logica de `ETag`.

### CAT-03 - Listagem do dicionario

Origem:
- `GET /api/v1/category-dictionary/`

Percurso:
1. `ListDictionaryHandler.Handle` interpreta filtros:
   - `category_id`
   - `kind`
   - `signal_type`
   - `cursor`
   - `page_size`
2. `ListDictionary.Execute` consulta `DictionaryRepository`.
3. O handler retorna `ETag` e pagina resultados.

### CAT-04 - Busca no dicionario

Origem:
- `GET /api/v1/category-dictionary/search`

Percurso:
1. `SearchDictionaryHandler.Handle` exige `q` e `kind`.
2. `SearchDictionary.Execute` usa:
   - `DictionaryRepository`
   - `CategoryRepository`
   - `VersionReader`
   - `services.CandidateResolver`
3. O retorno inclui candidatos, outcome e sinal predominante.
4. O handler aplica `ETag`.

### CAT-05 - Uso interno por budgets

Origem:
- `BudgetsModule.buildCategoriesCache`

Percurso:
1. `BudgetsModule` cria `postgres.NewCategoriesReaderAdapter` a partir de:
   - `ResolveBySlug`
   - `ValidateSubcategory`
   - `VersionReader`
2. `CategoriesCache.Boot` aquece os dados no bootstrap do modulo budgets.
3. Durante `UpsertExpense`, `budgets` chama `ValidateExpenseSubcategory`.
4. O adaptador devolve `root_slug` e metadados editoriais usados no calculo do budget.

## Rotas internas e dependencias cruzadas

- O modulo `categories` nao publica eventos.
- O modulo `budgets` depende dele sincronicamente para validacao editorial.
- O modulo usa `VersionReader` para invalidacao HTTP e tambem para cache interna de `budgets`.

## Observacoes arquiteturais

- Este modulo e somente leitura no runtime normal.
- O `ETag` e parte central do contrato externo e interno.

## Eficiencia, robustez e operacao

- `Caminho critico`
  - leituras dominadas por SQL em catalogo e version reader;
  - busca de dicionario e a rota mais custosa por classificacao e ordenacao.
- `Controles de robustez`
  - `RequireUser` protege endpoints;
  - `ETag` evita transferencia redundante e reduz pressao no banco;
  - validacao estrita de `kind`, `signal_type` e query.
- `Falhas esperadas`
  - query invalida: falha definitiva com 4xx;
  - falha de banco ou version reader: falha transiente com 5xx;
  - cache interna de `budgets` desatualizada depende da versao exposta por este modulo.
- `Observabilidade`
  - counters e histogramas por endpoint;
  - logs estruturados de request e outcome;
  - acompanhar taxa de `304` para validar eficiencia do ETag.
- `Capacidade`
  - modulo read-heavy; tuning principal e indexacao do dicionario e custo da busca textual.

## Guardrails operacionais

### Precondicoes e pos-condicoes

- endpoints publicos do modulo:
  - pre: principal autenticado e filtros validos;
  - pos: resposta consistente com `ETag`, ou `304` quando nada mudou.
- uso interno por budgets:
  - pre: `VersionReader` e adaptador operacionais;
  - pos: `root_slug` resolvido de forma consistente com o catalogo atual.

### Invariantes

- `version` precisa refletir mudanca semantica do catalogo consumido externamente;
- `kind` e `signal_type` invalidos nunca devem cair em fallback silencioso;
- o adaptador usado por `budgets` deve seguir o mesmo catalogo do HTTP.

### Runbook resumido

- queda de hit em `304`:
  - revisar estrategia de cache do cliente;
  - verificar se `version` esta mudando em excesso.
- busca degradada:
  - inspecionar indices do dicionario;
  - amostrar queries de maior custo e cardinalidade.

### Sinais e thresholds recomendados

- alerta se a latencia de `search_dictionary` crescer acima do SLO definido;
- alerta se `invalid_kind` ou `invalid_query` sair do baseline e indicar problema de cliente;
- alerta se o reader interno de `budgets` divergir da versao atual por periodo prolongado.
