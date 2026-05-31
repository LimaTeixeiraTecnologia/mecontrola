# Tarefa 1.0: Bootstrap — Taskfile production + governança + hooks + tooling pinning + CODEOWNERS + SECURITY.md

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o **chassi operacional** do repositório greenfield `LimaTeixeiraTecnologia/mecontrola` antes de qualquer linha de código de produto: Taskfile production-ready (skill `taskfile-production`), pre-commit hooks via framework, conventional-commit hook, governance ai-spec já instalada e revalidada, CODEOWNERS, SECURITY.md, `tools.go`, `taskfiles/vars.yml` com versões pinadas, `.gitignore` cobrindo `.task/`. **Sem este bootstrap nenhuma outra task compila/lint passa**.

<requirements>
- Aplicar skill `taskfile-production` na íntegra (layout isolado obrigatório: `Taskfile.yml` raiz + `taskfiles/{build,test,lint,security,mocks,ci}.yml` + `taskfiles/scripts/`).
- `task --list-all` resolve todas as tarefas sem erro de include.
- `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` retorna `SUCCESS`.
- `task setup` instala pre-commit hooks + gitsign + ferramentas pinadas em `taskfiles/vars.yml`.
- `CODEOWNERS` na raiz com `* @JailtonJunior94`.
- `SECURITY.md` na raiz com política de disclosure (canal + SLA 7d + escopo + safe harbor).
- `tools.go` na raiz com `//go:build tools` listando deps Go de tooling (testify, mockery).
- `taskfiles/vars.yml` com `GOLANGCI_LINT_VERSION`, `MOCKERY_VERSION`, `GOVULNCHECK_VERSION`, `TRIVY_VERSION`, `COSIGN_VERSION`, `GITSIGN_VERSION`, `MIGRATE_VERSION`, `PRE_COMMIT_VERSION`, `TASK_VERSION=v3.51.1`.
- `.pre-commit-config.yaml` cobrindo: `gofmt`/`goimports`, `golangci-lint run --fast`, `ai-spec lint .`, conventional-commit `commit-msg` hook.
- `.gitignore` inclui `.task/`, `.env`, binários temporários, `coverage.out`.
- `.editorconfig` mínimo.
- `ai-spec doctor` + `ai-spec lint` retornam `pass`.
</requirements>

## Subtarefas

- [ ] 1.1 Aplicar skill `taskfile-production` no projeto (gera `Taskfile.yml` + `taskfiles/*.yml` + `taskfiles/scripts/` + `.taskrc.yml` + `.env.example`).
- [ ] 1.2 Editar `taskfiles/vars.yml` com versões pinadas das 8 ferramentas listadas em ADR-014.
- [ ] 1.3 Criar `tools.go` na raiz com `//go:build tools` + imports `_ "github.com/stretchr/testify"` e `_ "github.com/vektra/mockery/v2"`.
- [ ] 1.4 Criar `.pre-commit-config.yaml` com hooks: `gofmt`, `goimports`, `golangci-lint --fast`, `ai-spec lint`, conventional commits via hook customizado ou `conventional-pre-commit`.
- [ ] 1.5 Implementar tarefa `task setup` que instala pre-commit + gitsign + executa `pre-commit install --install-hooks` + configura `git config gpg.format x509` + `git config gpg.x509.program gitsign` + `git config commit.gpgsign true`.
- [ ] 1.6 Criar `CODEOWNERS` na raiz com `* @JailtonJunior94`.
- [ ] 1.7 Criar `SECURITY.md` na raiz com seções: Disclosure Channel, SLA de Resposta, Escopo, Safe Harbor (conforme ADR-013).
- [ ] 1.8 Atualizar `.gitignore` para incluir `.task/`, `.env`, `coverage.out`, `dist/`, `.idea/`, `.vscode/`.
- [ ] 1.9 Criar `.editorconfig` mínimo (UTF-8, LF, indent_size 4 para go, 2 para yaml/json).
- [ ] 1.10 Rodar `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` até `SUCCESS`.
- [ ] 1.11 Rodar `ai-spec inspect .` + `ai-spec doctor .` + `ai-spec lint .` e confirmar `pass`.

## Detalhes de Implementação

Ver techspec §"Arquitetura do Sistema" (árvore de diretórios) + ADR-009 (insecure.go placeholders) + ADR-013 (SECURITY.md conteúdo) + ADR-014 (vars.yml + tools.go) + skill `taskfile-production` §"Layout Isolado (obrigatorio)".

Não duplicar conteúdo do template do Taskfile aqui — chamar a skill `taskfile-production` consome `assets/*` dela.

## Critérios de Sucesso

- `task --list-all` lista pelo menos: `setup`, `build`, `test:unit`, `test:integration`, `lint`, `lint:fix`, `security:vulncheck`, `mocks:generate`, `migrate:up`, `migrate:down`, `run`, `check`, `ci`.
- `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` → `SUCCESS`.
- `ai-spec doctor .` → `tudo ok`; `ai-spec lint .` → `pass`.
- `cat CODEOWNERS` mostra `* @JailtonJunior94`.
- `cat SECURITY.md` cobre as 4 seções exigidas.
- `git config commit.gpgsign` retorna `true` após `task setup`.
- `pre-commit run --all-files` executa sem erro contra repo (mesmo vazio de código Go).
- Cobre RF-05, RF-08, RF-13, RF-16 (commit-msg hook), RF-17, RF-19, RF-20, RF-21 (pinning), RF-22 (`.task/` no .gitignore + schema comment).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `taskfile-production` — configurar Taskfile production-ready com layout isolado obrigatório, hooks de CI/CD e validação multiplataforma, conforme ADRs 014 e D-14/D-15/D-16

## Testes da Tarefa

- [ ] Testes unitários: n/a (tarefa de bootstrap; nenhum código Go produzido).
- [ ] Testes de integração: `task --list-all` + `validate-taskfile.py` + `ai-spec doctor/lint` executados localmente; resultado capturado em evidência.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `Taskfile.yml` (raiz)
- `taskfiles/{build,test,lint,security,mocks,ci}.yml`
- `taskfiles/scripts/*`
- `taskfiles/vars.yml`
- `.taskrc.yml`
- `.env.example`
- `.pre-commit-config.yaml`
- `tools.go`
- `CODEOWNERS`
- `SECURITY.md`
- `.gitignore`
- `.editorconfig`
