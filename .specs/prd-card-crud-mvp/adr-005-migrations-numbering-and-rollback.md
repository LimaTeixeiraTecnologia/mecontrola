# Registro de Decisão Arquitetural (ADR-005)

## Metadados

- **Título:** Numeração de migrations padronizada em 6 dígitos e `down` por rename
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Jailton (tech lead)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (RF-17–RF-20), `.specs/prd-card-crud-mvp/techspec.md`, `migrations/embed.go`

## Contexto

O PRD descreve `0010_create_platform_idempotency_keys` e `0011_create_card_cards`. Inspecionando `migrations/`:

- Convenção vigente é 6 dígitos: `000001_initial_baseline`, `000002_seed_smoke_user_staging`, `000003_categories_unaccent`.
- `migrations/embed.go` aplica `//go:embed *.sql` sem prefixo específico; golang-migrate ordena lexicograficamente.

Adotar `0010_*`/`0011_*` quebra a ordenação (`0010 < 000001` lexicograficamente). Há também a obrigação de `down` preservar dados via rename (RF-18).

## Decisão

- Adotar **6 dígitos** vigentes: `000004_create_platform_idempotency_keys.{up,down}.sql` e `000005_create_card_cards.{up,down}.sql`.
- `down` SEMPRE por rename: `ALTER TABLE mecontrola.<t> RENAME TO <t>_archived_<timestamp_estatico>;` + `DROP INDEX IF EXISTS …`. Sem `DROP TABLE` direto.
- Suffix `<timestamp_estatico>` é o timestamp UTC do momento da migration (string literal embutida no SQL — não calculada em runtime), garantindo idempotência e auditoria.

## Alternativas Consideradas

1. **Manter 4 dígitos como o PRD pediu** — Vantagens: alinhamento literal com o PRD; Desvantagens: quebra ordering de golang-migrate, exige refactor de `000001..000003`, alto risco operacional. Rejeitada.
2. **`DROP TABLE` na `down`** — Vantagens: schema limpo; Desvantagens: perde dados em rollback, contradiz RF-18 e cultura do repo. Rejeitada.
3. **Suffix dinâmico (`now()::text`) no rename** — Vantagens: nome único por execução; Desvantagens: migrations não-determinísticas, integration test instável, dificulta replay. Rejeitada.

## Consequências

### Benefícios Esperados

- Ordering consistente com o `embed.go` e com o golang-migrate.
- Rollback preserva dados; cliente pode reverter rename manualmente se necessário.
- Auditoria via nome arquivado.

### Trade-offs e Custos

- Drift documental frente ao PRD (D-02) — registrado e justificado neste ADR.
- Suffix estático significa que múltiplas execuções de `down` em ambientes diferentes geram o mesmo nome de arquivo — se for re-aplicada após rename, segunda execução falha (esperado; runbook documenta).

### Riscos e Mitigações

- **Aplicação parcial em ambiente de homologação** → `up` idempotente (`IF NOT EXISTS`) reduz risco; `down` documentado no runbook.
- **Confusão entre PRD (0010/0011) e código (000004/000005)** → README dentro de `.specs/prd-card-crud-mvp/` (próximo passo opcional) registra drift; spec-hash garante rastreabilidade.

## Plano de Implementação

1. Criar `000004_create_platform_idempotency_keys.{up,down}.sql`.
2. Criar `000005_create_card_cards.{up,down}.sql`.
3. Estender `migrations/migrations_integration_test.go` para cobrir `up`/`down`/`up` das novas.
4. Documentar nome dos arquivos arquivados no `docs/runbooks/card-rollback.md`.

Adoção concluída quando: `make migrate-up` aplica as duas + integration test verde.

## Monitoramento e Validação

- `migrations_integration_test.go` valida `up`/`down`/`up`.
- Critério de revisão: 0 incidente de migration falhando em homolog/produção nas primeiras 4 semanas.

## Impacto em Documentação e Operação

- `docs/runbooks/card-rollback.md` — comandos `migrate down` + verificação de tabelas `*_archived_*`.
- Onboarding: registrar convenção de 6 dígitos no `AGENTS.md` (proposta de PR follow-up).

## Revisão Futura

Revisitar quando:

- Repositório migrar de golang-migrate para outra ferramenta.
- Convenção de numeração mudar (ex.: timestamp puro `20260101120000_`).
