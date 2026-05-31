# PRD — MeControla Backend: Foundation Robusta e Readiness Operacional Mínima

<!-- spec-version: 9 -->

## Visão Geral

O MeControla é um agente financeiro conversacional via WhatsApp em PT-BR, voltado às classes C/D, com restrição dominante de custo de inferência LLM e LGPD reforçada desde o dia 1. Não existe backend hoje: o repositório `LimaTeixeiraTecnologia/mecontrola` nasce greenfield e precisa de uma **foundation production-proof** antes que o canal WhatsApp, o motor conversacional, o motor financeiro, o scheduler, o RBAC pleno, a LGPD operacional (audit/DSAR) e o Swagger sejam construídos — cada um desses módulos terá seu próprio PRD subsequente.

Este PRD define **apenas a foundation técnica + readiness operacional mínima**: chassi hexagonal Go modular, observabilidade OpenTelemetry ponta a ponta, Postgres com migrations e UnitOfWork, deploy reproduzível no Fly.io região `gru`, governança SDD (`ai-spec`) ativa desde o primeiro commit, e os mandatórios de tooling (Copilot CLI, Codex CLI, Claude Code CLI) já materializados. A foundation já nasce **preparada para evolução segura**: split futuro `server`/`worker` sem reescrita de domínio, troca futura de provider LLM via interface, migração futura de rate-limit para Redis, e expansão dos módulos hexagonais sem violar fronteiras.

Valor entregue: chassi que reduz a R$ 0 o custo recorrente de retrabalho estrutural quando os módulos de negócio começarem a ser construídos, garante rastreabilidade e auditoria via SDD desde o primeiro commit, e oferece um repositório que qualquer um dos três agentes (Copilot, Codex, Claude Code) consegue operar com paridade.

## Objetivos

- **OBJ-01**: Materializar o chassi monolito modular Go com layout hexagonal por módulo, composição do `devkit-go` v0.4.0 e binário único `cmd/server` com flag `APP_MODE`, sem débito estrutural que force reescrita ao adicionar módulos de negócio.
- **OBJ-02**: Ligar observabilidade OpenTelemetry OTLP (traces + metrics + logs) com redaction automática de PII, correlação trace ↔ log ↔ `request_id` e métricas HTTP/DB automáticas do `devkit-go` desde o primeiro deploy.
- **OBJ-03**: Estabelecer Postgres como fonte de verdade transacional com migrations versionadas (`golang-migrate`) e UnitOfWork transacional (`pkg/database` do `devkit-go`), incluindo seed mínimo (schema base e seeds operacionais — sem domínio de negócio neste ciclo).
- **OBJ-04**: Entregar deploy reproduzível no Fly.io região `gru` (app + Postgres managed com PITR), com CI/CD mínimo (test, lint, build, deploy), rollback documentado e runbook básico.
- **OBJ-05**: Ativar governança SDD obrigatória desde o primeiro commit (`ai-spec install`, `inspect`, `doctor`, `lint` verdes; CODEOWNERS com `@JailtonJunior94`; suporte paritário a Copilot/Codex/Claude Code), de forma que toda mudança subsequente nasça do fluxo `create-prd → create-technical-specification → create-tasks → execute-task`.
- **OBJ-06**: Manter o custo de infraestrutura da foundation ≤ **R$ 60/mês** (fora LLM, revisado na v8 — D-24), comportando 2 instâncias Fly shared-cpu-1x (server + worker) + Fly Postgres dev tier + Grafana Cloud free tier para o piloto fechado.

Métrica de sucesso agregada: um desenvolvedor novo consegue ir do `git clone` ao primeiro `task build` verde em ≤ 30 min, e ao primeiro `/health` 200 em ambiente Fly em ≤ 1 dia, sem precisar tocar em nada além das CLIs mandatórias.

## Histórias de Usuário

- Como **tech lead / mantenedor (`@JailtonJunior94`)**, quero que o repositório nasça com governança SDD ativa, para que toda feature subsequente entre pelo fluxo `create-prd` sem rota de fuga e produza rastreabilidade auditável.
- Como **dev de backend (1–2 pessoas)**, quero clonar o repo e ter `task setup`, `task build`, `task test:unit`, `task lint` funcionando localmente em minutos, para que eu não gaste tempo decidindo layout ou infra antes de implementar o primeiro módulo de negócio.
- Como **agente de IA (Copilot / Codex / Claude Code)**, quero encontrar adaptadores e skills paritários no repo (`.copilot/`, `.codex/`, `.claude/`, `.agents/skills/`), para que eu consiga operar o fluxo SDD com o mesmo contexto independentemente da CLI em uso.
- Como **operador Fly.io**, quero um deploy reproduzível na região `gru` com `/health`, `/ready`, `/live` respondendo e telemetria OTLP saindo, para que rollback e restore sigam o runbook sem improviso.
- Como **futuro dev do módulo `conversation` ou `agent`**, quero que a foundation já tenha modo `worker` separável via `APP_MODE`, eventbus in-process e fronteiras hexagonais rígidas, para que minha implementação não force refactor estrutural.

## Funcionalidades Core

