# Relatorio de Bugfix

- Total de bugs no escopo: 7
- Corrigidos: 7
- Testes de regressao adicionados: 4 grupos de testes/validacoes
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-001
- Severidade: major
- Origem: finding de review, RF-04, CS-20
- Estado: fixed
- Causa raiz: `LoadConfig` tratava `ConfigFileNotFoundError` como toleravel em todos os ambientes, deixando o default `ENVIRONMENT=local` subir sem `.env`.
- Arquivos alterados: `configs/config.go`, `configs/config_test.go`, `cmd/cmd_integration_test.go`
- Teste de regressao: `TestLoadConfigLocalSemArquivoEnvRetornaErro`; testes de integracao do CLI passaram a criar `.env` real por processo.
- Validacao: `go test ./...` passou; `go test ./... -count=1 -race` passou.

- ID: BUG-002
- Severidade: major
- Origem: finding de review, RF-14, RF-20
- Estado: fixed
- Causa raiz: o job de governanca do CI degradava para skip quando `ai-spec`/validator nao existiam e usava `|| true`, anulando o bloqueio de drift.
- Arquivos alterados: `.github/workflows/ci.yml`
- Teste de regressao: parse YAML validado e `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` executado com sucesso.
- Validacao: `python3 ... validate-taskfile.py Taskfile.yml` retornou `SUCCESS`; parse YAML de `.github/workflows/ci.yml` retornou `yaml ok`.

- ID: BUG-003
- Severidade: major
- Origem: finding de review, RF-16
- Estado: fixed
- Causa raiz: `check-conventional-commit.sh` aceitava apenas arquivo de mensagem, mas o CI chamava o script com `base` e `head`, quebrando a leitura com `head <ref>`.
- Arquivos alterados: `taskfiles/scripts/check-conventional-commit.sh`, `.github/workflows/ci.yml`
- Teste de regressao: `bash taskfiles/scripts/check-conventional-commit.sh HEAD~1 HEAD` passou; mensagem invalida em arquivo retornou exit 1.
- Validacao: `bash -n taskfiles/scripts/check-conventional-commit.sh` passou.

- ID: BUG-004
- Severidade: major
- Origem: finding de review, RF-25
- Estado: fixed
- Causa raiz: `task security:vulncheck` aceitava Trivy ausente e retornava sucesso apos imprimir recomendacao, removendo o gate HIGH/CRITICAL.
- Arquivos alterados: `taskfiles/security.yml`, `.github/workflows/ci.yml`
- Teste de regressao: `task security:vulncheck` sem Trivy agora falha em precondition com `trivy ausente`, e o CI instala Trivy antes de executar o gate.
- Validacao: `task --list-all` passou; `python3 ... validate-taskfile.py Taskfile.yml` retornou `SUCCESS`.

- ID: BUG-005
- Severidade: major
- Origem: finding de review, RF-17
- Estado: fixed
- Causa raiz: hooks locais usavam `command -v ... && ... || true`, entao ferramentas ausentes ou checks falhando nao bloqueavam commits.
- Arquivos alterados: `.pre-commit-config.yaml`
- Teste de regressao: `pre-commit validate-config .pre-commit-config.yaml` passou; hooks agora executam `goimports`, `golangci-lint` e `ai-spec` diretamente.
- Validacao: parse YAML de `.pre-commit-config.yaml` retornou `yaml ok`.

- ID: BUG-006
- Severidade: major
- Origem: finding de review, RF-04
- Estado: fixed
- Causa raiz: `Validate` nao checava ranges de pool tunables nem tamanho minimo de secret OTLP em production.
- Arquivos alterados: `configs/config.go`, `configs/config_test.go`, `internal/infrastructure/database/manager.go`
- Teste de regressao: `TestValidateProductionOTLPHeadersCurto` e `TestValidatePoolTunablesInvalidos`.
- Validacao: `go test ./configs/... ./internal/infrastructure/runtime/...` passou; `go test ./...` passou.

- ID: BUG-007
- Severidade: minor
- Origem: finding de review, robustez operacional
- Estado: fixed
- Causa raiz: o subsystem HTTP criava DB manager e provider OTel no `Start`, mas so guardava o servidor; `Stop` nao tinha como drenar DB/OTel.
- Arquivos alterados: `internal/infrastructure/runtime/http_subsystem.go`, `internal/infrastructure/runtime/runtime_test.go`
- Teste de regressao: `TestLazyServerSubsystemStopFechaDependencias`.
- Validacao: `go test ./internal/infrastructure/runtime/...` passou; `go test ./... -count=1 -race` passou.

## Comandos Executados

- `go test ./configs/... ./internal/infrastructure/runtime/...` -> passou.
- `go test ./...` -> passou.
- `go vet ./...` -> passou.
- `golangci-lint run ./...` -> passou com `0 issues`.
- `go build ./...` -> passou.
- `go test ./... -count=1 -race` -> passou.
- `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` -> `SUCCESS`.
- `task --list-all` -> passou.
- `git diff --check` -> passou.
- `pre-commit validate-config .pre-commit-config.yaml` -> passou.
- `bash taskfiles/scripts/check-conventional-commit.sh HEAD~1 HEAD` -> passou.
- `bash taskfiles/scripts/check-conventional-commit.sh /tmp/commit-msg-bad` -> falhou como esperado para mensagem invalida.
- `task security:vulncheck` sem Trivy local -> falhou como esperado em precondition (`trivy ausente`).
- `task security:vulncheck` apos instalacao do Trivy -> passou; Trivy reportou 0 vulnerabilidades em `go.mod`.
- `task ci` -> passou completo.

## Riscos Residuais

- A action `JailtonJunior94/orchestrator/.github/actions/setup-ai-spec@setup-action-v1` foi alinhada ao PRD, mas a execucao real dela depende do GitHub Actions.
