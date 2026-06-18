# Plano: Testes para ValidateSubcategory e ResolveBySlug

## Context

O arquivo `docs/melhorias/2026-06-17-categories.md` descreve melhorias para o módulo `internal/categories`. A análise do codebase identificou que dois use cases estão completamente sem cobertura de teste:

- `ValidateSubcategory` — valida que um UUID referencia uma subcategoria (não raiz) e retorna o slug normalizado do pai
- `ResolveBySlug` — mapeia slugs de categorias raiz para seus UUIDs

Ambos os use cases existem e funcionam em produção, mas não têm nenhum arquivo `_test.go`. Todos os demais use cases do módulo (`GetCategory`, `ListCategories`, `ListDictionary`, `SearchDictionary`) têm testes com `testify/suite`. A criação desses dois arquivos de teste fecha a lacuna de cobertura descrita nas regras de implementação do documento de melhorias.

## Skill obrigatória

`.agents/skills/go-implementation/SKILL.md` — carregada antes de qualquer edição.

## Arquivos a criar

### 1. `internal/categories/application/usecases/validate_subcategory_test.go`

**Suite:** `ValidateSubcategorySuite` / runner `TestValidateSubcategorySuite`

**Campos:**
```go
type ValidateSubcategorySuite struct {
    suite.Suite
    ctx     context.Context
    repo    *mockInterfaces.CategoryRepository
    useCase *usecases.ValidateSubcategory
}
```

**SetupTest:**
```go
s.ctx = context.Background()
s.repo = mockInterfaces.NewCategoryRepository(s.T())
s.useCase = usecases.NewValidateSubcategory(s.repo, noop.NewProvider())
```

**Cenários:**

| Método | Mock sequence | Assertion chave |
|--------|---------------|----------------|
| `TestExecute_SubcategoriaAtiva` | `GetByID(subID)→category(ParentID=&rootID, active)`, `GetByID(rootID)→parent(Slug="custo-fixo", Kind=Expense)` | `result.ParentSlug == "expense.custo_fixo"`, `result.Deprecated == false` |
| `TestExecute_SubcategoriaDeprecada` | `GetByID(subID)→category(ParentID=&rootID, DeprecatedAt=&now)`, `GetByID(rootID)→parent` | `result.Deprecated == true` |
| `TestExecute_CategoriaEhRaiz` | `GetByID(id)→category(ParentID=nil)` | `errors.Is(err, usecases.ErrSubcategoryNotRoot)` |
| `TestExecute_CategoriaNaoEncontrada` | `GetByID(id)→{}, interfaces.ErrNotFound` | `errors.Is(err, usecases.ErrCategoryNotFound)` |
| `TestExecute_PaiNaoEncontrado` | `GetByID(subID)→category(ParentID=&rootID)`, `GetByID(rootID)→{}, errors.New("db error")` | `s.Error(err)`, `s.Contains(err.Error(), "buscar categoria pai")` |

**Normalização de slug:** `buildRootSlug` converte `-` em `_`. Para `Kind=KindExpense`, `Slug="custo-fixo"` → `"expense.custo_fixo"`.

**Imports:**
```go
"context" / "errors" / "testing" / "time"
"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
"github.com/google/uuid"
"github.com/stretchr/testify/suite"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces"
mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
```

---

### 2. `internal/categories/application/usecases/resolve_by_slug_test.go`

**Suite:** `ResolveBySlugSuite` / runner `TestResolveBySlugSuite`

**Campos:**
```go
type ResolveBySlugSuite struct {
    suite.Suite
    ctx     context.Context
    repo    *mockInterfaces.CategoryRepository
    useCase *usecases.ResolveBySlug
}
```

**SetupTest:**
```go
s.ctx = context.Background()
s.repo = mockInterfaces.NewCategoryRepository(s.T())
s.useCase = usecases.NewResolveBySlug(s.repo, noop.NewProvider())
```

**Cenários:**

| Método | Mock `repo.List` retorna | Input | Assertion chave |
|--------|--------------------------|-------|----------------|
| `TestExecute_SlugUnico` | `[root(Slug="custo-fixo", ID=rootID)]` | `["custo-fixo"]` | `result["custo-fixo"] == rootID` |
| `TestExecute_MultiplosSlugs` | dois roots com slugs distintos | `["custo-fixo","renda"]` | `len(result)==2`, IDs corretos |
| `TestExecute_SlicesVazia` | qualquer (ou vazio) | `[]string{}` | `s.Empty(result)`, sem erro |
| `TestExecute_SlugNaoEncontrado` | root diferente do solicitado | `["inexistente"]` | `errors.Is(err, usecases.ErrCategoryNotFound)` |
| `TestExecute_SubcategoriasIgnoradas` | root + sub com `ParentID!=nil` | `["aluguel"]` (slug do sub) | `errors.Is(err, usecases.ErrCategoryNotFound)` |
| `TestExecute_ErroNoRepositorio` | `nil, errors.New("db error")` | qualquer | `s.Error(err)`, `s.Contains(err.Error(), "listar categorias")` |

**Detalhe:** `repo.List` é chamado com `interfaces.CategoryQuery{IncludeDeprecated: false}`. Nos cenários onde a query exata não é relevante, usar `mock.Anything`. No cenário `TestExecute_SlicesVazia` o mock ainda deve ser configurado (o UC sempre chama `List`, mesmo sem slugs).

**Imports:**
```go
"context" / "errors" / "testing"
"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
"github.com/google/uuid"
"github.com/stretchr/testify/mock"
"github.com/stretchr/testify/suite"
mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/interfaces/mocks"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/application/usecases"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/entities"
"github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories/domain/valueobjects"
```

---

## Arquivos de referência

- `internal/categories/application/usecases/get_category_test.go` — padrão canônico de suite + mock
- `internal/categories/application/usecases/validate_subcategory.go` — implementação a testar
- `internal/categories/application/usecases/resolve_by_slug.go` — implementação a testar
- `internal/categories/application/interfaces/mocks/category_repository.go` — mock gerado (mockery)

## Regras hard aplicáveis

- **R-ADAPTER-001.1**: zero comentários em ambos os arquivos (nem `// arrange`, nem `// Cenário`)
- **R6**: `context.Context` via `context.Background()` no `SetupTest`
- **R7.6**: `errors.Is` para asserções de sentinel errors
- Sem `var _ Interface = (*Type)(nil)`
- Sem `time.Now()` fora de `entities.Category{DeprecatedAt: &now}` (variável local no teste)
- UUIDs fixos: `uuid.MustParse("11111111-1111-1111-1111-111111111111")` e `"22222222-..."`

## Verificação

```bash
go test ./internal/categories/application/usecases/... -v -run 'TestValidateSubcategorySuite|TestResolveBySlugSuite'

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" internal/categories/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" || echo "OK"

go build ./internal/categories/...
```