- **Chassi hexagonal Go modular** — diretório `internal/<modulo>/{domain,application,adapters}` para os seis módulos de domínio previstos no discovery (`identity`, `conversation`, `agent`, `finance`, `notifications`, `telemetry`) materializados como esqueletos vazios prontos para receber código nos PRDs seguintes; cross-cutting de infraestrutura concentrado em `internal/infrastructure/` (config, observability, database, http, events, clock, ratelimit) — substitui o histórico `internal/platform/` do discovery; fronteiras de import enforced (regra `depguard`) impedindo violações arquiteturais.
- **Binário único `mecontrola` via `spf13/cobra` v1.10.2 com subcomandos `server`, `worker`, `migrate`** (D-19, ADR-010) — segue o pattern de referência `JailtonJunior94/financial/cmd/main.go`: `cmd/main.go` é o root cobra que registra subcomandos cujos handlers chamam `cmd/<subcmd>.Run()`. Não há `APP_MODE` env nem `--migrate-only` flag. Cada subcomando é independente: `mecontrola server` sobe HTTP + worker pool de domínio; `mecontrola worker` sobe apenas o runtime worker (placeholder até PRDs futuros registrarem jobs); `mecontrola migrate` aplica migrations e termina. Shutdown coordenado via `Shutdowner` do `devkit-go` em cada subcomando.
- **Composição `devkit-go` v0.4.0** — `pkg/http_server` (Chi v5, Problem Details RFC 7807, middlewares de segurança, métricas Prometheus, health/ready/live), `pkg/observability` (OTLP traces + metrics + logs, redaction de PII, correlação `request_id`), `pkg/database` (pgx/v5, `manager.Manager`, `UnitOfWork[T]`, `migration` golang-migrate).
- **Configuração via `spf13/viper` v1.21.0 + `.env` mandatório** — pasta **`configs/config.go`** na raiz (D-17 + D-18) com struct `Config` agrupada por **tipo de variável** via `mapstructure:",squash"`: `AppConfig` (APP_MODE, ENVIRONMENT), `HTTPConfig` (PORT, SERVICE_NAME_API), `DBConfig` (DB_HOST/PORT/USER/PASSWORD/NAME/SSL_MODE + pool tunables + DSN()/SafeDSN()), `O11yConfig` (OTEL_*, LOG_LEVEL, LOG_FORMAT, SERVICE_VERSION). `.env` é **obrigatório no startup local** (Viper aborta bootstrap se ausente); em produção Fly, Viper consome env vars injetadas via Fly secrets — sem necessidade de arquivo `.env`. `Config.Validate()` roda **antes** de qualquer subsistema inicializar e rejeita defaults inseguros em produção (senha curta, secret fraco, credenciais default).
- **Build, test, lint e security via Taskfile (go-task) production-ready** — orquestrador `Taskfile.yml` fino na raiz + `taskfiles/` com Taskfiles por domínio (`build.yml`, `test.yml`, `lint.yml`, `security.yml`, `mocks.yml`, `ci.yml`) + `taskfiles/scripts/` para helpers, conforme skill `taskfile-production` (D-14). Comandos típicos: `task build`, `task test:unit`, `task test:integration`, `task lint`, `task security:vulncheck`, `task setup`, `task check`, `task ci`. Sem Makefile no repositório (D-14).
- **Pre-commit hooks locais** — instalados via `pre-commit` framework com `task setup`; cobrem `gofmt`/`goimports`, `golangci-lint`, `ai-spec lint`, e `commit-msg` enforcement de conventional commits (skill `semantic-commit`). Production-ready: pega drift antes do commit, não no CI (D-10 + D-12).
- **Testes de integração com testcontainers** — desde a foundation, validam `pkg/database` UoW + migration de exemplo contra Postgres ephemeral via `testcontainers-go`; rodam no CI em job dedicado para evitar acoplamento com testes unitários (D-13).
- **Cobertura reportada como artefato** — `task test:unit` gera relatório de cobertura anexável ao PR; sem gate de cobertura no MVP (D-11). Gates entram nos PRDs de módulos com lógica de negócio.
- **CI/CD mínimo no GitHub Actions** — workflow CI roda `task ci` (test + lint + build + security:vulncheck + ai-spec doctor/lint + validate-taskfile + conventional commits check) em PRs e na `main`; workflow CD constrói imagem **distroless `gcr.io/distroless/static-debian12:nonroot`** (D-23), publica e faz deploy no Fly região `gru` com **`fly.toml` declarando 2 processes — `app=mecontrola server` + `worker=mecontrola worker`** (D-24) — em push aprovado para `main`; setup do `ai-spec` via Action oficial; setup do `task` via `arduino/setup-task`.
- **Supply chain hardening desde o dia 1** — `task security:vulncheck` roda **`govulncheck`** (oficial Go com call-graph) + **`trivy fs`** (deps + Dockerfile + secrets em commit) (D-25); **Dependabot** nativo do GitHub (`.github/dependabot.yml`) agrupa updates por ecosystem (`gomod`, `github-actions`, `docker`) abrindo PRs auto-mergeable para minor/patch (D-26).
- **Health/ready/live em produção** — `/health` (processo), `/ready` (dependências, incluindo DB), `/live` (liveness puro) expostos por `pkg/http_server`; usados pelo Fly como probes.
- **Governança SDD instalada** — `.agents/skills/` com `create-prd`, `create-technical-specification`, `create-tasks`, `execute-task`, `go-implementation`, `object-calisthenics-go`, `review`, `pull-request`, `semantic-commit` (entre outras instaladas por `ai-spec install --tools claude,gemini,codex,copilot --langs go`); manifestos `AGENTS.md`, `CLAUDE.md`, `CODEX.md`, `COPILOT.md`, `GEMINI.md`; `.ai_spec_harness.json` versionado.
- **`CODEOWNERS` global** — `* @JailtonJunior94` cobrindo todo o repositório, obrigando review do owner em qualquer PR.

## Requisitos Funcionais

