<!-- spec-hash-prd: cc1021c7ec9c74909c692690bab838f2e96cdfc9f45ed0e9b341334658b3c3a4 -->

# Especificação Técnica — Módulo de Categorias (internal/categories)

## Resumo Executivo

O módulo `internal/categories` é um bounded context somente leitura que expõe taxonomia global PT-BR de receitas e despesas via API REST. A implementação segue o padrão obrigatório de módulo já estabelecido por `internal/billing` e `internal/identity`: DI manual explícita em `module.go`, repositórios Postgres, use cases stateless, handlers finos e routers que implementam `Register(chi.Router)`.

As decisões arquiteturais centrais são: (a) autenticação via `RequireUser` canônico de `identity`; (b) versão editorial monotônica em tabela dedicada para ETag/304; (c) normalização accent-insensitive em coluna gerada PostgreSQL com `unaccent`; (d) busca determinística por correspondência exata normalizada, sem fuzzy ou IA; (e) deduplicação por precedência editorial (`canonical_name > alias > phrase > merchant > segment`) com ambiguidade estrita quando coexistem múltiplas subcategorias.

## Arquitetura do Sistema

### Visão Geral dos Componentes

```text
internal/categories/
  module.go                                    -- DI manual, wiring repository -> usecase -> handler -> router
  application/
    dtos/input/
      list_categories_input.go
      get_category_input.go
      list_dictionary_input.go
      search_dictionary_input.go
    dtos/output/
      category_output.go
      category_tree_output.go
      dictionary_entry_output.go
      dictionary_search_output.go  -- inclui SignalTypeTop string e IsAmbiguous bool para métricas no handler
    usecases/
      list_categories.go
      get_category.go
      list_dictionary.go
      search_dictionary.go
    interfaces/
      category_repository.go
      dictionary_repository.go
      version_reader.go
  domain/
    entities/
      category.go          -- root ou subcategory, validação de profundidade máxima 2
      dictionary_entry.go  -- term, signal_type, confidence, is_ambiguous
    valueobjects/
      kind.go              -- income | expense, enum iota+1
      signal_type.go       -- canonical_name | alias | phrase | merchant | segment
      confidence.go        -- high | medium | low
      allocation_type.go   -- consumption | asset_allocation
    services/
      candidate_resolver.go -- recebe []entities.DictionaryEntry, retorna DictionarySearchOutput; não há candidate_output.go
  infrastructure/
    http/server/
      router.go            -- CategoryRouter, registra rotas em /api/v1/categories e /api/v1/category-dictionary; aplica RequireUser em todas as rotas
      handlers/
        list_categories_handler.go
        get_category_handler.go
        list_dictionary_handler.go
        search_dictionary_handler.go
    repositories/
      postgres/
        category_repository.go
        dictionary_repository.go
        version_reader.go
```

**Relacionamentos:**
- `cmd/server/server.go` instancia `categories.NewCategoriesModule(mgr, o11y)` e registra `CategoryRouter` no HTTP server.
- O módulo consome `manager.Manager` (Postgres) e `observability.Observability` (devkit-go).
- O módulo importa `internal/identity/infrastructure/http/server/middleware` apenas para aplicar `RequireUser` nos routers.
- Não há dependência de outbox, workers, consumers, jobs, WhatsApp ou billing.

## Design de Implementação

### Interfaces Chave

```go
package interfaces

type CategoryRepository interface {
	List(ctx context.Context, q CategoryQuery) ([]entities.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (entities.Category, error)
}

type DictionaryRepository interface {
	List(ctx context.Context, q DictionaryQuery) ([]entities.DictionaryEntry, string, error)
	Search(ctx context.Context, q DictionarySearchQuery) ([]entities.DictionaryEntry, error)
}

type VersionReader interface {
	Current(ctx context.Context) (int64, error)
}
```

```go
package usecases

type ListCategories struct {
	repo    interfaces.CategoryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

type GetCategory struct {
	repo    interfaces.CategoryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

type ListDictionary struct {
	repo    interfaces.DictionaryRepository
	version interfaces.VersionReader
	o11y    observability.Observability
}

type SearchDictionary struct {
	repo    interfaces.DictionaryRepository
	version interfaces.VersionReader
	resolver *services.CandidateResolver
	o11y    observability.Observability
}
```

