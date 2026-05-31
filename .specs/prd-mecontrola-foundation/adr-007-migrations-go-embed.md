# ADR-007 — Migrations via `//go:embed`

## Metadados

- **Título:** Migrations SQL empacotadas no binário via `//go:embed migrations/*.sql`
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-03, §RF-12](./prd.md), [techspec §Arquitetura, §Plano de Rollout](./techspec.md), [`golang-migrate`](https://github.com/golang-migrate/migrate)

## Contexto

A foundation usa `golang-migrate` (entregue pelo `devkit-go/pkg/database`). Migrations podem ser fornecidas via filesystem path (default da CLI do migrate) ou via Go embedding com `//go:embed`. Em deploy Fly.io, o app roda em imagem imutável — migrations no filesystem exigem volume montado, divergindo da imagem.

## Decisão

**Migrations SQL ficam em `migrations/*.sql` na raiz e são empacotadas no binário via `//go:embed migrations/*.sql`** no pacote `internal/infrastructure/database`. O helper `RunMigrations(ctx, manager)` aplica as embarcadas.

Conventional naming: `NNNN_<slug>.up.sql` + `NNNN_<slug>.down.sql` (4 dígitos zero-padded).

## Alternativas Consideradas

1. **Filesystem path montado via Fly volume**.
   - Vantagens: hotfix sem rebuild; permite editar SQL "rápido" em prod.
   - Desvantagens: divergência silenciosa entre volume e imagem; rollback de imagem não rollbacka volume; aumenta superfície de ataque (escrita em volume); viola R-SEC-001 §Filesystem ("toda escrita intencional e auditável").
2. **Híbrido: embed por default, override via `MIGRATIONS_PATH` env**.
   - Vantagens: flexível para debug local.
   - Desvantagens: dobra superfície de teste; cria caminho não-testado em prod; YAGNI no MVP.
3. **CLI separada `migrate` chamando migrate binário oficial**.
   - Vantagens: zero código Go; usa ferramenta oficial.
   - Desvantagens: dependência adicional no runner; sincronizar versão da CLI + lib é fricção; piora rollback (binário e SQL desacoplados).

## Consequências

### Benefícios Esperados

- Imagem do binário é fonte única de verdade: rollback de imagem ⇒ rollback de SQL coerente.
- Sem volume + sem dependência externa em runtime.
- `task migrate:up` local usa o mesmo SQL que prod (paridade).
- Aderente ao critério de rollback do PRD (`fly releases rollback` rebobina SQL).

### Trade-offs e Custos

- Mudança de SQL exige novo build/release (sem hotfix de SQL).
- Binário um pouco maior (alguns KB por migration; desprezível).

### Riscos e Mitigações

- **Risco:** dev edita `.sql` mas esquece `go build` ⇒ binário antigo aplica versão antiga.
  - **Mitigação:** `task build` é declarado com `sources: migrations/*.sql + **/*.go` ⇒ Task invalida cache automaticamente; integration test detecta divergência.
- **Risco:** migration acidentalmente irreversível (sem `.down.sql` correspondente).
  - **Mitigação:** lint custom no `task lint` confere par up/down; CI bloqueia.
- **Risco:** SQL muito grande inflar binário.
  - **Mitigação:** convenção: data seeds não vão em migration (vão em PRD do módulo respectivo via use case); cap soft de 50 KB por arquivo.

## Plano de Implementação

1. Criar diretório `migrations/` na raiz.
2. `internal/infrastructure/database/migrations.go`: `//go:embed migrations/*.sql` + `fs.FS` exposto + `RunMigrations(ctx, m *Manager) error` usando `golang-migrate/source/iofs`.
3. Primeira migration `0001_init.up.sql` + `0001_init.down.sql` criando `health_probe`.
4. Integration test em `database_integration_test.go` valida up/down.
5. `taskfiles/migrations.yml` ou subseção em `taskfiles/build.yml` com `task migrate:up`/`migrate:down` consumindo binário compilado.

## Monitoramento e Validação

- Log `info` em cada migration aplicada com nome e duração.
- Métrica `migrations_applied_total` (counter, com label `version`).
- Métrica `migrations_apply_duration_seconds` (histogram).
- Alerta: migração que demora >30 s.

## Impacto em Documentação e Operação

- Runbook "Aplicar migration manual em prod" (caso edge): documentar `fly ssh console -C "/app/server --migrate-only"` (a confirmar com flag) + cuidados.
- Runbook "Restore PITR": ordem é restore → deploy versão compatível (rebobina SQL via imagem).

## Revisão Futura

- Revisitar quando o ciclo de release for muito longo e SQL hotfix se tornar gargalo (improvável no MVP).
- Revisitar quando schema crescer >100 migrations (avaliar split por módulo via múltiplos `embed`).