- **RF-01**: O serviço HTTP DEVE expor `/health`, `/ready` e `/live`. `/ready` DEVE refletir o status das dependências (mínimo: Postgres). Todos retornam JSON com `status` e `version`.
- **RF-02**: O binário `mecontrola` DEVE expor subcomandos `cobra` (D-19): `server` (sobe HTTP + scheduler in-process placeholder), `worker` (sobe apenas runtime worker — placeholder até PRDs subsequentes registrarem jobs), `migrate` (aplica migrations e termina). Cada subcomando é independente; o root command `mecontrola` (sem subcomando) imprime help e sai com exit code 0. Layout: `cmd/main.go` (root cobra) + `cmd/{server,worker,migrate}/cmd.go` (cada um expõe `Run() error`).
- **RF-03**: O subcomando `mecontrola migrate` DEVE executar migrations pendentes via `golang-migrate` (D-19) e terminar com exit code 0 em sucesso ou ≠ 0 em falha, sem subir HTTP nem worker. NÃO há flag `--migrate-only` no subcomando `server` (separação por subcomando elimina o flag).
- **RF-04**: A configuração DEVE ser carregada por **`spf13/viper`** v1.21.0 a partir de `configs/config.go` (pasta na raiz — D-17 + D-18) com struct `Config` composta por grupos via `mapstructure:",squash"`: `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig` (foundation; grupos adicionais como `AuthConfig`, `RabbitMQConfig`, `OutboxConfig`, `ConsumerConfig`, `WorkerConfig` entram nos PRDs subsequentes que os introduzirem). Em desenvolvimento local o arquivo `.env` DEVE existir na raiz do projeto (Viper `ReadInConfig`); em produção Fly o `.env` é dispensado (Viper consome env vars via `AutomaticEnv`). `Config.Validate()` DEVE rodar antes de qualquer subsistema inicializar e DEVE rejeitar: (a) senha de DB <16 caracteres em produção; (b) chaves/secrets <64 caracteres em produção; (c) credenciais default conhecidas (`CHANGE_ME_*`, `guest:guest`, `your_secret_key`); (d) valores fora de range razoável para `Port`, pool tunables, sample rate; (e) `Environment` ∈ {`local`, `staging`, `production`}. `DBConfig` DEVE expor `DSN()` (com senha, uso interno) e `SafeDSN()` (mascarando senha como `***`, único formato permitido em logs/erros — proibido logar `DSN()`).
- **RF-05**: O repositório DEVE prover automação via **Taskfile (go-task)** seguindo a skill `taskfile-production` (D-14), com pelo menos as tarefas `setup`, `build`, `test:unit`, `test:integration`, `lint`, `lint:fix`, `security:vulncheck`, `mocks:generate`, `migrate:up`, `migrate:down`, `run`, `check` (agrega lint + test + security), `ci` (pipeline agregada). Todas executáveis localmente sem dependência além de Go 1.26.3, Docker, `task`, `pre-commit` e `ai-spec`.
- **RF-06**: O pipeline CI DEVE consumir as tarefas do Taskfile (e.g. `task ci`, `task lint`, `task test:unit`, `task test:integration`, `task security:vulncheck`) em todo PR aberto contra `main` e em todo push para `main`; o YAML do CI NÃO deve duplicar comandos do Taskfile. Falha em qualquer tarefa bloqueia o merge.
- **RF-07**: O pipeline CD DEVE construir imagem **`gcr.io/distroless/static-debian12:nonroot`** (D-23 + ADR-011) com o binário `mecontrola` estaticamente compilado (`CGO_ENABLED=0`, `-ldflags="-s -w"`), publicá-la em **`ghcr.io/limateixeiratecnologia/mecontrola`** (D-27 + ADR-013) com tag por commit SHA + tag semver (`v0.x.y`), **assiná-la com `cosign` keyless via OIDC** do GitHub Actions e gerar **atestados SLSA** (provenance + SBOM) anexados à imagem. Deploy no Fly.io região `gru` com `fly.toml` declarando **2 processes** (D-24) em push aprovado para `main`. Rollback via `fly releases rollback` ou `fly deploy --image <previous>` em até 1 comando. SBOM SPDX (gerado por `trivy image --format spdx-json`) DEVE ser anexado ao release no GitHub.
- **RF-08**: O repositório DEVE conter `CODEOWNERS` na raiz com `* @JailtonJunior94` como owner obrigatório global; configuração de branch protection no GitHub DEVE exigir review do CODEOWNER em PRs para `main` (registrado neste PRD; configuração executada no GitHub após criação do repo remoto).
- **RF-09**: O layout `internal/<modulo>/{domain,application,adapters}` DEVE estar presente para os **seis módulos de domínio** (`identity`, `conversation`, `agent`, `finance`, `notifications`, `telemetry`), mesmo que vazios. O cross-cutting de infraestrutura DEVE viver em **`internal/infrastructure/`** com sub-pacotes mínimos: `observability`, `database`, `http`, `events`, `clock`, `errors`, `runtime`. A **configuração centralizada** DEVE viver fora do `internal/infrastructure/`, em pasta **`configs/`** na raiz (D-18), por convenção de "config as data" — `configs/config.go` consumido por `cmd/server` e injetado nos subsistemas. Regra `depguard` em `.golangci.yml` DEVE impedir imports invertidos: (a) `<modulo>/domain` NÃO importa `adapters` nem `application`; (b) `<modulo>/application` NÃO importa `adapters`; (c) `<modulo>/domain` NÃO importa `internal/infrastructure/*` nem `configs/*` (Domain puro); (d) cross-module só via interface declarada em `application`.
- **RF-10**: A composição em `internal/infrastructure/` DEVE expor inicialização do `devkit-go` v0.4.0 (`pkg/http_server` em `internal/infrastructure/http`, `pkg/observability` em `internal/infrastructure/observability`, `pkg/database` em `internal/infrastructure/database`) consumida por `cmd/server`; o `manager.Manager` do devkit-go DEVE ser materializado uma única vez em `internal/infrastructure/database` e injetado em cada módulo via construtor. Eventbus tipado via generics em **`internal/infrastructure/events`** desde o primeiro commit, com API mínima (`Publish[E]`, `Subscribe[E]`, `Close`) e testes unitários, evitando refactor estrutural quando o PRD de `conversation` chegar.
- **RF-11**: O serviço DEVE emitir traces, metrics e logs OTLP correlacionados em toda request HTTP e em toda transação de DB, com `request_id` propagado em todos os três sinais; redaction de PII (`phone`, `password`, `token`, `card_number`, `amount`) DEVE estar ativa por padrão no `pkg/observability`.
- **RF-12**: O schema inicial Postgres DEVE conter pelo menos uma migration de exemplo aplicável e revertível via `golang-migrate up` / `down`, validando o caminho de migrations antes da chegada dos módulos de domínio.
- **RF-13**: O repositório DEVE nascer com o baseline do `ai-spec install --tools claude,gemini,codex,copilot --langs go` materializado no primeiro commit (`.agents/skills/`, `.claude/`, `.codex/`, `.copilot/`, `.gemini/`, `AGENTS.md`, `CLAUDE.md`, `CODEX.md`, `COPILOT.md`, `GEMINI.md`, `.ai_spec_harness.json`, `skills-lock.json`).
- **RF-14**: O CI DEVE executar `ai-spec doctor` e `ai-spec lint` (preflight de governança) em todo PR e bloquear o merge em caso de drift.
- **RF-15**: O repositório DEVE incluir runbook básico em `docs/runbooks/` cobrindo no mínimo: deploy via Fly, rollback via Fly, restore PITR do Fly Postgres, rotação de secret e procedimento de upgrade do `ai-spec` (`ai-spec upgrade . --check` + revalidação).
- **RF-16**: O repositório DEVE enforçar **conventional commits** em dois pontos (D-10): (a) `commit-msg` hook local instalado por `task setup` que rejeita commits fora do padrão; (b) job no CI que valida o histórico de commits do PR e bloqueia merge em caso de violação. Alinha com `ai-spec semver-next` e `ai-spec changelog`.
- **RF-17**: A tarefa `task setup` DEVE instalar os pre-commit hooks locais via framework `pre-commit` (D-12), cobrindo no mínimo: `gofmt`/`goimports`, `golangci-lint run --fast`, `ai-spec lint .`, e o `commit-msg` enforcement de conventional commits. A configuração DEVE viver em `.pre-commit-config.yaml` na raiz.
- **RF-18**: O repositório DEVE conter testes de integração com **testcontainers-go** (D-13) que sobem Postgres ephemeral e validam ponta-a-ponta: (a) aplicação e reversão da migration de exemplo via `golang-migrate up`/`down`; (b) commit/rollback transacional via `UnitOfWork[T]` do `devkit-go`. Job dedicado no CI executa esses testes via `task test:integration` e bloqueia o merge em falha. **Relatório de cobertura DEVE ser postado como comentário no PR** via action `fgrosse/go-coverage-report` (D-28 + ADR-015) — sem gate de cobertura (D-11), apenas visibilidade.
- **RF-19**: O Taskfile DEVE seguir o **layout isolado obrigatório** da skill `taskfile-production` (D-15): `Taskfile.yml` na raiz como orquestrador fino (somente `version`, `includes`, `vars`, `tasks` de alto nível); `taskfiles/` com Taskfiles por domínio (`build.yml`, `test.yml`, `lint.yml`, `security.yml`, `mocks.yml`, `ci.yml`); `taskfiles/scripts/` com helpers (`.sh`/`.ps1`/`.py`); `.taskrc.yml` para config de execução; `.env.example` documentando variáveis. **Nenhum** script de automação pode viver em `cmd/`, `internal/` ou `pkg/`.
- **RF-20**: O Taskfile DEVE passar `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` com retorno `SUCCESS`. Job dedicado no CI executa essa validação e bloqueia merge em falha. Adicionalmente, `task --list-all` DEVE resolver todas as tarefas sem erro de include.
- **RF-21**: A versão do Task DEVE ser **pinada** em `TASK_VERSION` (referência atual: `v3.51.1` — D-16), usada idêntica em ambiente local (via `task setup`/`task setup` ou script `install-task.sh` da skill) e no CI (via Action oficial `arduino/setup-task` ou similar com versão exata). Comportamento DEVE ser idêntico em macOS, Linux (Ubuntu) e Windows.
- **RF-22**: O repositório DEVE conter `.task/` no `.gitignore`, o comentário de schema `# yaml-language-server: $schema=https://taskfile.dev/schema.json` no topo de cada Taskfile, e declarar `version: '3'` em todos eles, conforme styleguide da skill.

## Restrições Técnicas de Alto Nível

