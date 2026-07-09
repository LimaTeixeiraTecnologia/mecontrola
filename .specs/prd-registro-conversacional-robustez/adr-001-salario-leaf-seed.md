# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Folha income "Salário > Salário" com slug distinto e seed idempotente
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Time de plataforma MeControla
- **Relacionados:** PRD `.specs/prd-registro-conversacional-robustez/prd.md` (RF-01..RF-05), techspec.md, US `docs/us/2026-07-08-us-registro-conversacional-robustez.md`

## Contexto

No incidente de produção (usuário `f56e1142`, 2026-07-08), "Recebi meu salário" não resolvia para
uma folha income de salário-base porque ela não existe na taxonomia. As folhas atuais sob a raiz
`Salário` (`86dd34b0-7342-525a-9a30-b1b5a76b109f`, kind income) são apenas: Décimo Terceiro, Férias,
PLR e Bônus, Vale-Alimentação e Vale-Refeição (`migrations/000001_initial_schema.up.sql:1213-1363`).
O `category_dictionary` só reconhece aliases de Décimo Terceiro (`:1491-1494`).

Restrição estrutural crítica descoberta na exploração: a tabela `mecontrola.categories` tem
`CONSTRAINT categories_kind_slug_uniq UNIQUE (kind, slug)` (`:432-444`). A raiz `Salário` já ocupa
`(kind=income, slug=salario)`. Uma folha nova com o mesmo slug `salario` colidiria e seria
**silenciosamente descartada** pelo `ON CONFLICT (kind, slug) DO NOTHING` usado no seed idempotente.

Existe também a tabela `category_editorial_version` (version BIGINT); `ResolveCategoryForWrite`
valida `ExpectedVersion` contra ela (`resolve_category_for_write.go`), e o pending workflow captura
`state.CategoryVersion` no classify e revalida no write.

## Decisão

Criar, via nova migração de seed idempotente, a folha income de salário-base:

- `id`: UUID v5 determinístico pela convenção **testada** do projeto —
  `uuid.NewSHA1(categoryNamespace, []byte("income:salario-base"))`, com
  `categoryNamespace = uuid.NewSHA1(uuid.Nil, []byte("mecontrola.io/categories"))`
  (`internal/categories/infrastructure/repositories/postgres/uuidv5_namespace_integration_test.go:15,48`;
  guardada por `TestSeedIDsAreDeterministicRecomputable`). O valor literal computado deve ser fixado
  na migração e coberto pelo teste de drift.
- `slug`: **`salario-base`** (distinto do root `salario`, evitando a colisão em `UNIQUE(kind, slug)`).
- `name`: **`Salário`** (o display é "Salário > Salário"; `UNIQUE` é sobre slug, não name).
- `kind`: `income`.
- `parent_id`: `86dd34b0-7342-525a-9a30-b1b5a76b109f` (raiz Salário).
- `allocation_type`: `consumption`.

Semear no `category_dictionary` (kind=income, apontando para a folha `salario-base`), com
`signal_type` apropriado e `confidence=high`, `is_ambiguous=false`:

- canonical_name: `salario`.
- aliases/phrases: `salário`, `meu salário`, `recebi salário`, `recebi meu salário` (mais variações
  sem acento cobertas pela coluna gerada `term_normalized` via `immutable_unaccent`).

IDs das entradas do dicionário: usar v5 determinístico próprio por termo —
`uuid.NewSHA1(categoryNamespace, []byte("dict:income:salario-base:"+term_normalized))` — em vez do
padrão hand-authored frágil das seeds legadas (id ≈ category_id com nibble final variado, ex.:
`...465d`/`...465a`/`...465e` em `migrations/000001:1491-1494`), garantindo unicidade e
recomputabilidade sem colisão com a coluna `id` da folha.

A migração incrementa `category_editorial_version` em 1. O arquivo `.down.sql` remove as entradas do
dicionário e a folha, e decrementa a versão. Segue o naming `NNNNNN_add_salario_base_leaf.{up,down}.sql`
e o padrão idempotente existente (`ON CONFLICT (kind, slug) DO NOTHING` para categoria;
`ON CONFLICT (id) DO UPDATE` para dicionário).

Décimo Terceiro permanece intacto: nenhum alias de "13º"/"décimo terceiro" é reapontado (RF-05).

## Alternativas Consideradas

1. **Reutilizar slug `salario` na folha** — Descartada: colide com o root em `UNIQUE(kind, slug)`;
   `ON CONFLICT DO NOTHING` descartaria a folha em silêncio, reproduzindo o gap. Falha silenciosa é
   exatamente o que o PRD combate.
2. **Renomear/mover a raiz Salário para virar a própria folha** — Descartada: quebraria FKs das
   folhas existentes (Décimo Terceiro etc.) e a semântica raiz→folha exigida por `ResolveForWrite`
   (`ErrRootWithoutLeaf`).
3. **Resolver salário só no dicionário apontando para uma folha existente** — Descartada: nenhuma
   folha existente representa salário-base; apontar para Décimo Terceiro/PLR seria semanticamente
   errado e contamina relatórios.

## Consequências

### Benefícios Esperados

- "Recebi meu salário" resolve deterministicamente para a folha correta, sem loop de clarify (RF-03).
- Seed idempotente e global (sem `user_id`), disponível a todos os usuários após a migração (RF-04).
- Décimo Terceiro preservado (RF-05).

### Trade-offs e Custos

- Incremento de `category_editorial_version` invalida `CategoryVersion` de pending entries suspensos
  criados antes da migração — esses resumes podem sofrer `ErrVersionDrift` e pedir reclassificação.
  Impacto restrito à janela de deploy; aceitável.
- Slug `salario-base` diverge do display `Salário`; documentar para evitar confusão futura.

### Riscos e Mitigações

- **Risco:** alias de salário capturar "13º salário" por sobreposição de termos. **Mitigação:**
  termos de salário-base não incluem "13"/"décimo"; precedência de `signal_type`
  (`canonical_name > alias > phrase`) e `match_quality` favorecem o match exato de Décimo Terceiro.
  Coberto por critério de aceite "Décimo terceiro continua na subcategoria específica".
- **Rollback:** `.down.sql` remove folha+termos e decrementa a versão.

## Plano de Implementação

1. Gerar UUID v5 determinístico para a folha `salario-base`.
2. Escrever `NNNNNN_add_salario_base_leaf.up.sql` (categoria + termos + bump de versão) e o `.down.sql`.
3. Estender `migrations/migrations_integration_test.go` cobrindo: folha existe sob a raiz Salário;
   dicionário resolve os termos para a folha; Décimo Terceiro inalterado; reexecução idempotente.
4. Validar via `ResolveCategoryForWrite` que a folha passa nas 7 validações (root≠leaf, kind match,
   não deprecated, leaf pertence ao root).

## Monitoramento e Validação

- Teste real-LLM: "Recebi meu salário de R$ 13.874,40" → resolve `Salário > Salário`, income,
  1387440 centavos, sem clarify.
- Teste real-LLM: "recebi meu 13º salário" → `Salário > Décimo Terceiro`.
- Migration integration test idempotente.

## Impacto em Documentação e Operação

- Atualizar documentação de taxonomia (se houver) com a folha `salario-base` (display "Salário").
- Runbook de categorias: nota sobre bump de editorial version na migração.

## Revisão Futura

- Revisitar se surgir personalização de taxonomia por usuário (hoje fora de escopo) ou se novas
  folhas de renda-base forem necessárias.