### Modelos de Dados

**Tabelas PostgreSQL (schema `mecontrola`):**

```sql
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TABLE mecontrola.category_editorial_version (
    version     BIGINT      NOT NULL PRIMARY KEY DEFAULT 1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed obrigatório: garante que VersionReader.Current nunca retorne zero rows
INSERT INTO mecontrola.category_editorial_version (version) VALUES (1)
ON CONFLICT DO NOTHING;

CREATE TABLE mecontrola.categories (
    id                UUID        NOT NULL PRIMARY KEY,
    slug              TEXT        NOT NULL,
    name              TEXT        NOT NULL,
    kind              TEXT        NOT NULL,
    parent_id         UUID        NULL REFERENCES mecontrola.categories(id),
    allocation_type   TEXT        NOT NULL DEFAULT 'consumption',
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT categories_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT categories_allocation_type_check CHECK (allocation_type IN ('consumption', 'asset_allocation')),
    CONSTRAINT categories_parent_same_kind CHECK (
        parent_id IS NULL OR EXISTS (
            SELECT 1 FROM mecontrola.categories p WHERE p.id = categories.parent_id AND p.kind = categories.kind
        )
    ),
    CONSTRAINT categories_no_cycles CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX categories_kind_slug_uniq_idx
    ON mecontrola.categories (kind, slug);

CREATE INDEX categories_kind_parent_idx
    ON mecontrola.categories (kind, parent_id)
    WHERE deprecated_at IS NULL;

CREATE INDEX categories_parent_sort_idx
    ON mecontrola.categories (parent_id, name COLLATE "pt_BR")
    WHERE deprecated_at IS NULL;

CREATE TABLE mecontrola.category_dictionary (
    id                UUID        NOT NULL PRIMARY KEY,
    category_id       UUID        NOT NULL REFERENCES mecontrola.categories(id),
    kind              TEXT        NOT NULL,
    term              TEXT        NOT NULL,
    term_normalized   TEXT        GENERATED ALWAYS AS (lower(unaccent(term))) STORED,
    signal_type       TEXT        NOT NULL,
    confidence        TEXT        NOT NULL,
    is_ambiguous      BOOLEAN     NOT NULL DEFAULT false,
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT dictionary_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT dictionary_signal_type_check CHECK (signal_type IN ('canonical_name', 'alias', 'phrase', 'merchant', 'segment')),
    CONSTRAINT dictionary_confidence_check CHECK (confidence IN ('high', 'medium', 'low'))
);

CREATE UNIQUE INDEX dictionary_active_term_uniq_idx
    ON mecontrola.category_dictionary (kind, category_id, term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized)
    WHERE deprecated_at IS NULL;
```

**Entidades de domínio:**

```go
package entities

type Category struct {
	ID             uuid.UUID
	Slug           string
	Name           string
	Kind           valueobjects.Kind
	ParentID       *uuid.UUID
	AllocationType valueobjects.AllocationType
	DeprecatedAt   *time.Time
}

func (c Category) IsRoot() bool   { return c.ParentID == nil }
func (c Category) IsActive() bool { return c.DeprecatedAt == nil }

type DictionaryEntry struct {
	ID            uuid.UUID
	CategoryID    uuid.UUID
	Kind          valueobjects.Kind
	Term          string
	SignalType    valueobjects.SignalType
	Confidence    valueobjects.Confidence
	IsAmbiguous   bool
	DeprecatedAt  *time.Time
}
```

### Endpoints de API

