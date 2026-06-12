# Run — Execução completa de `internal/transactions` via `execute-all-tasks`

- **Data**: 2026-06-12
- **PRD alvo**: `.specs/prd-transactions-monthly/`
- **Orquestrador**: skill `execute-all-tasks` (spawna subagent fresh por tarefa para isolar contexto, respeita DAG, paraleliza nativamente, halt-first, retomada idempotente)
- **Modo**: production-ready/proof inegociável, zero falso positivo
- **Ambiente esperado**: branch limpo do `main`, Postgres + Docker disponíveis, `mockery`, `golangci-lint`, `task` instalados

---

## Como executar no Claude Code

Cole o bloco abaixo como mensagem única no Claude Code (terminal ou IDE) e dê enter:

```
/execute-all-tasks .specs/prd-transactions-monthly
```

Quando o agente pedir confirmação de execução, **antes de aprovar**, leia este arquivo inteiro e cole o **Prompt de Reforço** logo abaixo como reply.

---

## Prompt de Reforço (cole após `/execute-all-tasks` iniciar)

> Use este prompt como contexto durável para a sessão de execução. Ele complementa — não substitui — `AGENTS.md`, `.claude/rules/governance.md`, `.claude/rules/go-adapters.md` e as 6 ADRs em `.specs/prd-transactions-monthly/`. Em conflito, prevalece a regra mais restritiva.

```
Você é o orquestrador de execução do PRD `.specs/prd-transactions-monthly/` (10 tarefas, 47 RFs).
Vai rodar via `execute-all-tasks`, com subagent fresh por tarefa. Sua diretriz é production-ready
inegociável e zero falso positivo. Você NÃO pode marcar tarefa como `done` sem evidência objetiva
de DoD validado.

# Regras inegociáveis de execução

## 1. Carga obrigatória antes de qualquer edição
- Ler `AGENTS.md` integralmente.
- Carregar `.claude/skills/agent-governance/SKILL.md`.
- Carregar `.claude/skills/go-implementation/SKILL.md` e executar Etapas 1–5 sem pular nenhuma.
- Carregar `.claude/rules/governance.md`, `.claude/rules/go-adapters.md`.
- Ler o `prd.md` e `techspec.md` da pasta `.specs/prd-transactions-monthly/`.
- Ler as 6 ADRs (`adr-001` a `adr-006`).
- Ler o task file alvo (`task-X.0-*.md`) inteiro antes de começar.
- Em tarefas que tocam adapters, carregar a Matriz R-ADAPTER-001.3 e selecionar as referências exatas (max 4 simultâneas).

## 2. Skills auto-carregadas (NÃO declarar na coluna Skills do tasks.md, mas USAR)
- `agent-governance` (category=governance)
- `go-implementation` (category=language)
- A tarefa 10.0 também carrega `otel-grafana-dashboards` (já declarada).

## 3. Gates HARD que bloqueiam `done` automaticamente
Antes de tocar `Status` para `done` em qualquer tarefa, validar TODOS os gates abaixo. Falha em
qualquer um → `failed` ou `needs_input`, NUNCA `done`:

### Gate R0–R7 (go-implementation)
- [ ] R0: nenhum `init()` introduzido.
- [ ] R1: toda função é método de struct, exceto `main`, factories `New*` e helpers de teste.
- [ ] R2: nenhum alias de campo sem transformação.
- [ ] R3: mocks via `mockery.yml`; rodar `task mocks` antes de testar.
- [ ] R4: testes em `testify/suite` table-driven.
- [ ] R5.8: enums com `iota+1`.
- [ ] R5.10: `errors.New` estático, `fmt.Errorf("ctx: %w", err)` para wrap; tratar erro 1x.
- [ ] R5.12: zero `panic` em produção.
- [ ] R6.3: interface declarada no consumidor.
- [ ] R6.4: nenhum `var _ Interface = (*Type)(nil)`.
- [ ] R6.7: nenhum `clock.Clock`; `time.Now().UTC()` inline ou instante via parâmetro.
- [ ] R7.1: `any` em vez de `interface{}`.
- [ ] R7.2: `log/slog` via `observability.Logger`.
- [ ] R7.6: `errors.Join` para agregar.

### Gate R-ADAPTER-001 (governance.md + go-adapters.md)
- [ ] R-ADAPTER-001.1 — **zero comentários** em `.go` de produção (exceções: `// Code generated`,
      `//go:`, `//nolint:` com justificativa). Rodar:
      ```bash
      grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
        "^[[:space:]]*//" internal/transactions/ \
        | grep -Ev "(//go:|//nolint:|// Code generated)" \
        && echo "FAIL" && exit 1 || true
      ```