- **Linguagem obrigatória**: Go **1.26.3** (alinhada ao `devkit-go` v0.4.0), fixada no `go.mod` desde o primeiro commit.
- **Persistência obrigatória**: Postgres único (Fly Postgres managed em produção). Sem outros stores de dados na foundation.
- **Foundation obrigatória**: `JailtonJunior94/devkit-go` v0.4.0 (`pkg/http_server`, `pkg/observability`, `pkg/database`). Sem reinventar HTTP, OTel ou DB.
- **Arquitetura obrigatória**: monolito modular com camadas hexagonais por módulo. Sem microserviços, sem broker externo, sem Redis no MVP.
- **Hosting obrigatório**: Fly.io região `gru` (PaaS). Sem AWS/GCP/Azure no MVP.
- **Observabilidade obrigatória**: OpenTelemetry OTLP gRPC exportando para **Grafana Cloud free tier** (Tempo/Loki/Mimir) — zero infra extra, dashboards prontos, dentro do budget. Coletor self-hosted fica como opção futura caso o free tier deixe de comportar a volumetria.
- **Governança obrigatória**: `ai-spec` mandatório; toda mudança de comportamento DEVE começar em `create-prd`. Sem rota alternativa.
- **CLIs obrigatórias**: Copilot CLI, Codex CLI e Claude Code CLI são suportadas e mandatórias com paridade. Gemini CLI é **mantida** pelo default oficial `--tools claude,gemini,codex,copilot` do `ai-spec install` (custo zero adicional, paridade com o orchestrator).
- **Codeowner obrigatório**: `@JailtonJunior94` DEVE ser o codeowner global do repositório.
- **Organização GitHub obrigatória**: o repositório DEVE nascer em `https://github.com/LimaTeixeiraTecnologia/mecontrola`.
- **Compliance**: LGPD aplicável ao MVP (titular: pessoa física brasileira). A foundation NÃO implementa controles operacionais LGPD (audit, DSAR, retenção, consent) — esses ficam em PRD próprio. A foundation apenas NÃO IMPEDE esses controles: redaction de PII está ativa, schema permite tabelas append-only futuras, há espaço arquitetural para decorator de audit em use cases.
- **Performance/escalabilidade (alvos a respeitar pela foundation)**: o chassi DEVE comportar pico p99 ~50 msg/s e 1–10k usuários ativos previstos no discovery, sem mudanças estruturais; deploy inicial em Fly shared-cpu-1x DEVE ser suficiente para o piloto fechado.
- **Custo**: infra inicial (fora LLM) DEVE permanecer ≤ R$ 25/mês.
- **Sensibilidade de dados**: nenhum segredo pode ser logado nem aparecer em payload de erro; redaction automática enforced pelo `pkg/observability`.

## Fora de Escopo

Este PRD **NÃO** cobre:

- **Canal WhatsApp** (webhook Meta Cloud API, HMAC, idempotência por `message_id`, inbox/outbox, retry, envio via Graph API) — Epic 06 do discovery, vira **PRD próprio**.
- **Motor conversacional** (cliente OpenAI `gpt-4o-mini`, intent router determinístico, prompt registry, tools com JSON Schema, sliding window, working memory JSONB, cache exato, budget hard-cap) — Epic 07, **PRD próprio**.
- **Motor financeiro** (movimentações, categorias, regras, metas, saldos) — Epic 08, **PRD próprio**.
- **Identity, RBAC e Auth pleno** (usuário final, sessões, JWT/refresh com rotação, roles/permissions completas, middleware `requirePermission`) — Epic 04, **PRD próprio**. A foundation deixa apenas o módulo `identity` esqueletado.
- **Rate-limit, segurança HTTP avançada e validação de DTO** (token bucket Postgres, CORS, security headers afinados, validator v10, mapeamento global RFC 7807 por contexto) — Epic 05, **PRD próprio**. A foundation usa apenas defaults do `devkit-go`.
- **Scheduler embutido, alertas e lembretes** — Epic 09, **PRD próprio**.
- **Swagger e contrato público** — Epic 10, **PRD próprio**.
- **LGPD operacional reforçada** (consents, audit log append-only via decorator, jobs de retenção por classe, endpoints DSAR `export`/`delete`) — Epic 11, **PRD próprio**. A foundation apenas garante que a arquitetura admite esses controles.
- **Runbooks operacionais avançados** (Webhook caído, Custo LLM, DSAR) — entram nos PRDs dos módulos correspondentes; a foundation entrega só os runbooks de infra (deploy, rollback, restore PITR, rotação de secret, upgrade do `ai-spec`).
- **Painel administrativo web (frontend)**, **landing page**, **checkout**, **billing**, **inadimplência**, **multi-provider LLM**, **multi-região**, **read-replicas Postgres**, **cache semântico por embedding**, **DPO formal e ROPA completo** — fora do escopo de qualquer PRD de backend MVP (alguns são épicos próprios, outros são fase pós-MVP).
- **Tests de carga, chaos test, smoke trimestral** — entram em PRD de hardening na Fase 5 do roadmap do discovery.

(Riscos técnicos de implementação detalhada serão tratados na Especificação Técnica derivada deste PRD.)

## Critérios de Sucesso

Critérios mensuráveis para considerar a foundation pronta:

- **CS-01**: `ai-spec inspect`, `ai-spec doctor` e `ai-spec lint` retornam `pass` no repositório.
- **CS-02**: `/health` responde 200 em ambiente Fly região `gru` em deploy de produção real.
- **CS-03**: Pipeline CI verde em pelo menos um PR de exemplo (`test` + `lint` + `build` + `ai-spec doctor/lint`).
- **CS-04**: Pipeline CD executa deploy bem-sucedido no Fly região `gru` a partir de push para `main`.
- **CS-05**: Span OTel correlaciona uma request HTTP (`/health` ou `/ready`) com uma transação Postgres em uma única árvore de trace exportada para o coletor.
- **CS-06**: Migration de exemplo aplica via `task migrate:up` e reverte via `task migrate:down` sem erro.
- **CS-07**: Rollback testado: `fly releases rollback` (ou `fly deploy --image <previous>`) restaura a versão anterior do app em menos de 5 min em ambiente Fly.
- **CS-08**: Tempo do `git clone` ao primeiro `task build` verde, em uma máquina de dev novo com Go e Docker já instalados, ≤ 30 min.
- **CS-09**: `CODEOWNERS` ativo no GitHub e cobertura de review obrigatória do owner validada em PR de exemplo.
- **CS-10**: Os três agentes mandatórios (Copilot, Codex, Claude Code) conseguem listar e invocar a skill `create-prd` no repositório sem erro (validação manual de paridade).
- **CS-11**: Commit fora do padrão conventional commits é rejeitado pelo `commit-msg` hook local **e** pelo job de CI; PR com histórico não-conformante não pode ser mergeado (D-10 + RF-16).
- **CS-12**: `task setup` em uma máquina nova instala os pre-commit hooks com sucesso e o primeiro `git commit` dispara as verificações configuradas (D-12 + RF-17).
- **CS-13**: O job de testes de integração com testcontainers-go passa em CI verde, com Postgres ephemeral subindo, migration de exemplo aplicada/revertida e commit/rollback de UoW validados (D-13 + RF-18).
- **CS-14**: `python3 .agents/skills/taskfile-production/scripts/validate-taskfile.py Taskfile.yml` retorna `SUCCESS` em CI e localmente (D-14 + RF-20).
- **CS-15**: `task --list-all` resolve todas as tarefas declaradas em `Taskfile.yml` + `taskfiles/*.yml` sem erro de include (RF-19 + RF-20).
- **CS-16**: `task ci` executa localmente o caminho crítico (lint + test:unit + test:integration + security:vulncheck) com o mesmo resultado obtido no GitHub Actions, comprovando paridade local/CI (D-14 + D-16 + RF-21).
- **CS-17**: Nenhum script de automação (`.sh`, `.ps1`, `.py`) vive em `cmd/`, `internal/` ou `pkg/`; todos os helpers estão em `taskfiles/scripts/` (D-15 + RF-19).
- **CS-18**: `Config.Validate()` rejeita corretamente, em testes table-driven, os 5 cenários enumerados em RF-04 (senha curta, secret fraco, credenciais default, ranges inválidos, environment inválido) e aceita um `.env` de exemplo válido (D-17 + D-18 + RF-04).
- **CS-19**: `go test ./configs/...` cobre 100% dos validadores e nenhum log de qualquer subsistema contém `DBConfig.DSN()` cru (todos usam `SafeDSN()`) — verificado por teste de redaction (RF-04 + R-SEC-001).
- **CS-20**: Em dev local, ausência do arquivo `.env` aborta o bootstrap com erro explícito apontando o caminho esperado e os campos faltantes; em Fly prod (sem `.env`), bootstrap completa lendo env vars nativas (RF-04).
- **CS-21**: `mecontrola --help` lista os subcomandos `server`, `worker`, `migrate`; cada subcomando responde a `--help`; integration test executa cada um e valida exit code (D-19 + RF-02 + RF-03).
- **CS-22**: `mecontrola migrate` aplica e logra a versão final; `mecontrola server` sobe HTTP em modo isolado; `mecontrola worker` sobe runtime worker idle ("sem jobs registrados") sem erro (D-19 + RF-02).
- **CS-23**: Imagem final do binário publicada com base `gcr.io/distroless/static-debian12:nonroot`, contendo apenas `/mecontrola` + CA certs; `docker image inspect` confirma `User: nonroot`; tamanho ≤ 30 MB (D-23 + ADR-011).
- **CS-24**: `fly status` mostra ≥ 1 instância de cada um dos 2 processes (`app` e `worker`) em estado `started`; ambos respondem a `fly logs -i <id>` sem erro de bootstrap (D-24 + ADR-011).
- **CS-25**: `task security:vulncheck` executa `govulncheck ./...` + `trivy fs --severity HIGH,CRITICAL --exit-code 1 .` e bloqueia o CI se houver CVE HIGH/CRITICAL não-suprimida; supressões vivem em `.trivyignore` com data e referência ao CVE (D-25 + ADR-012).
- **CS-26**: `.github/dependabot.yml` versionado abre PRs com grouping conforme D-26; PR de exemplo (minor patch) é auto-mergeado após CI verde + review do CODEOWNER (D-26 + ADR-012).
- **CS-27**: `cosign verify --certificate-identity-regexp '^https://github.com/LimaTeixeiraTecnologia/mecontrola/' --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' ghcr.io/limateixeiratecnologia/mecontrola:<sha>` retorna `verified=true`; `cosign verify-attestation` valida provenance + SBOM (D-27 + ADR-013).
- **CS-28**: PR de exemplo recebe comentário automático com tabela de cobertura por package; comentário é atualizado em push subsequente (D-28 + ADR-015).
- **CS-29**: `task --list-all` mostra versões pinadas; `git diff taskfiles/vars.yml` em um deploy mostra exatamente quais versões mudaram; `task setup` reproduz versões idênticas local e CI (D-29 + ADR-014).
- **CS-30**: `SECURITY.md` presente na raiz; commit não-assinado é rejeitado pela branch protection da `main` (validado com PR de exemplo); `git log --show-signature` mostra "Good signature" em todos os commits do CODEOWNER (D-30 + ADR-013).

## Riscos Iniciais

- **R-01 — Drift de governança** entre versão do binário `ai-spec` instalado e baseline do repositório quando novas releases do orchestrator forem publicadas.
  - Impacto: skills/adaptadores ficam desatualizados; `lint` quebra silenciosamente em CI.
  - Mitigação: pin do `ai-spec` via Action oficial no CI; `ai-spec upgrade . --check` rodado periodicamente; revalidação obrigatória após upgrade.
- **R-02 — Dependência `devkit-go` v0.4.0 sem release nova** durante a janela da foundation, atrasando fixes de bugs descobertos no chassi.
  - Impacto: chassi com bug conhecido sem caminho rápido de correção.
  - Mitigação: fork local opcional sob `replace` no `go.mod` como contingência; abrir PR upstream no `devkit-go`; registrar bug em backlog.
- **R-03 — Quota / capacidade do Fly Postgres dev tier insuficiente** mesmo no piloto fechado.
  - Impacto: timeout / lentidão antes de qualquer carga real chegar.
  - Mitigação: smoke test após deploy; upgrade documentado para production tier em runbook; monitorar p99 de tx desde dia 1.
- **R-04 — Aprovação Meta de número WhatsApp comercial em paralelo** (não bloqueia a foundation, mas é dependência externa do produto inteiro).
  - Impacto: cronograma do produto escorrega independente do código pronto.
  - Mitigação: iniciar processo de verificação Meta na semana 1 em paralelo à foundation; manter número de teste.
- **R-05 — Risco de violação de fronteira hexagonal silenciosa** se `depguard` não for configurado corretamente desde o início.
  - Impacto: módulos passam a se acoplar mal antes de termos código de negócio, forçando refactor depois.
  - Mitigação: regra `depguard` em `.golangci.yml` desde o primeiro commit; teste de import em pacote de exemplo; review obrigatório do CODEOWNER.

(Riscos de implementação técnica adicionais serão detalhados na Especificação Técnica.)

## Dependências Externas

- **`JailtonJunior94/devkit-go` v0.4.0** — foundation técnica obrigatória; sem substituição.
- **`JailtonJunior94/orchestrator`** — fonte de governança SDD (`ai-spec install`/`upgrade`); skills canônicas embutidas no binário `ai-spec` versão 0.26.0 (ou superior).
- **Fly.io** — app + Postgres managed na região `gru`; secrets nativos; PITR; setup CLI `flyctl`.
- **GitHub** — hospedagem do repositório `LimaTeixeiraTecnologia/mecontrola`; Actions para CI/CD; CODEOWNERS + branch protection na `main`.
- **`testcontainers-go`** — biblioteca para subir Postgres ephemeral em testes de integração (D-13); requer Docker disponível no runner do CI.
- **`pre-commit`** — framework de hooks locais (D-12) com configuração versionada em `.pre-commit-config.yaml`.
- **`go-task` / Task** v3.51.1 (D-14, D-16) — orquestrador de automação; substitui Makefile. Binário oficial publicado em https://github.com/go-task/task/releases.
- **Action de CI `arduino/setup-task`** (ou equivalente) — para pinar `TASK_VERSION` idêntica em local e CI.
- **`spf13/viper` v1.21.0** (D-17) — loader de configuração (parse `.env` + env vars + key replacer); fonte: https://github.com/spf13/viper.
- **`spf13/cobra` v1.10.2** (D-19) — framework de subcomandos CLI; fonte: https://github.com/spf13/cobra. Pattern de referência: https://github.com/JailtonJunior94/financial/blob/main/cmd/main.go.
- **`gcr.io/distroless/static-debian12:nonroot`** (D-23) — base image Docker minimalista; fonte: https://github.com/GoogleContainerTools/distroless.
- **`govulncheck`** (Google oficial) + **`trivy`** (Aqua Security) (D-25) — scanners de vulnerabilidade Go + filesystem; instalação via release pinada no CI.
- **GitHub Dependabot** (D-26) — bot nativo de update de deps; configurado por `.github/dependabot.yml`.
- **GHCR (GitHub Container Registry)** (D-27) — registry de imagens; autenticação OIDC nativa via GitHub Actions; sem credencial extra.
- **`cosign`** (Sigstore) (D-27) — keyless image signing via OIDC; atestados SLSA; integração com Rekor (transparency log).
- **`fgrosse/go-coverage-report` action** (D-28) — comentário automático de cobertura no PR.
- **`gitsign`** (Sigstore) (D-30) — keyless commit/tag signing; substitui GPG tradicional; OIDC via GitHub.
- **Coletor OTLP** — Grafana Cloud (free tier) ou self-hosted no Fly (decisão em pendência).
- **CLIs operacionais obrigatórias** — `ai-spec`, `gh`, `claude` (Claude Code CLI), `codex` (Codex CLI), `copilot` (GitHub Copilot CLI), `fly` (flyctl), `go` 1.26.3, `docker` (Postgres local em dev e testcontainers em CI), `pre-commit` (framework para hooks locais), **`task` (go-task) v3.51.1** (orquestrador production-ready — D-14 + D-16), `python3` (para `validate-taskfile.py` e `check-task-version.py` da skill `taskfile-production`), **`cosign`** (D-27 — verify de imagem em runbook), **`gitsign`** (D-30 — assinatura local de commits).