| Método | Path | Descrição | Autenticação |
|---|---|---|---|
| GET | `/api/v1/categories` | Listar categorias e subcategorias ativas; filtros `kind`, `parent_id`, `include_deprecated`; sem paginação — retorna tudo (teto ~400 registros) | RequireUser |
| GET | `/api/v1/categories/{id}` | Obter categoria por ID; 2 queries sequenciais: (a) root → GET by ID + LIST subcategorias (parent_id=id); (b) subcategoria → GET by ID + GET parent by parent_id. Categoria depreciada sem `include_deprecated=true` retorna `not_found`. | RequireUser |
| GET | `/api/v1/category-dictionary` | Listar entradas do dicionário; filtros `category_id`, `kind`, `signal_type` | RequireUser |
| GET | `/api/v1/category-dictionary/search` | Buscar termo no dicionário; query params `q`, `kind` | RequireUser |

**Headers de cache em todos os endpoints de leitura (incluindo 404 e 422):**
- Resposta: `ETag: "v<N>"`, `Content-Type: application/json`, corpo inclui `version: N`.
- Requisição de revalidação: `If-None-Match: "v<N>"` → responde `304 Not Modified` quando `N` é igual à versão atual.
- Erros `not_found`, `invalid_query`, `invalid_kind` também incluem `ETag` e `version` no corpo/header, pois são respostas de leitura.

**Envelope de erro:** segue `responses.ErrorWithDetails` do devkit-go, reutilizando o padrão de `billing` e `identity`. Códigos de erro específicos:
- `invalid_query` — `q` ausente, vazio, somente pontuação ou normalizado < 3 caracteres.
- `invalid_kind` — `kind` ausente ou não é `income`/`expense`.
- `not_found` — categoria inexistente ou descontinuada sem `include_deprecated=true`.

**Ordenação:** todas as listagens (categorias e dicionário) usam `ORDER BY ... COLLATE "pt_BR"` para garantir ordenação alfabética PT-BR (RF-11).

**Paginação cursor-based (RF-14a):**
- `page_size` default 50, máximo 200.
- Cursor opaco: base64 de `term_normalized + "|" + id` da última entrada retornada.
- Query usa `WHERE (term_normalized, id) > ($cursorTerm, $cursorID) ORDER BY term_normalized COLLATE "pt_BR", id LIMIT $pageSize`.
- Resposta inclui `next_cursor` quando há mais resultados.
- `Search` usa `LIMIT 100` fixo: `DictionarySearchQuery.Limit` é preenchido pelo use case com a constante `searchCandidateLimit = 100` antes de chamar o repositório; candidatos excedentes são descartados pelo resolver silenciosamente (volumetria máxima esperada por termo: dezenas, não centenas).
- `DictionaryRepository.List` retorna `([]entities.DictionaryEntry, string, error)` — o segundo valor é o `next_cursor` opaco (base64); vazio quando não há próxima página.

## Pontos de Integração

- **Identity**: importação de `internal/identity/infrastructure/http/server/middleware` para `RequireUser`. Não há chamada a use cases de identity.
- **Database**: `manager.Manager` do devkit-go fornece `DBTX(ctx)` para repositórios.
- **Observability**: `observability.Observability` do devkit-go para traces, logs e métricas.
- **Nenhuma integração externa**: não há HTTP outbound, filas, workers, outbox, WhatsApp ou agentes de IA.

## Abordagem de Testes

### Testes Unitários

- Todo use case com suite `testify/suite`, table-driven.
- Mocks de `CategoryRepository`, `DictionaryRepository` e `VersionReader` gerados via `mockery.yml`.
- `CandidateResolver` testado com cenários positivos/negativos cobrindo deduplicação e ambiguidade.
- Cenários obrigatórios por regra R4: happy path, erro de validação, erro de infraestrutura.

### Testes de Integração

- Adotados (RT-07): fronteira Postgres real com testcontainers-go, build tag `//go:build integration`.
- Suites:
  - `CategoryRepositoryIntegrationSuite` — valida schema, seed, listagem, filtro por `kind`/`parent_id`, ordenação PT-BR, inclusão de deprecated.
  - `DictionaryRepositoryIntegrationSuite` — valida normalização `unaccent`, busca exata, paginação cursor, rejeição de `deprecated_at`.
  - `VersionReaderIntegrationSuite` — valida leitura da versão editorial.
- Seed de teste: migrations editoriais mínimas executadas no container para garantir IDs determinísticos e dados canônicos para os cenários de aceitação.

