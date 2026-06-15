# O que pode ser removido — Auditoria Completa

**Data:** 2026-06-15
**Escopo:** Taskfile, scripts, arquivos/diretórios, dependências Go, código Go morto
**Metodologia:** Cada achado foi confirmado por busca independente antes de ser classificado.
**Política:** zero falso positivo — apenas HIGH e MEDIUM confidence são listados.

---

## 1. Scripts sem referência (órfãos)

### 1.1 — HIGH confidence

| Arquivo | Evidência de orfanato | Busca de confirmação |
|---|---|---|
| `taskfiles/scripts/configure-branch-protection.sh` | Zero ocorrências em `*.yml`, `*.yaml`, `*.sh` fora de si mesmo | `grep -rn "configure-branch-protection"` → apenas self-refs dentro do script |
| `deployment/scripts/vps-ssh-hardening.sh` | Zero ocorrências em qualquer taskfile ou CI workflow | `grep -rn "vps-ssh-hardening"` → apenas self-refs dentro do script |
| `deployment/scripts/caddyfile-smoke.sh` | Zero ocorrências em qualquer taskfile ou CI workflow | `grep -rn "caddyfile-smoke"` → vazio |
| `scripts/smoke/telegram_webhook/main.go` | Nunca chamado em taskfile ou CI; métrica `telegram_webhook_rate_limit_exceeded_total` em `identity/module.go` é nome de métrica, não referência ao script | `grep -rn "telegram_webhook"` em `*.yml/*.sh` → vazio |
| `scripts/load-test/auth-webhook.k6.js` | As tasks `loadtest:card*` que poderiam chamá-lo foram removidas pois referenciavam `./loadtest/card/` (diretório inexistente); nenhuma task atual aponta para este arquivo | `grep -rn "auth-webhook.k6"` em `*.yml` → vazio |

### 1.2 — MEDIUM confidence (revisar antes de remover)

| Arquivo | Situação | Risco de remoção |
|---|---|---|
| `deployment/scripts/pg-dump.sh` | Não está em nenhum taskfile nem CI. Referenciado apenas em `docs/prompts/2026-06-15-auditoria-infraestrutura-production-readiness.md` como "backup auxiliar para VPS self-managed". Pode estar agendado via `cron` no VPS fora do repo. | MÉDIO — verificar se existe crontab no VPS antes de remover |

---

## 2. Código Go morto

### 2.1 — HIGH confidence (zero callers em produção, confirmado)

#### `NewDecideUserEntitlement`
- **Arquivo:** `internal/identity/application/usecases/decide_user_entitlement.go:36`
- **Evidência:** `grep -rn "NewDecideUserEntitlement(" internal/ --include="*.go" | grep -v "_test.go" | grep -v "func New"` → vazio
- **Situação:** Construtor e struct `DecideUserEntitlement` existem com testes, mas nenhum módulo de wiring (`module.go`) ou handler os instancia. Use case sem consumidor.
- **Impacto de remoção:** Baixo. Testes associados também precisam ser removidos.

#### `NewTransactionsAdapter` (variante sem "Full")
- **Arquivo:** `internal/agent/infrastructure/dispatcher/transactions_adapter.go:48`
- **Evidência:** `grep -rn "NewTransactionsAdapter(" internal/ --include="*.go" | grep -v "_test.go" | grep -v "func New"` → vazio
- **Situação:** Apenas `NewTransactionsAdapterFull` (linha 52, aceita 4 dependências) é chamado. A variante simplificada foi provavelmente substituída durante a expansão do adapter.
- **Impacto de remoção:** Baixo. A variante Full cobre todos os casos de uso.

#### `NewMaterializeRecurring`
- **Arquivo:** `internal/transactions/domain/commands/materialize_recurring.go:25`
- **Evidência:** `grep -rn "NewMaterializeRecurring(" internal/ --include="*.go" | grep -v "_test.go" | grep -v "func New"` → vazio
- **Situação:** A função de domínio nunca é chamada. A função usada na camada de aplicação é `NewMaterializeRecurringForDay`. Este construtor parece ser um rascunho anterior à decisão de nomear por "ForDay".
- **Impacto de remoção:** Baixo. Verificar se há testes unitários cobrindo apenas este construtor.

### 2.2 — MEDIUM confidence (value objects sem instanciadores, pode ser reserva intencional)

#### `IntentActionUpdate()`, `IntentActionDelete()`
- **Arquivo:** `internal/agent/domain/valueobjects/intent_action.go:41-42`
- **Situação:** Os construtores `IntentActionList`, `IntentActionGet`, `IntentActionCreate` são usados. `Update` e `Delete` estão definidos mas sem nenhum caller em produção ou em testes.
- **Risco:** Podem ser reserva para features futuras do módulo agent. Remoção prematura pode gerar retrabalho.
- **Recomendação:** Manter se houver PRD de update/delete de entidades via agent. Remover se o roadmap não inclui essas operações.

#### `ModelSlugMistralSmall()`, `ModelSlugClaudeHaiku45()`
- **Arquivo:** `internal/agent/domain/valueobjects/model_slug.go:39-40`
- **Situação:** Apenas `ModelSlugGeminiFlashLite()` e `ModelSlugGPT5Nano()` possuem callers. Mistral e Claude Haiku foram possivelmente testados e descartados.
- **Risco:** Se o time planeja habilitar esses modelos futuramente, manter. Caso contrário são dead code.
- **Recomendação:** Confirmar com o time de produto se esses modelos estão no roadmap.

