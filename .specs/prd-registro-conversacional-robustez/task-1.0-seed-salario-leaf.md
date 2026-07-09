# Tarefa 1.0: Seed folha income `Salário > Salário` + dicionário

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a folha income de salário-base sob a raiz `Salário` e os termos de reconhecimento no
`category_dictionary`, via nova migração de seed idempotente, para que "Recebi meu salário" resolva
sem clarify. Ver ADR-001.

<requirements>
- RF-01: folha income "Salário > Salário" sob a raiz `Salário`, distinta de Décimo Terceiro, Férias,
  PLR e Bônus, Vale-Alimentação e Vale-Refeição.
- RF-02: `category_dictionary` reconhece "salário", "salario", "meu salário", "recebi salário",
  "recebi meu salário" com `kind=income`, `confidence=high`, `is_ambiguous=false`.
- RF-03: "Recebi meu salário de R$ X" resolve para a folha de salário-base, `direction=income`.
- RF-04: folha global (sem `user_id`), disponível após a migração de seed.
- RF-05: "13º salário" continua apontando para "Salário > Décimo Terceiro".
</requirements>

## Subtarefas

- [ ] 1.1 Computar o UUID v5 da folha por `uuid.NewSHA1(categoryNamespace, []byte("income:salario-base"))`
  (`categoryNamespace = uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))`) e os UUIDs dos
  termos por `dict:income:salario-base:<term_normalized>`.
- [ ] 1.2 Escrever `migrations/NNNNNN_add_salario_base_leaf.up.sql`: INSERT da folha (slug
  `salario-base`, name `Salário`, kind `income`, parent_id `86dd34b0-7342-525a-9a30-b1b5a76b109f`,
  allocation_type `consumption`) com `ON CONFLICT (kind, slug) DO NOTHING`; INSERT dos termos com
  `ON CONFLICT (id) DO UPDATE`; `UPDATE category_editorial_version SET version = version + 1`.
- [ ] 1.3 Escrever o `.down.sql` (remover termos + folha; decrementar a versão).
- [ ] 1.4 Estender `migrations/migrations_integration_test.go` cobrindo os critérios de aceite.

## Detalhes de Implementação

Ver techspec.md seção "Modelos de Dados" (schema seed) e ADR-001. Respeitar `UNIQUE (kind, slug)`
(slug distinto do root). Não reapontar aliases de Décimo Terceiro.

## Critérios de Sucesso

- Folha `salario-base` (display "Salário") existe sob a raiz Salário, kind income.
- Dicionário resolve os 5 termos para a folha; Décimo Terceiro inalterado.
- Migração idempotente (reexecução sem erro); `category_editorial_version` incrementado.
- Teste de drift de UUID v5 permanece verde.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `postgresql-production-standards` — migração de seed idempotente, constraints e editorial version em PostgreSQL.
- `domain-modeling-production` — taxonomia income como tipo fechado (kind/confidence/signal_type) e invariante raiz→folha.
- `design-patterns-mandatory` — gate `não aplicar padrão` para seed determinístico (sem estrutura nova).
- `mastra` — a folha alimenta a resolução consumida pelo agente; validar via binding no integration test.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `migrations/NNNNNN_add_salario_base_leaf.{up,down}.sql` (novo)
- `migrations/migrations_integration_test.go`
- `internal/categories/infrastructure/repositories/postgres/uuidv5_namespace_integration_test.go`
- `internal/categories/application/usecases/resolve_category_for_write.go`