### Testes de Cenários Canônicos

Cobertura obrigatória dos cenários CC-B1 a CC-B5, CC-D1 a CC-D5, CC-L1 a CC-L5, CC-V1 a CC-V4 descritos no PRD. Cada cenário é um teste de integração ou um teste de use case com seed controlado.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Schema e migrations** — DDL das tabelas `categories`, `category_dictionary`, `category_editorial_version`; migration de habilitação da extensão `unaccent`; migration de seed do catálogo e dicionário mínimo.
2. **Domínio** — value objects (`kind`, `signal_type`, `confidence`, `allocation_type`), entidades (`Category`, `DictionaryEntry`), `CandidateResolver`.
3. **Repositórios** — implementações Postgres com queries e testes de integração.
4. **Use cases** — `ListCategories`, `GetCategory`, `ListDictionary`, `SearchDictionary`.
5. **Handlers e router** — handlers finos, `CategoryRouter`, aplicação de `RequireUser`.
6. **Module e wiring** — `CategoriesModule` com DI manual (sem struct de config), assinatura `NewCategoriesModule(mgr manager.Manager, o11y observability.Observability)`, registro em `cmd/server/server.go`.
7. **Observabilidade** — métricas custom (counters por outcome e q_len_bucket), traces nos handlers e use cases.
8. **OpenAPI** — `openapi.yaml` versionado em `internal/categories/openapi.yaml`; primeiro contrato OpenAPI do projeto, servindo como referência para futuros módulos.
9. **Testes de cenários canônicos e validação R0-R7**.

### Dependências Técnicas

- Extensão PostgreSQL `unaccent` deve estar disponível no ambiente de CI e produção.
- Migration de seed editorial deve ser aplicada antes dos testes de integração.
- `VersionReader.Current` emite 1 query por request (sem cache); aceitável para MVP dado volume de leitura. Revisitar se `category_dictionary_search_total` indicar latência de DB > 5ms em P99.

## Monitoramento e Observabilidade

### Métricas

Expostas via `o11y.Metrics()` (devkit-go):

```
categories_list_total          counter   labels=[endpoint, kind, outcome]
categories_get_total           counter   labels=[endpoint, outcome]
category_dictionary_list_total counter   labels=[endpoint, kind, outcome]
category_dictionary_search_total counter labels=[endpoint, kind, outcome, q_len_bucket, signal_type_top]
```

- `outcome`: `matched`, `ambiguous`, `no_match`, `invalid_query`, `invalid_kind`.
- `q_len_bucket`: `3-4`, `5-8`, `9-16`, `17-32`, `33+`.
- `signal_type_top`: lido de `DictionarySearchOutput.SignalTypeTop` pelo handler; vazio quando `IsAmbiguous=true` ou sem candidato.

### Logs

- `INFO` em cada request de leitura com `endpoint`, `method`, `outcome`, `duration_ms`.
- `ERROR` apenas em falhas de infraestrutura ( Postgres indisponível); nunca logar termo bruto de busca.

### Traces

- Span por handler (`categories.handler.list`, `categories.handler.search`, etc.).
- Span por use case e por query de repositório.

## Considerações Técnicas

### Decisões Chave