- [ ] R-ADAPTER-001.2 — handlers, consumers, jobs, producers são adapters finos
      (`adapter → usecase → service/repo/client`). **Sem SQL direto** em adapter:
      ```bash
      grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
        "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
        internal/transactions/infrastructure/http/server/handlers/ \
        internal/transactions/infrastructure/messaging/database/consumers/ \
        internal/transactions/infrastructure/messaging/database/producers/ \
        internal/transactions/infrastructure/jobs/handlers/ \
        && echo "FAIL" && exit 1 || true
      ```

### Gate Repositório (audit fix do alert_repository.go:20-27)
- [ ] `db database.DBTX` é **campo da struct**, injetado em `NewXxxRepository(o11y, db)`.
- [ ] Nenhum método de repositório recebe `db` como parâmetro. Verificar:
      ```bash
      grep -rEn "func \([a-z]+ \*[a-z]+Repository\) [A-Z][a-zA-Z]+\(ctx context.Context, db database\.DBTX" \
        internal/transactions/infrastructure/repositories/ \
        && echo "FAIL: repo method com db parameter" && exit 1 || true
      ```

### Gate DMMF (ADR-006 + .claude/rules/transactions-workflows.md a criar na Task 10.0)
- [ ] Smart constructors em VOs e `domain/commands/`; validação NUNCA fora deles.
- [ ] `Decide*` puro nos 5 workflows obrigatórios: CreateTransaction, UpdateTransaction,
      CreateCardPurchase, UpdateCardPurchase, MaterializeRecurringForDay.
- [ ] `RefMonthsAffected` e `InvoiceDeltas` calculados SOMENTE no `Decide*`.
- [ ] Producers só mapeiam `entities.<Event>` → `outbox.Envelope`; sem branching de domínio.
- [ ] Proibido: `Result[T,E]`, function-as-DI, `Decide*` em CRUD trivial.

### Gate Audit Fixes da v1.5
- [ ] **#1**: `commands.New*` em `domain/commands/` (não em `application/`).
- [ ] **#2**: `CardBillingSnapshot` é o único nome em uso (zero ocorrências de `CardSnapshot`).
- [ ] **#3**: `BillingCycleResolver` aplica `min(day, last_day_of_target_month)`; teste com
      `due_day=30` em fevereiro retorna dia 28/29.
- [ ] **#4**: `card_invoice_repository.ApplyDelta(invoiceID, delta, expectedVersion)` com
      optimistic locking; integration test cobre delta negativo.
- [ ] **#5**: `mockery.yml` lista 12 mocks nominais; `task mocks` gera sem erro.
- [ ] **#6**: idempotência do `RecurringMaterializerJob` descrita como **double-layer**
      (advisory lock + PK), não "triple".

### Gate Cobertura de Requisitos
- [ ] Cada RF da coluna "Cobertura de Requisitos" do task file tem comportamento implementado
      e teste cobrindo o caso happy + 1 caso de borda.
- [ ] `ai-spec check-spec-drift .specs/prd-transactions-monthly` continua `OK` após cada tarefa.

### Gate Validação Proporcional (go-implementation Etapa 5)
Antes de marcar `done`, executar e anexar saída resumida na evidência:
- [ ] `gofmt -l internal/transactions/... | grep . && exit 1 || true`
- [ ] `go vet ./internal/transactions/...`
- [ ] `go build ./...`
- [ ] `go test -race -count=1 ./internal/transactions/...`
- [ ] Integration tests com tag: `go test -race -count=1 -tags=integration ./internal/transactions/...`
- [ ] `golangci-lint run ./internal/transactions/...` (escopo da tarefa)

## 4. Política de halt-first
- Falha em qualquer gate hard → marca a tarefa como `failed`, escreve diagnóstico no campo
  Status do tasks.md e PARA. Não tentar 'esticar' a tarefa, não criar workaround.
- Falha em teste → investigar causa raiz; corrigir ou marcar `needs_input`. Nunca pular ou
  marcar como `xfail`.
- Suspeita de falso positivo (teste passa mas comportamento errado) → degradar para `needs_input`
  com a hipótese explicada.

## 5. Política de paralelização
- Respeitar coluna `Paralelizável` do tasks.md sem interpretar.
- Pares aprovados: 5.0↔6.0 (módulos disjuntos), 7.0↔8.0 (adapters distintos).
- Para cada par paralelo, abrir 2 subagents fresh simultâneos; aguardar barrier antes da
  próxima fase. Não fundir output dos dois antes de ambos `done`.

