# Registro de Decisão Arquitetural (ADR-001)

## Metadados

- **Título:** Pacote genérico `internal/platform/idempotency/` com PK `(scope, key, user_id)`
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Jailton (tech lead), Codex/Claude (apoio de arquitetura)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (F-04, RF-30–RF-32), `.specs/prd-card-crud-mvp/techspec.md`

## Contexto

O PRD exige idempotência exatamente-uma-vez para `POST/PUT/DELETE /api/v1/cards`. O módulo `internal/billing` já implementa um esquema próprio para webhooks Kiwify (`processed_event_repository`), com schema/contrato distintos. Reaproveitar o pacote de billing acopla outro bounded context ao `card`; criar uma quarta implementação local quebra o princípio de plataforma compartilhada do `AGENTS.md` ("proibido criar implementações locais redundantes de capacidades transversais").

Restrições:

- O middleware precisa atuar antes do handler e capturar resposta para replay.
- Não pode haver `init()`, `panic` em runtime, abstração de relógio (R6.7) ou comentários em código Go (`R-ADAPTER-001.1`).
- A migração futura de `billing` e `identity` para o pacote é planejada como dívida controlada (PRD OBJ-04).

## Decisão

Criar `internal/platform/idempotency/` expondo:

- `Storage` interface (`Get`, `Put`) + tipo `Record`.
- `PostgresStorage` baseada em `database.DBTX` (pgx puro), com `INSERT … ON CONFLICT DO NOTHING RETURNING` para resolver corrida de inserção.
- `Middleware(scope string, storage Storage, ttl time.Duration, o11y observability.Observability) func(http.Handler) http.Handler` que:
  - exige header `Idempotency-Key` (1–128 ASCII);
  - calcula `request_hash` por SHA-256 do body;
  - em hit com hash igual → replica `(status, body)` armazenados;
  - em hit com hash divergente → 409 `idempotency_conflict`;
  - em miss → executa handler em `responseRecorder`, persiste resultado se `2xx`.
- Tabela compartilhada `mecontrola.idempotency_keys` com PK `(scope, key, user_id)` + índice por `expires_at`.

Escopo MVP: somente `card` consome (`scope = "card"`). Migração de `billing`/`identity` é fase 2.

**Split de responsabilidade middleware × use case** (detalhado em [ADR-006](adr-006-idempotency-atomicity-via-uow.md)): middleware é pre-check/replay/conflict + cache best-effort de 4xx; use case grava `Storage.Put` de respostas 2xx dentro da mesma `uow.UnitOfWork` da escrita de negócio. Resultado é exactly-once real em 2xx.

## Alternativas Consideradas

1. **Idempotência local por módulo (status quo de billing)** — Vantagens: zero impacto cross-module; Desvantagens: duplicação, drift de schema, contradiz `AGENTS.md`. Rejeitada.
2. **Reusar o pacote de billing** — Vantagens: nenhuma migration nova; Desvantagens: acoplamento entre bounded contexts, schema otimizado para webhook (sem `user_id` como parte da PK), risco de regressão no fluxo Kiwify. Rejeitada.
3. **Redis como storage** — Vantagens: TTL nativo, sem migration; Desvantagens: adiciona infraestrutura nova; perde durabilidade transacional (não compartilha tx com o write do recurso). Rejeitada para MVP.

## Consequências

### Benefícios Esperados

- Reutilização cross-módulo a partir de fase 2 (billing/identity).
- Storage transacional Postgres já em uso; sem nova infra.
- Contrato minimalista (`Get`/`Put`) facilita troca futura por Redis se volume justificar.

### Trade-offs e Custos

- Necessidade de uma migration adicional e responsabilidade de cleanup (em fase 2).
- `response_body` como `BYTEA` adiciona overhead em respostas grandes; limite implícito ~64 KB no buffer do middleware.

### Riscos e Mitigações

- **Race de mesma key** → `INSERT ON CONFLICT DO NOTHING RETURNING` + replay via `SELECT`. Teste de integração com 10 goroutines.
- **Crescimento da tabela** → TTL 24h + índice `expires_at`. Volume estacionário ~10k linhas.
- **Hash mismatch falso-positivo** (cliente envia mesma key para payload diferente) → 409 com mensagem clara em pt-BR.

## Plano de Implementação

1. Migration `000004_create_platform_idempotency_keys.{up,down}.sql`.
2. `Storage`/`PostgresStorage`/`Middleware`/`responseRecorder` em `internal/platform/idempotency/`.
3. Mocks via mockery.
4. Testes unitários + integração com testcontainers.
5. Consumo no `internal/card` (POST/PUT/DELETE).

Adoção concluída quando: testes verdes + uso real em `internal/card` + ausência de drift no schema entre golden e migration.

## Monitoramento e Validação

- Métrica `http_server_*` por rota `/api/v1/cards*` para taxa de replay (deve crescer linearmente com retentativas reais).
- Log estruturado `card.idempotency.replay` com `outcome=replay|miss|conflict`.
- Dashboard "Card Module" — painel "Idempotency outcomes".
- Critério de revisão: se replay > 5% das requisições por > 30 dias, investigar cliente abusando da key.

## Impacto em Documentação e Operação

- `docs/runbooks/card-rollback.md` cobre rename das tabelas em rollback.
- Onboarding técnico: novo pacote `internal/platform/idempotency/` documentado em `AGENTS.md` (entrada na seção "Plataforma Compartilhada").

## Revisão Futura

Revisitar quando:

- Volume da tabela > 100k linhas em estado estacionário (sinal de necessidade do job de cleanup).
- 2º módulo (billing ou identity) for migrado para o pacote — possível necessidade de extensão do contrato (`Delete`, métricas embutidas).