| Decisão | ADR | Resumo |
|---|---|---|
| Autenticação via RequireUser em todos os endpoints | [ADR-001](adr-001-autenticacao-requireuser.md) | Endpoints autenticados com middleware canônico de identity; anula RT-08 para este módulo por decisão de produto v7. |
| Versão editorial em tabela dedicada | [ADR-002](adr-002-versao-editorial-tabela.md) | Tabela `category_editorial_version` com uma linha; migrations editoriais dão UPDATE explícito. |
| Cursor opaco base64 de último ID + ordem alfabética | [ADR-003](adr-003-cursor-paginacao.md) | Cursor codifica `id` da última entrada; query usa `(term_normalized, id) > ($term, $id)`. |
| UUIDv5 namespace derivado do domínio | [ADR-004](adr-004-namespace-uuidv5.md) | `uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))` como namespace fixo. |
| Normalização via coluna gerada `unaccent` | [ADR-005](adr-005-normalizacao-unaccent.md) | `term_normalized GENERATED ALWAYS AS (lower(unaccent(term))) STORED`; sem paridade Go/SQL. |
| Deduplicação por precedência editorial + ambiguidade estrita | [ADR-006](adr-006-deduplicacao-ambiguidade.md) | 1 candidato por `category_id`; empate em signal_type resolve por path alfabético; >1 candidato → todos ambíguos. |

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|---|---|---|
| `unaccent` indisponível no Postgres de produção | Migration falha, módulo não inicializa | Verificar pré-requisito no runbook de deploy; CI roda migrations em container idêntico. |
| Seed editorial exceder volumetria-alvo (~400 subcategorias, ~5k entradas) | Degradação de performance, cache ineficaz | Monitorar métricas de latência; revisão de PR obrigatória para migrations editoriais. |
| Colisão de UUIDv5 se namespace ou slug mudarem | IDs divergentes entre ambientes | Namespace e slugs imutáveis por contrato; teste de integração valida IDs determinísticos. |
| Consumidor cacheia versão stale por mais de 7 dias | Rollback editorial pode expor item depreciado ao consumidor | Janela mínima de coexistência documentada; ETag garante invalidação. |
| Termo ambíguos não cobertos por testes negativos | Falso positivo em produção | RF-39 obriga teste negativo por termo ambíguo; gate de CI. |
| `kind` denormalizado em `category_dictionary` sem constraint de consistência com `categories.kind` | Entrada com `kind='income'` apontando para categoria `kind='expense'` | Módulo é somente leitura; seed controlado via migration append-only; teste de integração valida `kind` consistente no seed. |

### Conformidade com Padrões

- Regra `R0`: sem `init()`.
- Regra `R1`: funções são métodos de struct (exceto `main`, factories, helpers de teste).
- Regra `R5.8`: enums com `iota + 1`.
- Regra `R5.12`: sem `panic` em produção.
- Regra `R6`: `context.Context` em toda fronteira de IO.
- Regra `R6.4`: proibido `var _ Interface = (*Type)(nil)`.
- Regra `R6.7`: proibido `clock.Clock`; usar `time.Now().UTC()` inline.
- Regra `R7`: usar `any`, `log/slog`, `errors.Join`, `slices`, `maps`, `min`/`max` conforme `go.mod` (Go 1.26.4).
- Layout obrigatório de módulo: `application/`, `domain/`, `infrastructure/`.
- Padrão de DI manual explícito em `module.go`.
- Comentários em arquivos `.go` proibidos — R-ADAPTER-001.1 [HARD] (exceções: `//go:build`, `//nolint`, cabeçalhos de geração).
- Handlers finos sem lógica de negócio — R-ADAPTER-001.2 [HARD]; fluxo `handler → usecase` obrigatório.
- Referências go-implementation carregadas conforme Matriz R-ADAPTER-001.3 (HTTP Handler: `architecture.md` + `api.md`).

### Arquivos Relevantes e Dependentes

- `.specs/prd-categories-crud/prd.md` — fonte primária de requisitos.
- `AGENTS.md` — regras de arquitetura e governança.
- `internal/identity/infrastructure/http/server/middleware/require_user.go` — middleware de autenticação reutilizado.
- `cmd/server/server.go` — composition root onde `CategoriesModule` será registrado.
- `migrations/` — local das migrations de schema e seed editorial.
- `mockery.yml` — deve ser atualizado para gerar mocks das interfaces do módulo.

### Decisões Fechadas pelo PRD (sem ADR)

- Catálogo global, somente leitura, sem personalização.
- Hierarquia de exatamente dois níveis.
- Seed append-only; rollback por depreciação + novo ID.
- Subcategorias de receita sob raiz "Investimentos" usam `allocation_type=asset_allocation`; demais receitas e subcategorias de despesa fora "Metas" e "Liberdade Financeira" usam `consumption`.
- Correspondência normalizada exata; sem fuzzy, IA ou contexto.
- `openapi.yaml` versionado no módulo.