## 6. Política de evidência (governance.md "Política de Evidência")
Cada tarefa concluída produz nas notas internas:
- Lista de arquivos criados/modificados (caminhos absolutos).
- Saída resumida (PASS/FAIL + contadores) dos 6 comandos de validação.
- Hash SHA-256 do task file consumido (para detectar drift de re-execução).
- Confirmação explícita dos gates HARD aplicáveis à tarefa.
- Riscos residuais (1–3 bullets) e suposições assumidas.

## 7. Skills processuais por tarefa (auto-detectadas em runtime)
- Tasks 1.0..9.0: apenas auto-carregadas.
- Task 10.0: `otel-grafana-dashboards` declarada — usar.

## 8. Anti-falso-positivo checklist final por tarefa
Antes de assinar `done`, responder honestamente cada pergunta:
1. Algum teste foi pulado, `t.Skip`, `_test.go` com build tag desligada, ou comentado? Se sim → `failed`.
2. Algum gate HARD acima foi marcado [ ] sem evidência de comando executado? Se sim → `failed`.
3. Existem TODOs no código deixados como dívida? Se sim → `needs_input` com lista.
4. O subagent inferiu que algo "provavelmente funciona" sem rodar? Se sim → `failed`.
5. Comentário em `.go` foi adicionado fora das 3 exceções? Se sim → remover e retestar.
6. `mockery` foi pulado e mock manual foi criado? Se sim → corrigir via `task mocks`.

## 9. Ordem real de execução (DAG)
```
Fase 1: [1.0]
Fase 2: [2.0] (depende de 1.0)
Fase 3: [3.0, 4.0] paralelo (ambos dependem de 2.0)
Fase 4: [5.0, 6.0] paralelo (Com 6.0/Com 5.0; ambos dependem de 3.0+4.0)
Fase 5: [7.0, 8.0] paralelo (Com 7.0/Com 8.0; ambos dependem de 5.0+6.0)
Fase 6: [9.0] (depende de 5.0,6.0,7.0,8.0)
Fase 7: [10.0] (depende de 5.0..9.0)
```

## 10. Recuperação / Retomada
- Re-rodar `/execute-all-tasks .specs/prd-transactions-monthly` é idempotente: tarefas `done`
  pulam automaticamente; tarefas `failed`/`needs_input` retomam.
- NUNCA editar manualmente o tasks.md para "destravar" uma tarefa sem resolver causa raiz.
- Em re-execução, re-rodar `ai-spec check-spec-drift .specs/prd-transactions-monthly` antes de
  começar — se hash divergir, parar e reportar drift do PRD/techspec.

## 11. Comandos prontos para validação final do PRD inteiro
Após todas as 10 tarefas em `done`, rodar e capturar evidência:

```bash
# 1. Cobertura completa de RFs
ai-spec check-spec-drift .specs/prd-transactions-monthly

# 2. Migrations
task migrate
task migrate-down
task migrate

# 3. Mocks
task mocks

# 4. Testes
go test -race -count=1 ./internal/transactions/...
go test -race -count=1 -tags=integration ./internal/transactions/...
go test -race -count=1 ./internal/card/...

# 5. Lint + vet + fmt
gofmt -l internal/transactions/ internal/card/application/usecases/get_card_for_user.go
go vet ./internal/transactions/... ./internal/card/...
golangci-lint run ./internal/transactions/... ./internal/card/application/usecases/...

# 6. Gates R-ADAPTER-001
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/transactions/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentarios proibidos" && exit 1 || echo "OK: zero comentarios"

grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/transactions/infrastructure/http/server/handlers/ \
  internal/transactions/infrastructure/messaging/database/consumers/ \
  internal/transactions/infrastructure/messaging/database/producers/ \
  internal/transactions/infrastructure/jobs/handlers/ \
  && echo "FAIL: SQL em adapter" && exit 1 || echo "OK: adapters finos"

# 7. Gate Repositório (db como campo)
grep -rEn "func \([a-z]+ \*[a-z]+Repository\) [A-Z][a-zA-Z]+\(ctx context.Context, db database\.DBTX" \
  internal/transactions/infrastructure/repositories/ \
  && echo "FAIL: db em método de repo" && exit 1 || echo "OK: db é campo"

# 8. Build final dos binários
go build ./cmd/api ./cmd/worker

# 9. Promtool (alertas Grafana)
promtool check rules docs/alerts/transactions.yaml

# 10. Validar dashboard JSON
jq . docs/dashboards/transactions-overview.json > /dev/null && echo "OK: dashboard JSON valido"
```

Anexar a saída desses 10 blocos como evidência final em um comentário do PR ou em
`docs/runs/2026-06-12-execute-all-tasks-transactions-monthly-evidence.md`.