---

## 3. Dependências Go

**Resultado: nenhuma dependência removível com HIGH ou MEDIUM confidence.**

Auditoria de todos os 14 direct deps em `go.mod`:

| Dependência | Usada em produção | Status |
|---|---|---|
| `github.com/JailtonJunior94/devkit-go` | Sim — cmd/, infrastructure/ | Manter |
| `github.com/getkin/kin-openapi` | Sim — contract tests | Manter |
| `github.com/go-chi/chi/v5` | Sim — todos os handlers HTTP | Manter |
| `github.com/google/uuid` | Sim — DTOs, use cases, repos | Manter |
| `github.com/jackc/pgerrcode` | Sim — 8+ repositórios Postgres | Manter |
| `github.com/jackc/pgx/v5` | Sim — driver Postgres principal | Manter |
| `github.com/robfig/cron/v3` | Sim — job scheduler | Manter |
| `github.com/spf13/cobra` | Sim — CLI (server/migrate/worker) | Manter |
| `github.com/spf13/viper` | Sim — config.go | Manter |
| `github.com/stretchr/testify` | Sim — 617+ usos em testes (via tools.go) | Manter |
| `github.com/testcontainers/testcontainers-go` | Sim — integration tests | Manter |
| `github.com/vektra/mockery/v2` | Sim — geração de mocks (tools.go) | Manter |
| `golang.org/x/text` | Sim — collator pt-BR em categories | Manter |
| `golang.org/x/time` | Sim — rate limiting em middleware | Manter |

---

## 4. Arquivos/diretórios não rastreados pelo git

O `git status` mostra `docs/` inteiro como não rastreado (`??`). Dentro:

| Caminho | Natureza | Decisão |
|---|---|---|
| `docs/postman/` | Coleção Postman + environment — documentação de API | Manter se usado pelo time de QA/API; não está em automação |
| `docs/diagrams/` | Diagramas PlantUML por módulo — design reference | Manter se mantido atualizado; não está em automação |
| `docs/runbooks/*.md` | Gerados em 2026-06-15 (hoje) | Avaliar se é saída de sessão Claude Code — podem ser arquivados ou commitados |
| `docs/prompts/*.md` | Incluindo este arquivo — auditorias e prompts gerados em sessão | Commitar os que tiverem valor operacional; descartar os que forem rascunhos |

**Nota:** Nenhum arquivo em `docs/` está em CI ou automação. A decisão de commitar ou remover é de curadoria, não técnica.

---

## 5. Resumo executivo

### Prioridade 1 — Seguro remover sem revisão adicional (HIGH)

```
taskfiles/scripts/configure-branch-protection.sh
deployment/scripts/vps-ssh-hardening.sh
deployment/scripts/caddyfile-smoke.sh
scripts/smoke/telegram_webhook/main.go (+ diretório se ficar vazio)
scripts/load-test/auth-webhook.k6.js (+ diretório se ficar vazio)

internal/identity/application/usecases/decide_user_entitlement.go  ← use case + test
internal/agent/infrastructure/dispatcher/transactions_adapter.go:48-51  ← só a func NewTransactionsAdapter
internal/transactions/domain/commands/materialize_recurring.go:25-35  ← só a func NewMaterializeRecurring
```

### Prioridade 2 — Revisar com o time antes de remover (MEDIUM)

```
deployment/scripts/pg-dump.sh              ← checar crontab no VPS primeiro
internal/agent/domain/valueobjects/intent_action.go:41-42    ← confirmar roadmap
internal/agent/domain/valueobjects/model_slug.go:39-40       ← confirmar roadmap
```

### Não remover (confirmado)

```
Todas as 14 dependências diretas em go.mod
Todos os 42 tasks do Taskfile.yml (pós-limpeza desta sessão)
docs/postman/, docs/diagrams/   ← decisão de curadoria, não técnica
deployment/scripts/pg-dump.sh  ← até confirmar crontab no VPS
```

---

## 6. Como executar a limpeza (quando aprovado)

```bash
# Prioridade 1 — scripts órfãos
rm taskfiles/scripts/configure-branch-protection.sh
rm deployment/scripts/vps-ssh-hardening.sh
rm deployment/scripts/caddyfile-smoke.sh
rm -r scripts/smoke/telegram_webhook/
rm scripts/load-test/auth-webhook.k6.js
# Se scripts/load-test/ ficar vazio: rmdir scripts/load-test/

# Prioridade 1 — código Go morto
# Remover arquivo completo se use case não tiver outros símbolos usados:
rm internal/identity/application/usecases/decide_user_entitlement.go
# Remover teste correspondente:
rm internal/identity/application/usecases/decide_user_entitlement_test.go  # se existir

# Remover apenas a função (edição cirúrgica):
# internal/agent/infrastructure/dispatcher/transactions_adapter.go  → remover linhas 48-51
# internal/transactions/domain/commands/materialize_recurring.go    → remover linhas 25-35

# Validação pós-remoção
task card:audit
task lint:run
task test:unit
go build ./...
```
