# Tarefa 2.0: Seed editorial do catálogo completo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar migration de seed com todas as raízes e subcategorias de despesas (5 raízes) e receitas (8 raízes), usando UUIDv5 determinístico com namespace derivado do domínio. Incluir `allocation_type` correto: `asset_allocation` para "Metas", "Liberdade Financeira" e receitas sob "Investimentos"; `consumption` para demais.

<requirements>
- RF-01: IDs UUIDv5 determinísticos com namespace fixo + par `(kind, slug)`
- RF-02: raiz `parent_id=NULL`, subcategoria aponta para raiz do mesmo `kind`
- RF-03: profundidade máxima 2
- RF-04: toda classificação futura aponta para subcategoria; raízes são agrupamento
- RF-05: `allocation_type` correto por categoria
- RF-30: seed exato de despesas
- RF-31: seed exato de receitas
- RF-36: seed append-only
- RF-36a: rollback por depreciação + novo ID
- RF-38: alteração por migration versionada em PR
- RF-40: validar IDs determinísticos e constraints
- ADR-004: namespace `uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))`
</requirements>

## Subtarefas

- [ ] 2.1 Declarar `categoryNamespace` em código Go (para validação) e calcular todos os UUIDv5
- [ ] 2.2 Migration de seed de despesas: 5 raízes + subcategorias com `allocation_type`
- [ ] 2.3 Migration de seed de receitas: 8 raízes + subcategorias com `allocation_type`
- [ ] 2.4 Inserir `category_editorial_version` inicial (`version = 1`) na baseline ou nesta migration
- [ ] 2.5 Teste de integração: valida que todos os IDs são determinísticos e recalculáveis

## Detalhes de Implementação

Ver PRD seções **Seed de Despesas** e **Seed de Receitas** para lista exata.

Regras Go mandatórias para validação:
- Carregar obrigatoriamente `go-implementation`
- Carregar exemplos apenas sob demanda
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- Slugs em kebab-case PT-BR (ex: `custo-fixo`, `decimo-terceiro`).
- `allocation_type=asset_allocation` para: todas as subcategorias de "Metas" e "Liberdade Financeira", e subcategorias de receita sob "Investimentos" (`Rendimentos`, `Dividendos`, `Juros`, `Resgates`).
- Demais receitas e despesas: `consumption`.
- Seed deve ser idempotente (`ON CONFLICT (kind, slug) DO NOTHING` ou `WHERE NOT EXISTS`).

## Critérios de Sucesso

- [ ] Migration de seed aplica sem erro após baseline
- [ ] Todos os IDs são recalculáveis via `uuid.NewSHA1(categoryNamespace, []byte(kind+"+"+slug))`
- [ ] `SELECT count(*) FROM categories WHERE kind='expense'` retorna exatamente o número esperado (raízes + subcategorias)
- [ ] Índice único `(kind, slug)` não violado
- [ ] Teste de integração com seed completo passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de integração: recalcula UUIDv5 de todas as categorias e confere com banco
- [ ] Teste de integração: valida `allocation_type` de cada categoria
- [ ] Teste de integração: valida profundidade máxima 2 (nenhuma subcategoria tem filhos)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0000XX_seed_categories.up.sql`
- `migrations/0000XX_seed_categories.down.sql`
- `internal/categories/domain/entities/category.go` (factory de UUIDv5)
- Testes de integração em `migrations/` ou `internal/categories/infrastructure/repositories/postgres/`