## 12. Critérios de Aceite Globais (Definition of Done do PRD)

Marca o PRD inteiro como **"production-ready/proof"** SOMENTE quando TUDO abaixo for verdade:

- [ ] 10 tarefas em `done` no `tasks.md` (sem `failed`, sem `needs_input`, sem `pending`).
- [ ] `ai-spec check-spec-drift` `OK` em `.specs/prd-transactions-monthly`.
- [ ] Spec-hash do PRD e da techspec batem no cabeçalho do `tasks.md` (sem drift).
- [ ] Todos os 47 RFs (RF-01..RF-47) cobertos por pelo menos uma tarefa concluída com teste.
- [ ] Suite unit + integration verde em ambiente local.
- [ ] Lint, vet, fmt limpos.
- [ ] Gates R-ADAPTER-001.1 e R-ADAPTER-001.2 verificados via `grep` e retornaram vazios.
- [ ] Gate "db como campo de repositório" verificado e retornou vazio.
- [ ] 6 ADRs implementadas conforme planos (validar pontos críticos: clamp #3, ApplyDelta #4,
      double-layer #6, snapshot estático ADR-001, debounce 1500ms ADR-004, single event ADR-003,
      cascade silenciosa ADR-005, DMMF seletivo ADR-006).
- [ ] Feature flag `TransactionsConfig.Enabled` testada em smoke E2E em ambos os modos (true/false).
- [ ] Dashboard Grafana importa sem erro; 4 alertas validam via `promtool check rules`.
- [ ] Runbook `docs/runbooks/transactions.md` cobre 3 cenários (consumer travado, drift,
      dead-letter).
- [ ] Regra `.claude/rules/transactions-workflows.md` criada e referenciada no
      `.claude/rules/governance.md`.
- [ ] `cmd/api` e `cmd/worker` compilam sem erro com o módulo wireado.
- [ ] Zero comentários em `.go` de produção (gate vazio).

## 13. Política de NÃO-execução / pausa
Pare e peça intervenção humana se:
- Suspeitar de bug no `internal/card` que afete `GetCardForUser` (escopo cross-module).
- Detectar conflito entre regra inegociável e estado atual do repo que não estava previsto.
- O `mockery` ou `golangci-lint` exigir upgrade de versão.
- Encontrar tabela `mecontrola.transactions*` pré-existente (não deveria; AS-11 assume base vazia).
- A migração `000014` colidir com numeração existente (verificar `ls migrations/` na execução).

## 14. Formato final de relatório
Ao concluir o último `done`, escrever em `docs/runs/2026-06-12-execute-all-tasks-transactions-monthly-evidence.md` no formato:

```markdown
# Evidência de Execução — 2026-06-12

## Estado final
- Total de tarefas: 10
- Done: <n>
- Failed: <n>
- Needs input: <n>

## Comandos de validação global (saída resumida)
<colar saída dos 10 comandos da seção 11>

## Riscos residuais
- ...

## Suposições
- ...

## Evidência da Definition of Done
<colar tabela da seção 12 marcada>
```

— Fim do prompt de reforço.
```

---

## Pré-requisitos antes de rodar

1. Working tree limpo (`git status` sem changes não-commitadas).
2. Branch dedicada: `git checkout -b feat/transactions-monthly-mvp`.
3. Docker + Postgres rodando localmente (testcontainers exige).
4. Variáveis de ambiente do `configs/config.go` setadas (`DB_*`, `OUTBOX_*`, `OTEL_*`).
5. `task install-tools` ou equivalente para garantir `mockery`, `golangci-lint`, `migrate`.
6. Verificar que `ai-spec` está instalado: `which ai-spec`.

## Estimativa de duração

- Fase 1 (Task 1.0): ~30min
- Fase 2 (Task 2.0): ~45min
- Fase 3 paralela (3.0+4.0): ~1h
- Fase 4 paralela (5.0+6.0): ~2h
- Fase 5 paralela (7.0+8.0): ~2h
- Fase 6 (9.0): ~1h
- Fase 7 (10.0): ~45min

**Total esperado**: ~8h de execução assistida com gates rodando entre fases.

## Sinal de execução bem-sucedida

O agente deve, ao final, escrever em `docs/runs/2026-06-12-execute-all-tasks-transactions-monthly-evidence.md` a tabela DoD da seção 12 com **TODOS os itens marcados [x]**. Qualquer item desmarcado bloqueia o PR.

## Pós-execução

1. Abrir PR para `main` referenciando este run e a evidência.
2. Rodar `/code-review ultra` no PR para review multi-agent.
3. Após merge, executar `/schedule` para revisão pós-implantação em 7 dias (ADR-006 revisão futura).