## Suposições e Questões em Aberto

### Suposições explícitas (consolidadas após decisões da v2)

- **S-01**: O módulo Go do projeto será `github.com/LimaTeixeiraTecnologia/mecontrola`.
- **S-02**: O repositório nasce **privado** na organização `LimaTeixeiraTecnologia` (decisão consolidada na v2).
- **S-03**: A equipe inicial é de 1–2 devs, compatível com a operação de monolito modular sem microserviços.
- **S-04**: O usuário tem `ai-spec` 0.26.0+ instalado localmente e `gh` CLI autenticado, bem como `fly` (flyctl) configurado para a região `gru`.
- **S-05**: O ciclo da foundation cabe nas Fases 0–1 do roadmap do discovery (semanas 1–6) cobrindo Epic 01 (Plataforma) + Epic 02 (Observabilidade) + Epic 03 (Postgres+UoW) + CI/CD mínimo.
- **S-06**: A versão de Go fixada é **1.26.3** (decisão consolidada na v2), alinhada ao `devkit-go` v0.4.0.
- **S-07**: O orçamento de infra da foundation permanece ≤ **R$ 60/mês** (revisado na v8 — D-24) usando 2 instâncias Fly shared-cpu-1x (server + worker) + Fly Postgres dev tier + Grafana Cloud free tier.

### Decisões consolidadas (resolvidas na v2)

As pendências P-01 a P-09 da v1 foram resolvidas e promovidas a decisões firmes desta v2:

