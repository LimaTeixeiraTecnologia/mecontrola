# Prompt — Execução de todas as tarefas de E1 via `execute-all-tasks`

**Data:** 2026-06-05
**Épico:** E1 — `identity-foundation`
**Slug do PRD:** `identity-foundation`
**Skill a invocar:** `execute-all-tasks` (governance; spawna subagent fresh por tarefa, halt-first, retomada idempotente).
**Pré-requisitos validados nesta sessão:**
- `ai-spec sync-spec-hash .specs/prd-identity-foundation/tasks.md` → OK.
- `ai-spec check-spec-drift .specs/prd-identity-foundation` → "OK: sem drift detectado".
- 20 RF-IDs cobertos (RF-01..RF-18 + RF-08-bis + RF-08-ter), 10 tasks declaradas.
- Sincronia `tasks.md.Skills` ↔ `task-*.md.## Skills Necessárias` ok (todos vazios; só auto-carregadas).

---

## Invocação canônica (copiar e colar)

```text
/execute-all-tasks identity-foundation
```

Equivalente explícito (qualquer ferramenta de IA com a skill instalada):

```text
Skill: execute-all-tasks
Args: identity-foundation
```

> **Não passar argumentos extras.** A skill resolve `.specs/prd-identity-foundation/` automaticamente, lê `tasks.md`, monta o DAG, spawna subagent fresh por tarefa e respeita `Paralelizável`.

---

## Restrições inegociáveis (cole junto da invocação se a sessão não tiver contexto)

> Este bloco serve como **prompt de governança** quando a skill não puder consultar esta conversa. O orquestrador deve enforçar cada item **por tarefa**, sem permitir desvio, sem renegociar.

### Fontes de verdade (carregar **sob demanda**, nunca tudo de uma vez)

1. **Runbook canônico** — `docs/runbooks/handler-usecase-uow-repository.md` (shape de código por camada). Consultar antes de qualquer edição em `internal/identity/**` ou `internal/platform/outbox/**`.
2. **Techspec** — `.specs/prd-identity-foundation/techspec.md` (§Application, §Infrastructure, §10, §13, §14, §17). Consumir só a seção pertinente à tarefa corrente.
3. **PRD** — `.specs/prd-identity-foundation/prd.md` (requisitos RF-nn + critérios de aceite CA-01..CA-05). Releitura por RF apenas se a tarefa atual não tiver `Cobertura de Requisitos` explícita em sua row de `tasks.md`.
4. **ADRs 001–008** — leitura cirúrgica (somente a ADR citada no task file).
5. **Skill `go-implementation`** — auto-carregada por detecção de diff (`category: language`). Etapas 1–5 obrigatórias antes de codar.

**Economia de contexto (mandatória):**
- Por tarefa, **máximo 4 referências** carregadas simultaneamente. Se mais forem necessárias, priorizar 3 críticas e registrar as demais como "contexto não carregado" no relatório do subagent.
- **Nunca carregar** `patterns-structural.md` para Factory Function / Functional Options / Adapter / Decorator / Facade — já inline no `SKILL.md` do `go-implementation`.
- **Nunca carregar** ADR que não esteja citada no task file corrente.
- **Nunca carregar** o runbook inteiro — usar `grep`/`Read` com offset/limit para a seção alvo.

### Regras Go que **toda** tarefa deve preservar (R0–R7 + extensões da techspec)

- **R0:** sem `init()`.
- **R1:** funções de domínio/application/infra são métodos de struct. Exceções já documentadas: `IsEntitled` (função pura), `entities.NewID` (função package-level), construtores.
- **R5.10:** sentinels via `errors.New`; wrap com `fmt.Errorf("<prefixo>: %w", err)`; **tratar erro uma única vez**.
- **R5.12:** sem `panic` em produção.
- **R6:** `context.Context` em toda fronteira de IO; interface no consumidor.
- **R6.4:** sem `var _ Interface = (*Type)(nil)` em produção.
- **R6.7:** **`time.Now().UTC()` inline no call-site**; **proibido** `now := time.Now().UTC()`.
- **R6.8 (ADR-008):** **proibido injetar `IDGenerator`** em qualquer camada. Domínio gera via `entities.NewID()` (chama `uuid.NewString()` direto em `internal/identity/domain/entities/id.go`).
- **R6.9 (ADR-008):** **proibido reimplementar `UnitOfWork`** localmente. Consumir `github.com/JailtonJunior94/devkit-go/pkg/database/uow` (`uow.UnitOfWork[T]`, `New[T]`, `NewVoid`).
- **R6.10:** **proibido reimplementar** helpers de resposta HTTP. Consumir `github.com/JailtonJunior94/devkit-go/pkg/responses` (`JSON`, `Error`, `ErrorWithDetails`); códigos semânticos em `details` (`map[string]string{"code": "..."}`).
- **R6.11:** **proibido reimplementar** helpers de SQL NULL. Consumir `internal/platform/sqlnull` (`Str`, `Time`). Para colunas inteiras/booleanas anuláveis usar `*int64`/`*bool` ou `sql.Null*` explicitamente.
- **R7.1:** `any`, não `interface{}`.
- **R7.2:** logging estruturado via `o11y.Logger()`.
- **R7.6:** `errors.Join` para agregar (uso pontual; UoW da devkit já cobre rollback transacional).

