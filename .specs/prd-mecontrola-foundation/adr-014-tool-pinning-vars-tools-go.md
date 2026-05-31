# ADR-014 — Pinning de ferramentas de dev: `taskfiles/vars.yml` + `tools.go`

## Metadados

- **Título:** Versões de ferramentas de desenvolvimento pinadas em duas camadas para reprodutibilidade local/CI
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §D-29, §CS-29](./prd.md), [techspec §Plano de Rollout M5](./techspec.md), [ADR-006 (test stack)](./adr-006-test-stack-testify-mockery.md), [ADR-012 (supply chain)](./adr-012-supply-chain-scan-deps.md)

## Contexto

Foundation usa múltiplas ferramentas externas: `golangci-lint`, `mockery`, `govulncheck`, `trivy`, `cosign`, `gitsign`, `golang-migrate`, `pre-commit`, `task`. Sem pin centralizado, build local de hoje pode diferir do CI (e do build de amanhã) — violação direta de reprodutibilidade production-ready.

Há também deps Go de tooling (`testify`, `mockery` API, `mock`, etc.) que precisam estar em `go.mod` para `go install` reprodutível.

## Decisão

**Duas camadas de pinning**:

### Camada 1: `taskfiles/vars.yml` — binários CLI externos

Arquivo central no nível dos Taskfiles, declarando versões de ferramentas que NÃO são instaláveis via `go install` (ou cuja versão é desejável pinar mesmo se for via `go install`):

```yaml
# taskfiles/vars.yml — referência conceitual; sintaxe Task vars
vars:
  GOLANGCI_LINT_VERSION: v1.64.0
  MOCKERY_VERSION: v2.50.0
  GOVULNCHECK_VERSION: latest      # govulncheck recomenda latest pro database
  TRIVY_VERSION: v0.58.0
  COSIGN_VERSION: v2.4.0
  GITSIGN_VERSION: v0.13.0
  MIGRATE_VERSION: v4.18.1
  PRE_COMMIT_VERSION: v4.0.1
  TASK_VERSION: v3.51.1            # já existe em D-16
```

Cada Taskfile que precisa de um binário CLI faz `task: install-tool VERSION={{.GOLANGCI_LINT_VERSION}}` ou similar.

### Camada 2: `tools.go` — deps Go (convenção Go padrão)

Arquivo `tools.go` na raiz com build tag `//go:build tools`:

```go
//go:build tools
// +build tools

package main

import (
    _ "github.com/stretchr/testify"
    _ "github.com/vektra/mockery/v2"
    // outras deps que aparecem no go.mod via uso indireto via go install
)
```

Mantém deps de teste/codegen presentes no `go.mod` sem afetar build de produção (tag `tools` exclui do build normal). Dependabot atualiza ambos os arquivos. `task tools:install` ou parte de `task setup` instala todas as ferramentas locais a partir das versões pinadas.

### Estratégia de update

- **Dependabot** (ADR-012) cobre `go.mod` (incluindo `tools.go`) e `.github/workflows/*.yml` (que referenciam versões).
- **`taskfiles/vars.yml`** atualizado **manualmente** por PR (Dependabot não cobre Taskfile vars nativamente) — agendar review trimestral; OU usar Renovate config secundária só para vars.yml (futuro).
- **Critério de update**: bump quando release oficial + CI verde + 1 dia de soak em staging.

## Alternativas Consideradas

1. **Tudo em `go.mod` via `tools.go` (sem vars.yml)**.
   - Vantagens: 1 fonte de verdade; Dependabot cobre tudo.
   - Desvantagens: nem todos os CLIs são instaláveis via `go install` (cosign distribui binário oficial; trivy idem; golangci-lint recomenda script de install próprio). Forçar `go install` para esses perde features (cosign sem CGO perde funcionalidades).
2. **Tudo em `taskfiles/vars.yml`**.
   - Vantagens: 1 fonte de verdade.
   - Desvantagens: `testify` (lib Go) não cabe em vars.yml; precisa estar em `go.mod` para `go test` funcionar.
3. **Sem pin centralizado** (`@latest`/sem version).
   - Vantagens: zero overhead.
   - Desvantagens: build não-reprodutível; CI quebra silencioso; viola production-ready.
4. **`Brewfile` (macOS only)**.
   - Vantagens: dev experience macOS familiar.
   - Desvantagens: não funciona em Linux/CI; perde paridade local/CI.

## Consequências

### Benefícios Esperados

- **Reprodutibilidade total**: `task setup` local = workflow CI = mesma versão de ferramenta.
- **Audit trail**: `git log taskfiles/vars.yml` mostra exatamente quando cada ferramenta foi atualizada.
- **Dependabot cobre `go.mod`**: minor/patch de testify auto-merged sem intervenção.
- **Rollback de tooling**: reverter `vars.yml` traz versão anterior.

### Trade-offs e Custos

- 2 arquivos para manter (vs 1).
- vars.yml não tem Dependabot nativo (atualização manual ou Renovate secundário).
- `task setup` mais lento (instala 8 ferramentas) — mitigado por cache de install steps.

### Riscos e Mitigações

- **Risco**: Dev esquece de instalar uma ferramenta nova após `git pull`.
  - **Mitigação**: `task setup` re-executável idempotente; pre-commit hook (`pre-commit` framework) chama `task check-tools` antes de commit.
- **Risco**: `vars.yml` defasado vs `.github/workflows/*.yml` (versão duplicada).
  - **Mitigação**: workflow referencia vars.yml via `task tools:install GOLANGCI_LINT_VERSION={{from vars.yml}}` (cross-OS); ou job dedicado lê vars.yml via `yq` e exporta para env.
- **Risco**: Renovate secundário não configurado e versões ficam stale.
  - **Mitigação**: review trimestral no calendário do tech lead; alerta de versão >6 meses old.

## Plano de Implementação

1. `taskfiles/vars.yml` com versões iniciais pinadas (datas Q2 2026).
2. `tools.go` na raiz com `//go:build tools` + imports de `testify`, `mockery` etc.
3. `taskfiles/setup.yml` (ou `tools.yml`): tarefa `task setup` instala cada ferramenta na versão de vars.yml — uma sub-tarefa por ferramenta (`install-golangci-lint`, `install-mockery`, ...).
4. `.github/workflows/ci.yml`: cada job que usa ferramenta lê versão de vars.yml (via `yq` + env export).
5. README: seção "Setup" explicando `task setup` + lista de ferramentas + versões.
6. `dependabot.yml`: já cobre `gomod` (D-26); confirmar que `tools.go` é incluído.
7. Calendário: revisar vars.yml trimestralmente (Q1, Q2, Q3, Q4); bump por PR.

## Monitoramento e Validação

- `task setup` em CI roda em ~2 min cold, ~30s com cache; alerta se > 5 min.
- Job CI dedicado `check-tools` valida que versões instaladas == versões em vars.yml.
- Mensal: comando `task tools:list` mostra versões instaladas vs pinadas.

## Impacto em Documentação e Operação

- README: "Setup local".
- Onboarding: `task setup` resolve tudo.
- Runbook "Bump de ferramenta": PR editando vars.yml + CI verde + soak staging + merge.
- Dependabot config (ADR-012): explícito que `tools.go` está coberto via `gomod`.

## Revisão Futura

- Revisitar para adotar Renovate secundário se vars.yml ficar stale >2 meses.
- Revisitar se Dependabot ganhar suporte nativo a Taskfile vars (não-trivial).
- Revisitar consolidação em `mise` (asdf successor) se onboarding ficar fricção real.