- **D-01** (ex P-01): Go **1.26.3** fixado no `go.mod`.
- **D-02** (ex P-02): Sinais OTLP exportados para **Grafana Cloud free tier** (Tempo/Loki/Mimir).
- **D-03** (ex P-03): **CLI Gemini mantida** pelo default oficial do `ai-spec install` (`--tools claude,gemini,codex,copilot`); paridade total com Copilot/Codex/Claude permanece o mandato inegociável.
- **D-04** (ex P-04): **Sem `LICENSE`** declarada no repositório (proprietary private no MVP); revisitar antes de qualquer mudança para visibilidade public/internal.
- **D-05** (ex P-05): Política de tag = **`v0.1.0`** no primeiro deploy de produção; pré-releases marcados como **`v0.0.x`** (ex.: `v0.0.1`, `v0.0.2`); semver-next via skill `ai-spec semver-next`.
- **D-06** (ex P-06): Repositório **private** em `LimaTeixeiraTecnologia/mecontrola`.
- **D-07** (ex P-07): Branch protection na `main` = **CODEOWNER review aprovado** + **status checks obrigatórios do CI** (test, lint, build, `ai-spec doctor`, `ai-spec lint`) + **linear history** + **squash-merge** (changelog limpo via `semantic-commit` + `ai-spec changelog`).
- **D-08** (ex P-08, revisada na v5): Eventbus in-process via canais Go materializado em **`internal/infrastructure/events`** desde o primeiro commit, com API **tipada via generics Go 1.26** (`Publish[E Event]`, `Subscribe[E Event]`). Substitui o histórico `internal/platform/events` referenciado em versões anteriores deste PRD.
- **D-09** (ex P-09): Seeds de **roles/permissions iniciais** ficam no **PRD do módulo Identity** (futuro); a foundation entrega apenas migration de exemplo neutra, sem dados de domínio.
- **D-10** (v3): **Conventional commits obrigatórios** com enforcement em duas camadas: `commit-msg` hook local (instalado via `task setup`) + job de validação no CI. Alimenta `ai-spec semver-next` (D-05) e `ai-spec changelog` sem brecha.
- **D-11** (v3): **Sem gate de cobertura** de testes no MVP; cobertura é reportada como artefato anexável ao PR. Gates de cobertura entram nos PRDs dos módulos com lógica de negócio (Identity, Conversation, Agent, Finance), não na foundation.
- **D-12** (v3): **Pre-commit hooks locais** materializados via framework `pre-commit` em `.pre-commit-config.yaml`, instalados por `task setup`. Cobertura mínima: `gofmt`/`goimports`, `golangci-lint run --fast`, `ai-spec lint .`, `commit-msg` enforcement.
- **D-13** (v3): **Testes de integração com `testcontainers-go`** desde a foundation, validando `pkg/database` UoW + migration de exemplo contra Postgres ephemeral. Job dedicado no CI evita acoplamento com testes unitários.
- **D-14** (v4): **Taskfile (go-task) substitui Makefile** desde o primeiro commit. Adoção mandatória via skill `taskfile-production` instalada em `.agents/skills/taskfile-production/`. Sem Makefile no repositório.
- **D-15** (v4): **Layout isolado obrigatório** do Taskfile: `Taskfile.yml` na raiz (orquestrador fino) + `taskfiles/` por domínio (`build.yml`, `test.yml`, `lint.yml`, `security.yml`, `mocks.yml`, `ci.yml`) + `taskfiles/scripts/` para helpers + `.taskrc.yml` + `.env.example`. Scripts de automação **NÃO** podem viver em `cmd/`, `internal/` ou `pkg/`.
- **D-16** (v4): **`TASK_VERSION` pinada** em `v3.51.1` (referência atual da skill; revalidar via `python3 .agents/skills/taskfile-production/scripts/check-task-version.py --latest` antes de subir versão). Instalação local via `install-task.sh` da skill; instalação no CI via Action oficial `arduino/setup-task` com versão exata. Comportamento idêntico em macOS, Linux (Ubuntu) e Windows.
- **D-17** (v6): **`spf13/viper` v1.21.0** substitui `kelseyhightower/envconfig` como loader de configuração. Justificativa: Viper combina parsing de `.env` (`SetConfigType("env")`), env vars (`AutomaticEnv`) e key replacer (`SetEnvKeyReplacer(".", "_")`) num único pipeline; permite extensão futura para arquivos YAML/TOML sem refactor.
- **D-18** (v6): **Pasta `configs/` na raiz** (NÃO `internal/infrastructure/config/`) com `configs/config.go` único contendo struct `Config` agrupada por **tipo de variável** via `mapstructure:",squash"`. Cada grupo é um struct nomeado (`AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig` na foundation; novos grupos entram conforme PRDs subsequentes os introduzirem — sem RabbitMQ/Outbox/Consumer no MVP por fora-de-escopo). `Config.Validate()` roda no startup, gate fail-fast antes de qualquer subsistema; `.env` é obrigatório em dev e dispensado em prod Fly (env vars nativas).
- **D-19** (v7): **`spf13/cobra` v1.10.2 + binário único com subcomandos `server`/`worker`/`migrate`** (ADR-010), seguindo o pattern de `JailtonJunior94/financial/cmd/main.go` (referência mandatória). Substitui o approach anterior (`APP_MODE` env + `--migrate-only` flag); cada modo é um subcomando próprio com seu `Run()` em `cmd/<subcmd>/cmd.go`. Subcomandos adicionais (e.g. `consumer`, `admin`) entram em PRDs subsequentes conforme demanda, sem refatorar root.
- **D-20** (v7): **Image Postgres do testcontainers pinada em `postgres:16-alpine`** (major pin alinhado ao Fly Postgres dev tier 16.x; alpine reduz tempo de pull em CI ~80% vs imagem oficial; auto-patches dentro de 16.x desejáveis para CVE fixes).
- **D-21** (v7): **Defaults do `.env.example`** — `CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173` (origins comuns de dev React/Vite); `OTEL_TRACE_SAMPLE_RATE=1.0` (100% nas 2 primeiras semanas conforme discovery; reduzir para 0.2 via env override pós-estabilização sem rebuild); demais chaves com placeholders inseguros (`CHANGE_ME_*`) que o `Validate()` rejeita em `ENVIRONMENT=production`.
- **D-22** (v7): **`cmd/` excluído da cobertura de testes** (alinhado a D-11 e convenção Go); validação se dá via integration test do binário (`task test:integration` compila e executa `mecontrola server --help`, `mecontrola migrate`, etc.). `cmd/main.go` permanece como casca fina sem lógica de negócio testável — só registra cobra commands.
- **D-23** (v8): **Docker base image = `gcr.io/distroless/static-debian12:nonroot`** (ADR-011). Sem shell, sem package manager, ~2 MB, roda como não-root por default; binário Go estaticamente compilado (`CGO_ENABLED=0`); supply chain auditável via SBOM. Justificativa: maior aderência a R-SEC-001 (sem escrita acidental, sem subprocesso shell exploitável) com custo zero de debugging real (logs via OTel cobrem o caso de uso).
- **D-24** (v8): **`fly.toml` declara 2 processes**: `app = "mecontrola server"` (HTTP + scheduler placeholder) + `worker = "mecontrola worker"` (runtime worker placeholder). Trade-off de custo aceito (~R$ 60/mês vs R$ 25 com 1 process) para validar split de processes em produção real desde o foundation. Quando jobs reais entrarem (Epic 09), worker já está em prod sem rewire.
- **D-25** (v8): **Supply chain scanning = `govulncheck` + `trivy fs`** (ADR-012). `govulncheck` (oficial Go, call-graph, baixo falso positivo) cobre CVE em deps Go executáveis; `trivy fs` cobre Dockerfile, image layers, `go.sum`, secrets em commit. Ambos rodam em `task security:vulncheck` localmente e no CI. SBOM gerado por `trivy` anexado ao release.
- **D-26** (v8): **Dependabot nativo do GitHub** (`.github/dependabot.yml`) configurado desde o primeiro commit, com grupos por ecosystem (`gomod`, `github-actions`, `docker`); PRs auto-mergeable para `minor`/`patch` quando CI verde + revisão CODEOWNER aprovada; PRs `major` exigem revisão manual. Schedule semanal (terça 06:00 UTC).
- **D-27** (v9): **Registry da imagem = `ghcr.io/limateixeiratecnologia/mecontrola`** (ADR-013). Imagem assinada com **`cosign` keyless via OIDC** do GitHub Actions; **atestados SLSA** (provenance + SBOM SPDX) anexados via `cosign attest`; tag por commit SHA + tag semver. Sem credencial de registry adicional (OIDC nativo do GitHub).
- **D-28** (v9): **Reporting de cobertura = `fgrosse/go-coverage-report` action** posta tabela coverage por package como comentário do PR (ADR-015). Sem gate de cobertura (alinha com D-11); apenas visibilidade. Zero infra externa (sem Codecov SaaS — repo private estouraria orçamento).
- **D-29** (v9): **Versões de ferramentas de dev pinadas em duas camadas** (ADR-014): (a) `taskfiles/vars.yml` central com `GOLANGCI_LINT_VERSION`, `MOCKERY_VERSION`, `GOVULNCHECK_VERSION`, `TRIVY_VERSION`, `COSIGN_VERSION`, `MIGRATE_VERSION`, `PRE_COMMIT_VERSION` (binários CLI externos); (b) `tools.go` na raiz com `//go:build tools` e `import _ "github.com/<...>"` listando deps de teste/codegen instaláveis via `go install` (testify, mockery API). Dependabot atualiza ambos. Reprodutibilidade: build local idêntico ao CI.
- **D-30** (v9): **`SECURITY.md` + GPG signing obrigatório de commits e tags via Sigstore `gitsign`** (ADR-013). `SECURITY.md` declara política de disclosure (canal: email seguro + SLA 7 dias para resposta inicial); `gitsign` (keyless via OIDC) substitui GPG tradicional sem exigir chave local; branch protection na `main` exige `Require signed commits` ativado. Integra com Rekor para transparência pública (mesmo em repo private — apenas o hash é público).

### Pendências em aberto

Nenhuma pendência bloqueante em aberto nesta v2. Novas pendências surgidas durante a execução devem ser registradas nesta seção em revisões subsequentes do PRD com versão incrementada.

## Governança ai-spec Obrigatória

- O projeto **DEVE** iniciar e manter o fluxo SDD oficial: `create-prd → create-technical-specification → create-tasks → execute-task` (ou `execute-all-tasks` quando o lote for maduro e paralelo seguro). **Não** há rota alternativa: nenhuma mudança de comportamento, novo endpoint, nova flag CLI ou mudança de regra de negócio pode ser implementada sem PRD aprovado e techspec/tasks derivadas.
- O baseline de governança **DEVE** ser instalado com `ai-spec install` (binário `ai-spec` em versão alinhada à última release do orchestrator) e **DEVE** ser sincronizado em cada nova release via `ai-spec upgrade . --check` seguido de `ai-spec upgrade .` quando houver mudança pendente.
- Toda instalação ou sincronização **DEVE** terminar com `ai-spec inspect`, `ai-spec doctor` e `ai-spec lint` retornando `pass`. Sem `doctor` `pass`, a governança é considerada não-confiável e qualquer execução de skill fica bloqueada operacionalmente.
- O repositório **DEVE** suportar com paridade as três CLIs mandatórias: **Copilot CLI**, **Codex CLI** e **Claude Code CLI**. Adaptadores instalados em `.copilot/`, `.codex/` e `.claude/`, e manifestos `COPILOT.md`, `CODEX.md` e `CLAUDE.md` na raiz são gerados pelo `ai-spec install` e **DEVEM** permanecer versionados.
- O `spec-hash` em `tasks.md` (após `create-tasks`) **DEVE** ser mantido sincronizado via `ai-spec sync-spec-hash` ao editar o PRD ou a Especificação Técnica, evitando drift silencioso detectado pelo gate Stage 1 de `execute-task`.
- Telemetria de uso de skills via `GOVERNANCE_TELEMETRY=1` é **recomendada** para evolução do SDD com dados reais; opcional na foundation, obrigatória em PRDs futuros se assim definido.

