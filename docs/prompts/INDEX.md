# Indice de Prompts de Execucao por PRD — 2026-06-12

Cada prompt invoca a skill `execute-all-tasks` sobre 1 PRD inteiro, com mandatorios inegociaveis no cabecalho.
Sequencia numerica = ordem recomendada para go-live.

## Mandatorios comuns (aplicam a TODOS os prompts)

1. Carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1-5, R0-R7, R-ADAPTER-001).
2. **ZERO COMENTARIOS em codigo Go de producao** (R-ADAPTER-001.1). Excecoes apenas: `//go:build`, `//nolint:` com justificativa na mesma linha, `// Code generated`.
3. Aplicar **Domain Modeling Made Functional** (Scott Wlaschin) onde fizer sentido: smart constructors, discriminated union, workflow puro, state-as-type.
4. Padronizar com **`internal/transactions`** (baseline canonico): `Decide*` puros, repository SQL com `user_id` no `WHERE`, smart constructors em VOs, eventos tipados em `entities/events.go`, producers que apenas serializam.
5. Foco: MVP robusto, eficiente, economico, production-ready/proof, sem falso positivo, inegociavel.
6. Validacao final obrigatoria por tarefa: `task lint && task test && task vulncheck && task lint:user-isolation` PASS.

## Prompts (use `execute-all-tasks`)

| # | Pacote | PRD | Severidade | Tarefas | Esforco |
|---|---|---|---|---|---|
| [001](001-2026-06-12-pkgA-execute-gateway-auth-forensics.md) | A | `prd-gateway-auth-forensics` | Bloqueante critico seguranca | 8 (23 RFs) | ~5d |
| [002](002-2026-06-12-pkgB-execute-pre-golive-hardening.md) | B | `prd-pre-golive-hardening` | Bloqueante operacional | 8 (34 RFs) | ~3d paralelo |
| [003](003-2026-06-12-pkgC-execute-outbox-aggregate-user-id.md) | C | `prd-outbox-aggregate-user-id` | Padronizacao pos go-live | 8 (20 RFs) | ~4d paralelo |

## Ordem agregada de execucao

- **Sprint 1 (semana 1)**: prompts 001 + 002 em paralelo (Pacote A e B sao independentes).
- **GO-LIVE**: apos validacao integral de A + B (gates de lint, smoke, microbenchmark, runbooks).
- **Sprint pos go-live**: prompt 003 (Pacote C — padronizacao outbox).

## Como usar

Cada prompt e self-contained: invoque a skill `execute-all-tasks` apontando para o diretorio do PRD descrito no prompt. A skill spawna subagent fresh por tarefa, respeitando o DAG da `tasks.md`, paralelizando onde declarado, com retomada idempotente.

Os mandatorios inegociaveis do prompt devem ser incluidos no prompt do agente orquestrador para que cada subagent fresh os herde.
