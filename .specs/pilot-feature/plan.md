# Feature Piloto Production-Ready

## Objetivo

Executar uma feature pequena end-to-end usando o novo fluxo obrigatorio de skills e gates, provando que a governanca funciona na pratica e coletando metricas para a baseline.

## Feature escolhida

**Cadastro de dias de carencia por banco (BankDays)**

Motivo:
- Pequena e bem delimitada.
- Toca em dominio (BankDays como value object/entidade).
- Toca em PostgreSQL (nova tabela `bank_days`).
- Toca em design pattern (Repository/Factory para persistencia).
- Pode tocar em Docker Swarm se houver necessidade de seed/populacao inicial.

## Fluxo obrigatorio

### Fase 1 — Modelagem de dominio (`domain-modeling-production`)

- Carregar `.agents/skills/domain-modeling-production/SKILL.md`.
- Inicializar bundle: `python3 scripts/init-bundle.py bank-days`.
- Preencher `domain-model.md` com:
  - Linguagem ubiqua (banco, codigo do banco, dias de carencia).
  - Comando `RegisterBankDays`.
  - Evento `BankDaysRegistered`.
  - Invariante: dias de carencia >= 0.
  - Erros de dominio.
- Validar: `python3 scripts/validate-bundle.py discoveries/domain-bank-days`.

### Fase 2 — Design pattern (`design-patterns-mandatory`)

- Carregar `.agents/skills/design-patterns-mandatory/SKILL.md`.
- Avaliar se Repository/Factory e justificado para `BankDaysRepository`.
- Rodar seletor: `python3 scripts/select_pattern.py --input selector-input.json`.
- Preencher bundle e validar: `python3 scripts/validate_pattern_bundle.py pattern-decisions/bank-days-repository`.

### Fase 3 — PostgreSQL (`postgresql-production-standards`)

- Carregar `.agents/skills/postgresql-production-standards/SKILL.md`.
- Criar migration `00000X_create_bank_days_table.up.sql` com:
  - `id UUID PRIMARY KEY`
  - `bank_code TEXT NOT NULL UNIQUE`
  - `days_before_due INT NOT NULL CHECK (days_before_due >= 0)`
  - `created_at/updated_at TIMESTAMPTZ`
- Revisar com dupla evidencia (fato no projeto + regra oficial PostgreSQL).
- Validar migration reversivel.

### Fase 4 — Implementacao Go (`go-implementation`)

- Carregar `.agents/skills/go-implementation/SKILL.md`.
- Criar:
  - `internal/card/domain/valueobjects/bank_days.go`
  - `internal/card/domain/entities/bank_days.go`
  - `internal/card/application/interfaces/bank_days_repository.go`
  - `internal/card/infrastructure/repositories/postgres/bank_days_repository.go`
  - `internal/card/application/usecases/register_bank_days.go`
  - `internal/card/infrastructure/http/server/handlers/bank_days.go`
- Seguir R0-R7.
- Nao injetar repository diretamente no handler.

### Fase 5 — Docker/Deployment (se aplicavel, `docker-postgres-production-stack`)

- Se a feature exigir seed inicial ou alteracao no compose, aplicar a skill.
- Como o compose existente usa Caddy, documentar que a skill so e usada se criar novo stack no padrao Traefik.

### Fase 6 — Validacao

- `go build ./...`
- `go vet ./...`
- `go test -race -count=1 ./internal/card/...`
- `golangci-lint run ./internal/card/...`
- `task ci:init-prohibited`
- `task ci:zero-comments`
- `task ci:platform-gates`
- `task ci:agent-boundary`

### Fase 7 — Merge e Metricas

- Abrir PR.
- Coletar:
  - Tempo de ciclo.
  - Numero de comentarios de revisao.
  - Se houve rework.
  - Se todos os gates passaram.
- Atualizar `.specs/metrics/effectiveness-baseline.md`.

## Critérios de sucesso

- [ ] Todas as 4 skills foram carregadas e seus bundles validados.
- [ ] CI verde.
- [ ] Nenhum comentario em codigo Go de producao.
- [ ] Nenhum `init()`.
- [ ] Migration com rollback.
- [ ] PR mergeado.

## Proximo passo

Aprovar este plano e iniciar a Fase 1.
