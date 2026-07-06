# Tarefa 3.0: Reforcar Baseline SQL com Defesa Canonica

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Alterar o baseline SQL do banco novo para que `transactions` e `transactions_recurring_templates` tenham defesa em profundidade contra falso positivo mesmo se um use case for bypassado. Esta tarefa deve adicionar colunas de evidencia, FKs, checks e triggers semanticos no baseline.

<requirements>
RF-09, RF-10, RF-11, RF-15, RF-16, RF-18, RF-19, RF-24, RF-30, RF-35.
RNF-01, RNF-04.
CA-06, CA-07, CA-08, CA-13, CA-15, CA-19, CA-20.
</requirements>

## Subtarefas

- [ ] 3.1 Tornar `subcategory_id` e `subcategory_name_snapshot` obrigatorios em transacoes e templates recorrentes.
- [ ] 3.2 Adicionar colunas normalizadas de evidencia categorial definidas em `techspec.md`.
- [ ] 3.3 Adicionar FKs de `category_id` e `subcategory_id` para `mecontrola.categories(id)`.
- [ ] 3.4 Adicionar checks de outcome, score, kind, confidence, match quality, signal type, decision source e editorial version.
- [ ] 3.5 Criar funcao compartilhada de validacao semantica para raiz, folha direta, kind/direction, deprecated, version drift e textos vazios.
- [ ] 3.6 Criar triggers para `mecontrola.transactions` e `mecontrola.transactions_recurring_templates`.
- [ ] 3.7 Atualizar down migration do baseline de forma coerente com a estrutura atual.
- [ ] 3.8 Criar testes de migration/integration que tentem bypassar a aplicacao.

## Detalhes de Implementação

Seguir `techspec.md`, secoes "Estado Real das Migrations de Categorias" e "Persistencia". Como o banco e novo, alterar `migrations/000001_initial_schema.up.sql` e o down correspondente. Nao criar backfill.

## Critérios de Sucesso

- Banco rejeita IDs inexistentes por FK.
- Banco rejeita raiz como subcategoria, leaf de outra raiz, kind incompatível, deprecated e version drift.
- Banco rejeita evidencia textual vazia, enum invalido e score/version fora do intervalo.
- Baseline preserva a ordem real das tabelas de categorias e transacoes.
- Teste prova que mock ou bypass SQL nao consegue gravar estado invalido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes de migration/integration com build tag usada no projeto.
- [ ] Casos negativos para `subcategory_id NULL`, root como leaf, leaf de outra raiz, kind/direction incompatível, deprecated root/leaf, `category_kind` divergente e version drift.
- [ ] Casos negativos para source/confidence/quality/signal/outcome invalidos.
- [ ] Caso positivo para despesa e receita com evidencia completa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/000001_initial_schema.up.sql`
- `migrations/000001_initial_schema.down.sql`
- `migrations/migrations_integration_test.go`
