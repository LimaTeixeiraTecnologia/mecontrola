# Tarefa 5.0: Validação de categorias (raiz, filha direta, kind↔direction)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a integridade de categorização do CRUD unificado: categoria raiz obrigatória, subcategoria como filha direta validada, e coerência `kind`↔`direction`. Corrige de passagem dois bugs pré-existentes que impedem "0-gap": (a) `ValidateSubcategoryResult` não devolve `CategoryName`/`ParentName`/`Kind` e não valida vínculo pai-filho direto; (b) o adapter de leitura de categorias grava slug composto no `Name` do snapshot em vez do nome real. Cobre RF-17, RF-18, RF-19, RF-20, RF-21.

<requirements>
- `ValidateSubcategory` DEVE aceitar `expectedParentID uuid.UUID` e rejeitar subcategoria que não seja filha direta da raiz (`category.ParentID != nil && *category.ParentID == expectedParentID`), com novo erro `ErrSubcategoryNotDirectChild` (RF-18).
- `ValidateSubcategoryResult` DEVE devolver `CategoryName`, `ParentName` e `Kind` (além dos campos atuais), eliminando o bug de result incompleto.
- `CategorySnapshot` (`internal/transactions/application/interfaces/types.go`) DEVE ganhar `Kind` (string `income`/`expense`) e `ParentID`.
- `CategoriesCache.Validate` DEVE, no caminho com subcategoria, validar que `categoryID` é raiz (RF-17) e popular `Kind` no snapshot (o mapa de roots passa a carregar `id` + `kind`).
- `categories_reader_adapter.go` DEVE mapear `Name`/`ParentID`/`ParentName` corretamente (fim do slug composto no `Name`).
- Os guards de `direction` vivem no use case (não em `Decide*`): `outcome ⇒ subcategory obrigatória` (RF-19), `income ⇒ subcategory opcional` (RF-20), `kind == toKind(direction)` (RF-21).
- Zero comentários em Go de produção (R-ADAPTER-001.1); validação agregada por `errors.Join`; `Decide*` permanece puro (guards de categoria ficam no use case, não no workflow).
</requirements>

## Subtarefas

- [ ] 5.1 `validate_subcategory.go`: assinatura com `expectedParentID uuid.UUID`; checar filha direta; novo `ErrSubcategoryNotDirectChild`; enriquecer `ValidateSubcategoryResult` com `CategoryName`, `ParentName`, `Kind`.
- [ ] 5.2 `interfaces/types.go`: `CategorySnapshot` ganha `Kind` (string) e `ParentID`.
- [ ] 5.3 `categories_cache.go`: validar raiz de `categoryID` no caminho com subcategoria (RF-17); roots map vira `map[string]rootEntry{id,kind}`; popular `Kind` no snapshot.
- [ ] 5.4 `categories_reader_adapter.go`: corrigir mapeamento de `Name`/`ParentID`/`ParentName` (sem slug composto no `Name`).
- [ ] 5.5 Guards no use case (`create_transaction.go`, `update_transaction.go`, `helpers.go`): `outcome⇒subcategory` (RF-19), `income⇒opcional` (RF-20), `kind==toKind(direction)` (RF-21) — fora de `Decide*`.

## Detalhes de Implementação

Ver `techspec.md`:
- Seção "Validação (fronteira → smart constructor → use case)" — tabela RF-17/RF-18/RF-19/RF-20/RF-21 por camada (raiz em `CategoriesCache.Validate`; filha direta em `ValidateSubcategory(expectedParentID)`; kind↔direction e obrigatoriedade de subcategoria por `direction` no use case).
- Seção "Pontos de Integração" → `internal/categories` — `ValidateSubcategory` ganha `expectedParentID` e retorna `Kind`/`CategoryName`/`ParentName` (corrige bug de snapshot).
- Seção "Visão Geral dos Componentes" → `interfaces.CategoryValidator`/`CategorySnapshot` ganham `Kind` e `ParentID`.
- Seção "Sequenciamento de Desenvolvimento" item 5.
- Guards de `direction` no use case, nunca em `Decide*` (R-TXN-001, `Decide*` puro).

## Critérios de Sucesso

- `category_id` não-raiz → erro (RF-17).
- Subcategoria pertencente a outra raiz → erro `ErrSubcategoryNotDirectChild` (RF-18).
- `kind` da categoria ≠ `direction` da transação → erro (RF-21).
- `income` sem subcategoria → ok (RF-20); `outcome` sem subcategoria → erro (RF-19).
- Snapshots de categoria carregam `Name`/`ParentName` reais (não slug composto) e `Kind`/`ParentID` corretos.
- `Decide*` permanece puro (guards de categoria no use case); zero comentários Go; erros agregados por `errors.Join`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `ValidateSubcategory` com `expectedParentID` (filha direta → ok; subcategoria de outra raiz → `ErrSubcategoryNotDirectChild`; result carrega `CategoryName`/`ParentName`/`Kind`); `CategoriesCache.Validate` (categoria não-raiz → erro RF-17; filha direta → ok RF-18; `kind`≠`direction` → erro RF-21; `income` sem subcategory → ok RF-20); guards de use case (`outcome` sem subcategory → erro RF-19, em create e update).
- [ ] Testes de integração: cache + Postgres cobertos na Tarefa 8.0 (referenciar, não duplicar aqui).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/application/usecases/validate_subcategory.go` — `expectedParentID`, filha direta, `ErrSubcategoryNotDirectChild`, result enriquecido (`CategoryName`/`ParentName`/`Kind`).
- `internal/categories/application/usecases/resolve_by_slug.go` — porta de resolução consumida pelo cache/use case.
- `internal/categories/domain/entities/category.go` — `ParentID`, nome e vínculo pai-filho.
- `internal/categories/domain/valueobjects/kind.go` — `Kind` (income/expense) para coerência com `direction`.
- `internal/transactions/application/interfaces/category_validator.go` — porta consumida pelo use case.
- `internal/transactions/application/interfaces/types.go` — `CategorySnapshot` ganha `Kind` e `ParentID`.
- `internal/transactions/infrastructure/config/categories_cache.go` — validação de raiz (RF-17), roots `map[string]rootEntry{id,kind}`, popular `Kind`.
- `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go` — corrigir mapeamento `Name`/`ParentID`/`ParentName` (bug de snapshot de nome).
- `internal/transactions/application/usecases/create_transaction.go` — guards RF-19/RF-20/RF-21.
- `internal/transactions/application/usecases/update_transaction.go` — guards RF-19/RF-20/RF-21.
- `internal/transactions/application/usecases/helpers.go` — helper compartilhado dos guards de categoria/`direction`.