### Gates por tarefa (executar **dentro** do subagent antes de marcar `done`)

```bash
# Gate 1: build/vet
go build ./...
go vet ./...

# Gate 2: testes do escopo
go test -race -count=1 ./internal/identity/... ./internal/platform/sqlnull/... ./internal/platform/outbox/...

# Gate 3: integration (build tag) — só para tarefas 6.0 e 10.0
go test -race -count=1 -tags=integration ./internal/identity/infrastructure/repositories/postgres/... ./internal/platform/outbox/...

# Gate 4: aderência ao runbook (deve retornar 0 matches)
grep -RInE "internal/platform/uow|func writeError|func writeJSON|func nullableString|func nullableEmail|entities\.IDGenerator|idGen[^a-zA-Z_]|PrepareContext|database\.FromContext" internal/identity/ internal/platform/outbox/

# Gate 5: ausência ativa de RBAC/JWT/sessões (CA-03 do PRD)
grep -RInE "JWT|RBAC|\brole\b|is_admin|session" internal/identity/

# Gate 6: lint (somente após task 9.0 estar done)
golangci-lint run ./internal/identity/... ./internal/platform/outbox/...
```

**Qualquer gate vermelho ⇒ tarefa volta para `in_progress`; orquestrador NÃO marca `done`.** Halt-first: a próxima tarefa não inicia até a corrente fechar verde.

### Comandos proibidos / ações vetadas

- **Não** rodar `create-prd`, `create-technical-specification`, `create-tasks` novamente (artefatos já aprovados; modificá-los quebra `ai-spec check-spec-drift`).
- **Não** editar `.specs/prd-identity-foundation/{prd.md,techspec.md,adr-*.md,tasks.md}`. Drift detectado ⇒ abortar, reportar ao usuário.
- **Não** introduzir dependência nova em `go.mod` sem decisão registrada. Stack permitida: devkit-go v0.4.0, `github.com/google/uuid v1.6.0`, `github.com/jackc/{pgerrcode,pgx/v5}`, `github.com/stretchr/testify`, `github.com/testcontainers/testcontainers-go`. Mockery v2.30+ para genéricos.
- **Não** rodar `git push --force`, `git reset --hard`, `git rebase -i`, ou deletar branches.
- **Não** rodar `git commit --no-verify` nem desabilitar hooks.
- **Não** alterar `internal/platform/sqlnull/` (já criado e validado).
- **Não** alterar `internal/platform/id/` (preservado por compat histórica; não consumir em código novo).
- **Não** abrir PR antes de **todas** as 10 tarefas estarem `done` e CA-01..CA-05 do PRD validados.

### Paralelismo permitido (apenas onde a tool suporta nativamente)

Conforme `tasks.md`:
- `1.0 ∥ 2.0 ∥ 10.0` — domínios disjuntos (SQL files vs Go domain vs outbox).
- `8.0 ∥ 9.0` — `module.go`/`cmd/server.go` vs `.golangci.yml`.

Demais cadeias são sequenciais.

**Não** paralelizar entre cadeias dependentes (ex.: 5.0 não pode rodar antes de 4.0 mesmo se a tool tiver capacidade).

### Retomada idempotente

- Se a sessão for interrompida, **re-invocar a mesma skill com o mesmo slug** — `execute-all-tasks` lê `Status` de cada linha em `tasks.md` e pula tudo o que já estiver `done`.
- Tarefas `failed`/`blocked`/`in_progress` são reentregues ao próximo subagent fresh.
- **Não** editar `Status` manualmente para "destravar" — abrir issue no usuário se houver bloqueio real.

---

## Critérios de aceitação finais (orquestrador valida antes de reportar `done`)