## Bootstrap Inicial do Repositório

- O repositório **DEVE** ser criado em `https://github.com/LimaTeixeiraTecnologia/mecontrola` (organização `LimaTeixeiraTecnologia`; nome `mecontrola`; **visibilidade private** — D-06).
- O **primeiro commit** **DEVE** já conter o baseline completo de governança gerado por `ai-spec install . --tools claude,gemini,codex,copilot --langs go`: `.agents/skills/` (incluindo `taskfile-production`), `.claude/`, `.codex/`, `.copilot/`, `.gemini/`, `AGENTS.md`, `CLAUDE.md`, `CODEX.md`, `COPILOT.md`, `GEMINI.md`, `.ai_spec_harness.json` e `skills-lock.json`. Não é aceitável "instalar depois".
- O **primeiro commit** **DEVE** já conter o **Taskfile production-ready** completo (D-14 + D-15): `Taskfile.yml` na raiz, `taskfiles/{build,test,lint,security,mocks,ci}.yml`, `taskfiles/scripts/` com helpers cross-platform, `.taskrc.yml`, `.env.example`, `.task/` no `.gitignore` e schema comment em cada Taskfile. `validate-taskfile.py` DEVE retornar `SUCCESS` no commit inicial.
- O **primeiro commit** **DEVE** já conter `configs/config.go` (D-17 + D-18) com Viper + struct `Config` + `LoadConfig(path string) (*Config, error)` + `Config.Validate()` + `DBConfig.DSN()`/`SafeDSN()`, com testes table-driven cobrindo 100% dos validadores. `.env.example` na raiz DEVE documentar todas as chaves esperadas com valores placeholder seguros (e.g. `CHANGE_ME_USE_STRONG_PASSWORD`) — o `Validate()` rejeita esses placeholders quando `ENVIRONMENT=production`.
- O **primeiro commit** **DEVE** já conter `Dockerfile` multi-stage com builder `golang:1.26.3-alpine` (CGO_ENABLED=0, ldflags `-s -w`) e runtime `gcr.io/distroless/static-debian12:nonroot` (D-23 + ADR-011); `fly.toml` com 2 processes `app=mecontrola server` + `worker=mecontrola worker` (D-24); `.github/dependabot.yml` com grupos `gomod`/`github-actions`/`docker` (D-26).
- O **primeiro commit** **DEVE** já conter `SECURITY.md` na raiz declarando política de disclosure (D-30 + ADR-013), `tools.go` na raiz com `//go:build tools` listando deps Go de tooling (D-29 + ADR-014), `taskfiles/vars.yml` com versões pinadas dos binários CLI externos (D-29). Branch protection na `main` DEVE exigir `Require signed commits` ativado antes do primeiro PR (D-30).
- O repositório **DEVE** conter `CODEOWNERS` na raiz com `* @JailtonJunior94` como owner obrigatório global desde o primeiro commit.
- A branch principal **DEVE** ser `main`, com **branch protection** no GitHub configurada conforme **D-07**: review aprovado do CODEOWNER + status checks obrigatórios do CI (test/lint/build/`ai-spec doctor`/`ai-spec lint`) + **linear history** (sem merge commits) + **squash-merge** como única estratégia de merge — configuração executada no GitHub após criação do repo remoto, registrada como ação operacional neste PRD.
- Política de tag/release **DEVE** seguir **D-05**: `v0.1.0` no primeiro deploy de produção; pré-releases marcados como `v0.0.x`; geração de tags via `ai-spec semver-next` a partir de conventional commits (skill `semantic-commit`); changelog gerado por `ai-spec changelog`.
- O repositório **DEVE** conter um `README.md` mínimo descrevendo a stack (Go 1.26.3, devkit-go v0.4.0, Fly.io `gru`, Grafana Cloud free tier), comandos `task` principais (`task setup`, `task build`, `task test:unit`, `task ci`), mandato SDD e link para este PRD.
- O repositório **DEVE** conter `.gitignore` adequado ao Go + Fly + `.env` local.
- O repositório **DEVE** conter `.editorconfig` mínimo.
- O repositório **NÃO** conterá `LICENSE` no MVP (D-04: proprietary private); decisão a revisitar antes de qualquer mudança de visibilidade.

## Mandatórios de Tooling e Operação

- **Linguagem**: Go **1.26.3** (D-01), fixada no `go.mod`.
- **Foundation técnica**: `devkit-go` v0.4.0, sem substituição.
- **Persistência**: Postgres único (Fly Postgres managed com PITR em produção; Docker local em dev).
- **PaaS**: Fly.io região `gru`. Sem AWS/GCP/Azure no MVP.
- **Observabilidade**: OpenTelemetry OTLP gRPC com redaction obrigatória, exportando para **Grafana Cloud free tier** (D-02: Tempo + Loki + Mimir).
- **Governança SDD**: `ai-spec` obrigatório; toda mudança nasce em `create-prd`.
- **CLIs operacionais obrigatórias**:
  - `ai-spec` (binário `ai-spec-harness`) — versão alinhada à última release.
  - `gh` (GitHub CLI) — autenticado para a org `LimaTeixeiraTecnologia`.
  - `claude` (Claude Code CLI) — paridade total com Codex/Copilot.
  - `codex` (Codex CLI) — paridade total.
  - `copilot` (GitHub Copilot CLI) — paridade total.
  - `fly` (flyctl) — autenticado para a região `gru`.
  - `go`, `docker`, `make` — toolchain local.
- **CI/CD**: GitHub Actions; setup do `ai-spec` via Action oficial `JailtonJunior94/orchestrator/.github/actions/setup-ai-spec@setup-action-v1`; setup do Task via `arduino/setup-task` pinando `TASK_VERSION=v3.51.1` (D-16); CI obrigatório consumindo o Taskfile (`task ci` orquestrando: test unitário + integração com testcontainers + lint + build + security:vulncheck + `ai-spec doctor` + `ai-spec lint` + check de conventional commits + validate-taskfile); CD para Fly região `gru` em push aprovado para `main`.
- **Padrões de código**: `golangci-lint` com `depguard` enforced para fronteiras hexagonais; `gofmt`/`goimports` obrigatórios; **conventional commits obrigatórios** (D-10) via `commit-msg` hook + check no CI, alimentando `ai-spec changelog` e `ai-spec semver-next` sem brecha.
- **Pre-commit framework**: `pre-commit` (D-12) com configuração em `.pre-commit-config.yaml`; instalação via `task setup`.
- **Testes**: testes unitários com `go test`; testes de integração com `testcontainers-go` (D-13) em job dedicado no CI; sem gate de cobertura no MVP (D-11), só relatório anexável.
- **Segurança operacional**: secrets em Fly secrets em produção; `.env` local não-commitado; rotação documentada em runbook; sem segredos em logs (redaction enforced pelo `pkg/observability`).
- **Rollback**: `fly releases rollback` ou `fly deploy --image <previous>`; migrations DEVEM ser backward-compatible por uma versão para permitir rollback sem reescrita de dados (regra a manter a partir do primeiro módulo de negócio).
