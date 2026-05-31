# ADR-009 — `spf13/viper` v1.21.0 + pasta `configs/` + `Validate()` fail-fast + `DSN/SafeDSN`

## Metadados

- **Título:** Adoção do Viper v1.21.0, pasta `configs/` na raiz com `Config` agrupado por tipo de variável e gate `Validate()` no startup
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-04, §D-17, §D-18, §CS-18..CS-20](./prd.md), [techspec §Interfaces Chave, §Estratégia de Erros](./techspec.md), [R-SEC-001](../../.agents/skills/agent-governance/references/security.md), [R-DDD-001 §Value Objects](../../.agents/skills/agent-governance/references/ddd.md), [Viper v1.21.0](https://github.com/spf13/viper/releases/tag/v1.21.0)

## Contexto

A versão v4 da techspec adotava `kelseyhightower/envconfig` com tags `validate` para carregamento e validação de configuração. Durante a revisão, o tech lead solicitou substituição por **`spf13/viper`** (v1.21.0 — última estável publicada em 2025-09-08), seguindo um pattern já consolidado em outros projetos da org: pasta `configs/` na raiz com `config.go` único, struct `Config` composta por grupos via `mapstructure:",squash"` (por **tipo de variável** — `DBConfig`, `HTTPConfig`, `O11yConfig`, etc.), `LoadConfig(path)` consumindo `.env` em dev e env vars nativas em produção, e `Validate()` rodando como gate fail-fast antes de qualquer subsistema inicializar.

Requisitos:
- `.env` é **mandatório** para startup local (warmup do projeto); ausência aborta com erro explícito.
- Em produção Fly, secrets vêm de env vars (`fly secrets set ...`) e `.env` é dispensado.
- `Validate()` deve rejeitar defaults inseguros conhecidos (`CHANGE_ME_*`, `guest:guest`, secrets curtos) em produção.
- `DBConfig` deve expor `DSN()` (com senha, uso interno) e `SafeDSN()` (senha mascarada, único formato permitido em logs).

## Decisão

- **Lib**: `github.com/spf13/viper` **v1.21.0** (pinada no `go.mod`).
- **Layout**: pasta `configs/` na raiz do projeto (NÃO `internal/infrastructure/config/`), com `configs/config.go` + `configs/config_test.go`. Convenção "config as data": configuração não é "infra interna", é contrato público de operação.
- **Struct `Config`** composta por grupos via `mapstructure:",squash"`. Foundation inclui apenas grupos em escopo do MVP:
  - `AppConfig` (`ENVIRONMENT`, `APP_MODE`)
  - `HTTPConfig` (`HTTP_PORT`, `SERVICE_NAME_API`, `CORS_ALLOWED_ORIGINS`)
  - `DBConfig` (`DB_DRIVER`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`, `DB_MAX_IDLE_CONNS`, `DB_MAX_OPEN_CONNS`, `DB_CONN_MAX_LIFE_TIME_MINUTES`, `DB_CONN_MAX_IDLE_TIME_MINUTES`)
  - `O11yConfig` (`OTEL_SERVICE_VERSION`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_EXPORTER_OTLP_INSECURE`, `OTEL_TRACE_SAMPLE_RATE`, `LOG_LEVEL`, `LOG_FORMAT`)
- **Grupos fora do escopo da foundation** (entram em PRDs subsequentes): `AuthConfig` (Identity), `RabbitMQConfig`/`ConsumerConfig`/`OutboxConfig` (não há broker no MVP — só entram se um PRD futuro o introduzir; PRD atual proíbe), `WorkerConfig` (foundation pode incluir placeholder mínimo se necessário).
- **`LoadConfig(path string) (*Config, error)`** segue o pipeline canônico:
  1. `viper.AddConfigPath(path)` + `SetConfigName(".env")` + `SetConfigType("env")`.
  2. `viper.AutomaticEnv()` + `SetEnvKeyReplacer(strings.NewReplacer(".", "_"))`.
  3. `viper.ReadInConfig()`: em **dev local** falha aborta startup com erro explícito; em **prod** (`ENVIRONMENT=production` e arquivo ausente) o erro `viper.ConfigFileNotFoundError` é tolerado e o pipeline segue com env vars puras.
  4. `viper.Unmarshal(&config)`.
  5. `config.Validate()`: gate fail-fast — retorna erro composto se invariantes forem violadas.
- **`Validate()`** roda sempre (não só em produção). Em produção, escalations adicionais:
  - `DBConfig.Password` ≥16 chars; não em allowlist de inseguros (`CHANGE_ME_USE_STRONG_PASSWORD`, defaults documentados).
  - `AuthConfig.AuthSecretKey` ≥64 chars (quando PRD de Identity introduzir esse grupo — placeholder no foundation).
  - URLs OTLP/DB sem credenciais default conhecidas.
  - `AuthTokenDuration` ∈ [1, 24] horas (escopo Identity; foundation ignora se grupo ausente).
  - `Environment` ∈ {`local`, `staging`, `production`}.
  - Ranges: `HTTPConfig.Port` ∈ [1, 65535]; pool tunables > 0; `O11yConfig.TraceSampleRate` ∈ [0, 1].
- **`DBConfig.DSN()`** retorna `postgres://user:password@host:port/name?sslmode=<...>` — para uso INTERNO (driver pgx). **Proibido em logs** (enforce em review + lint custom).
- **`DBConfig.SafeDSN()`** retorna `postgres://user:***@host:port/name?sslmode=<...>` — **único formato permitido em logs e mensagens de erro**.

## Alternativas Consideradas

1. **Manter `kelseyhightower/envconfig` em `internal/infrastructure/config/`** (versão v4).
   - Vantagens: lib mínima, tags simples, zero deps externas além do parser.
   - Desvantagens: não casa com convenção da org; sem fallback a outros formatos (JSON/YAML); validação `validate` tag tem ergonomia inferior ao validator e ao Validate() manual.
2. **`spf13/viper` + estrutura plana (sem grupos)**.
   - Vantagens: menos boilerplate.
   - Desvantagens: viola pedido explícito ("separando por tipo de variável"); Config gigante mistura responsabilidades; viola Object Calisthenics #7 (entidades pequenas).
3. **`spf13/viper` em `internal/infrastructure/config/`**.
   - Vantagens: alinha com camada de infraestrutura.
   - Desvantagens: viola pedido explícito de pasta `configs/`; "config" como pasta raiz é convenção amplamente reconhecida em Go (`golang-standards/project-layout`).

## Consequências

### Benefícios Esperados

- Pattern repetível entre projetos da org (mesma estrutura de `configs/config.go` + grupos squash).
- `Validate()` como gate fail-fast elimina classe inteira de incidentes ("subiu em prod com password default").
- `SafeDSN()` como contrato torna impossível vazar senha em log por acidente (linter custom pode bloquear `.DSN()` fora de pacotes permitidos).
- Viper permite evolução para YAML/TOML/JSON ou config remoto (Consul/etcd) sem refactor.
- Grupos por tipo (`DBConfig`, `HTTPConfig`, ...) honram Object Calisthenics #3 + #7 (encapsular primitivos relacionados; entidades pequenas e coesas).

### Trade-offs e Custos

- Viper é heavier que envconfig (~200 KB no binário; deps transitivas).
- Pasta `configs/` na raiz "fora" do `internal/` é incomum em Go; precisa de README explicando.
- `LoadConfig(path)` precisa de path "." no startup — pequena fricção em testes (passar `"./testdata"` ou similar).

### Riscos e Mitigações

- **Risco:** dev loga `cfg.DBConfig.DSN()` por engano e expõe senha.
  - **Mitigação:** lint custom (`golangci-lint` com regra `forbidigo` apontando para `.DSN()` fora de `configs/` e `internal/infrastructure/database/`); review obrigatório.
- **Risco:** Viper consome `.env` mas dev tem variável de ambiente conflitante; debug confuso.
  - **Mitigação:** documentar precedência (env var > .env) no README; `Validate()` loga em `debug` qual chave veio de onde.
- **Risco:** `Validate()` rejeita `CHANGE_ME_*` em prod, mas dev usou outro placeholder não previsto.
  - **Mitigação:** lista de placeholders inseguros centralizada em `configs/insecure.go`; PR review deve adicionar novos placeholders à lista.
- **Risco:** Pasta `configs/` virar "junk drawer".
  - **Mitigação:** convenção: apenas `config.go`, `config_test.go`, `insecure.go` e (se necessário) `loader.go`; novo arquivo exige ADR.

## Plano de Implementação

1. `go get github.com/spf13/viper@v1.21.0`.
2. Criar `configs/config.go`:
   - Tipos: `Config`, `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig` com tags `mapstructure`.
   - `LoadConfig(path string) (*Config, error)` seguindo o pipeline canônico.
   - `Validate()` com cenários: senha, secret, credenciais default, ranges, environment.
   - `DBConfig.DSN()` + `DBConfig.SafeDSN()`.
   - `configs/insecure.go` com lista de placeholders inseguros conhecidos.
3. Criar `configs/config_test.go` com 100% de cobertura de `Validate()` (table-driven), `LoadConfig` (com testdata `.env` válido), e teste explícito que `SafeDSN()` nunca contém o valor de `Password`.
4. Criar `.env.example` na raiz com TODAS as chaves esperadas + placeholders seguros (`CHANGE_ME_USE_STRONG_PASSWORD`, `localhost`, `5432`, etc.).
5. Atualizar `cmd/server/main.go` para chamar `configs.LoadConfig(".")` antes do `runtime.Bootstrap`.
6. Atualizar `taskfiles/test.yml` `task test:unit` para incluir `configs/...`.
7. Lint custom (`forbidigo` em `.golangci.yml`) bloqueando `.DSN()` fora de `configs/` e `internal/infrastructure/database/`.
8. Documentar em `README.md` seção "Primeira execução local": `cp .env.example .env && task setup && task run`.

## Monitoramento e Validação

- Métrica `config_validation_failed_total{reason}` (counter) — incrementado em cada motivo de falha do `Validate()`.
- Log em `debug` mostrando quais chaves vieram de `.env` vs env var (sem valores).
- Alerta sintético em CI/CD: deploy em staging com `.env` propositalmente inseguro DEVE falhar.

## Impacto em Documentação e Operação

- `README.md`: seção "Configuração" e "Primeira execução local".
- Runbook "Rotacionar senha do DB": passos no Fly secrets + redeploy (sem ssh manual em `.env`).
- Runbook "Adicionar nova variável de configuração": criar grupo se for novo domínio; adicionar tag `mapstructure`; atualizar `.env.example`; atualizar `Validate()` se houver invariante; atualizar testes.
- Onboarding: incluir esta ADR como leitura obrigatória junto com ADR-001 e ADR-004.

## Revisão Futura

- Revisitar para suportar JSON/YAML/remote config (Consul/etcd) quando houver demanda real (não no MVP).
- Revisitar se a quantidade de grupos cruzar 8 — sinal de erosão arquitetural.
- Revisitar quando primeira variável precisar de hot-reload (Viper suporta; foundation não usa).