1. Todas as 10 linhas em `tasks.md` com `Status: done`.
2. `go build ./...` verde.
3. `go vet ./...` verde.
4. `go test -race -count=1 ./...` verde.
5. `go test -race -count=1 -tags=integration ./internal/identity/infrastructure/repositories/postgres/... ./internal/platform/outbox/...` verde.
6. `golangci-lint run ./...` verde.
7. Gate 4 e Gate 5 com 0 matches.
8. `ai-spec check-spec-drift .specs/prd-identity-foundation` retorna "OK: sem drift detectado".
9. **CA-01 (PRD):** cobertura ≥100% em `NewWhatsAppNumber`, `NewEmail`, `IsEntitled` validada por `go test -cover ./internal/identity/...`.
10. **CA-02 (PRD):** zero violações `depguard` em `internal/identity/`.
11. **CA-03 (PRD):** `grep -RInE "JWT|RBAC|\brole\b|is_admin" internal/identity/**/*.go` retorna 0 matches.
12. **CA-04 (PRD):** smoke E2E com Postgres real cobrindo (a)–(h) executado e verde (testcontainers).
13. **CA-05 (PRD):** `ai-spec check-spec-drift .specs/prd-identity-foundation/tasks.md` verde.

---

## Reporte esperado do orquestrador (formato YAML compacto)

```yaml
run_id: e1-identity-foundation-2026-06-05
slug: identity-foundation
total_tasks: 10
done: <n>
failed: []
blocked: []
needs_input: []
skipped_done_on_resume: <n>
elapsed_seconds: <n>
acceptance_criteria:
  ca_01: ok|fail
  ca_02: ok|fail
  ca_03: ok|fail
  ca_04: ok|fail
  ca_05: ok|fail
gates:
  go_build: ok|fail
  go_vet: ok|fail
  go_test_unit: ok|fail
  go_test_integration: ok|fail
  golangci_lint: ok|fail
  gate_4_runbook_adherence: ok|fail
  gate_5_no_rbac_jwt: ok|fail
artifacts:
  branch: <nome do branch criado, se houver>
  commits: <n>
notes: <texto livre opcional para handoff humano>
```

**Esse bloco YAML é o único output que o orquestrador deve emitir ao final** — sem narração extra, sem markdown extenso. Logs detalhados ficam no transcript individual de cada subagent.

---

## Operação manual de fallback (se a skill não puder ser invocada)

Caso a sessão atual não tenha `execute-all-tasks` registrada, executar as tarefas em sequência com `execute-task`, respeitando o DAG:

```text
/execute-task identity-foundation 1.0
/execute-task identity-foundation 2.0      # em paralelo com 1.0 se a tool suportar
/execute-task identity-foundation 10.0     # em paralelo com 1.0/2.0 se a tool suportar
/execute-task identity-foundation 3.0
/execute-task identity-foundation 4.0
/execute-task identity-foundation 5.0
/execute-task identity-foundation 6.0
/execute-task identity-foundation 7.0
/execute-task identity-foundation 8.0
/execute-task identity-foundation 9.0      # em paralelo com 8.0 se a tool suportar
```

Cada `execute-task` carrega o task file correspondente, aplica os gates locais e produz YAML compacto. **Mesmas restrições inegociáveis acima** valem por tarefa.

---

## Fora de escopo desta execução

- **Não** publicar release, gerar changelog, abrir PR ou tag — esses passos ficam em runs separadas (`github-release-publication-flow`, `pull-request`).
- **Não** atualizar `docs/epics/epic-01-identity-foundation.md` aqui — encerramento documental do épico vira run pós-execução.
- **Não** implementar E2 (`billing-pipeline`) ou E3 (`onboarding-magic-token`). E1 destrava esses épicos, mas eles têm PRDs próprios.
- **Não** introduzir métricas Prometheus / dashboards Grafana — restrição operacional do PRD (E4).
- **Não** publicar evento `user_created` no outbox a partir do identity — S-04 do PRD confirma ausência de consumidor; o refactor outbox em 10.0 preserva contrato sem alteração funcional.

---

## Checklist humano antes de iniciar

- [ ] Postgres local disponível em `localhost:5432` (para smoke E2E manual) **ou** Docker disponível (testcontainers).
- [ ] `golangci-lint` v1.55+ instalado.
- [ ] `mockery` v2.30+ instalado (suporte a generics).
- [ ] `git status` limpo (sem mudanças não commitadas que conflitem com o escopo).
- [ ] Branch dedicada criada (ex.: `feat/e1-identity-foundation`) — orquestrador pode operar na branch atual, mas isolar reduz risco.
- [ ] Working tree sincronizado com `main` no último merge conhecido.

**Pronto. Invocar `/execute-all-tasks identity-foundation` e aguardar o YAML final do orquestrador.**
