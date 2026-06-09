# Tarefa 1.0: Schema baseline, extensão unaccent e tabela de versão editorial

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o schema PostgreSQL do módulo categories: habilitar extensão `unaccent`, criar tabela de versão editorial, tabela de categorias com constraints de hierarquia e índices, e tabela de dicionário com coluna gerada `term_normalized` e índices de unicidade/busca.

<requirements>
- RF-06: catálogo global sem `user_id` ou propriedade por usuário
- RF-20: normalização accent-insensitive via coluna gerada
- RF-40: constraints de schema (profundidade, kind, ciclos)
- RT-09: extensão `unaccent` é dependência obrigatória
- ADR-002: tabela `category_editorial_version`
- ADR-005: coluna `term_normalized GENERATED ALWAYS AS (lower(unaccent(term))) STORED`
</requirements>

## Subtarefas

- [ ] 1.1 Migration de habilitação da extensão `unaccent`
- [ ] 1.2 Migration de DDL das tabelas `category_editorial_version`, `categories`, `category_dictionary`
- [ ] 1.3 Migration down correspondente (DROP TABLE com CASCADE se necessário)
- [ ] 1.4 Verificar que locale `pt_BR` está disponível no ambiente de teste
- [ ] 1.5 Teste de integração: aplicar up/down migrations em container Postgres sem erro

## Detalhes de Implementação

Ver techspec.md seção **Modelos de Dados** e ADR-002/ADR-005 para DDL completo.

Pontos críticos:
- `category_editorial_version` deve ter exatamente uma linha com `version = 1`.
- Índice `categories_parent_sort_idx` usa `COLLATE "pt_BR"`.
- `category_dictionary` NÃO possui coluna `version` (removido por decisão).
- Constraint `categories_parent_same_kind` garante que subcategoria aponte para raiz do mesmo `kind`.
- Constraint `categories_no_cycles` impede auto-referência.

## Critérios de Sucesso

- [ ] Migrations up aplicam sem erro em Postgres 16+ com `unaccent` habilitado
- [ ] Migrations down revertem o schema completamente
- [ ] Índices criados são usados em `EXPLAIN` das queries previstas
- [ ] Teste de integração valida schema com `testcontainers-go`, build tag `//go:build integration`

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste de integração com container Postgres: aplica migrations up, valida tabelas e índices, aplica down, valida remoção
- [ ] Teste que `unaccent` está disponível (`SELECT unaccent('á') = 'a'`)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/0000XX_categories_baseline.up.sql`
- `migrations/0000XX_categories_baseline.down.sql`
- `migrations/embed.go` (se existir, não modificar sem necessidade)
- `migrations/migrations_integration_test.go`
