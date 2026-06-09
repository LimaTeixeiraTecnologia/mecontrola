# Tarefa 8.0: CategoriesModule, wiring e registro em cmd/server/server.go

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar `CategoriesModule` com DI manual explícita (padrão `InvoiceModule`/`IdentityModule`): construtor que recebe `manager.Manager` e `observability.Observability`, monta repositórios, use cases, handlers e router, e expõe `CategoryRouter`. Registrar o módulo em `cmd/server/server.go`.

<requirements>
- RT-03: layout e wiring manual definidos em AGENTS.md
- Padrão obrigatório de módulo: DI manual, struct concreta, campos nomeados
- R-ADAPTER-001: wiring segue `repository → usecase → handler → router`
</requirements>

## Subtarefas

- [ ] 8.1 Criar `internal/categories/module.go` com `CategoriesModule` e `NewCategoriesModule(mgr, o11y)`
- [ ] 8.2 Implementar wiring: repository factory (ou repositórios diretos) → use cases → handlers → router
- [ ] 8.3 Atualizar `cmd/server/server.go` para instanciar e registrar `CategoriesModule`
- [ ] 8.4 Validar que servidor sobe sem erro e rotas respondem

## Detalhes de Implementação

Ver techspec.md seção **Arquitetura do Sistema** e referenciar `internal/billing/module.go` / `internal/identity/module.go` como padrão.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/architecture.md`
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- `NewCategoriesModule(mgr manager.Manager, o11y observability.Observability) CategoriesModule`
- Sem struct de configuração (decisão consciente do MVP).
- `CategoriesModule` expõe apenas `CategoryRouter` (e opcionalmente repositórios se outros módulos precisarem).
- Wiring: `repo := postgres.NewCategoryRepository(mgr, o11y)` → `uc := usecases.NewListCategories(repo, versionReader, o11y)` → `handler := handlers.NewListCategoriesHandler(uc, o11y)` → `router := server.NewCategoryRouter(...handlers..., o11y)`.
- Em `cmd/server/server.go`: `categoriesModule := categories.NewCategoriesModule(dbManager, o11y)` e `srv.RegisterRouters(categoriesModule.CategoryRouter)`.

## Critérios de Sucesso

- [ ] `go build ./...` passa sem erro
- [ ] Servidor sobe e responde em `/api/v1/categories` e `/api/v1/category-dictionary`
- [ ] `CategoriesModule` segue padrão de `billing` e `identity`
- [ ] Sem `init()`, sem `panic`, sem comentários em `.go`

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `go build ./...` e `go vet ./...`
- [ ] Teste de integração E2E: servidor sobe, rota `/api/v1/categories?kind=expense` retorna 200 com árvore
- [ ] Teste de integração E2E: rota sem auth retorna 401

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/module.go`
- `cmd/server/server.go`
- `configs/config.go` (se precisar adicionar algo, mas provavelmente não)
