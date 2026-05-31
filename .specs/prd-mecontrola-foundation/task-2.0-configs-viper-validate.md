# Tarefa 2.0: `configs/` — Viper + grupos + `Validate()` + `DSN()`/`SafeDSN()` + `.env.example`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar a pasta **`configs/`** na raiz com `config.go` único contendo struct `Config` agrupada por **tipo de variável** via `mapstructure:",squash"` (`AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig`), `LoadConfig(path)` consumindo `.env` (obrigatório em dev) + env vars (Fly prod), `Validate()` fail-fast antes de qualquer subsistema, `DBConfig.DSN()`/`SafeDSN()`, e `configs/insecure.go` com lista de placeholders inseguros conhecidos. Cobre **RF-04** integralmente.

<requirements>
- Lib: `github.com/spf13/viper@v1.21.0` pinada em `go.mod` (D-17).
- Pasta `configs/` na raiz (NÃO `internal/infrastructure/config/` — D-18, ADR-009).
- Struct `Config` com 4 grupos squash: `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig`.
- `LoadConfig(path string) (*Config, error)` segue pipeline: `SetConfigName(".env")` + `SetConfigType("env")` + `AutomaticEnv()` + `SetEnvKeyReplacer(".", "_")` + `ReadInConfig` (tolera `ConfigFileNotFoundError` apenas em `ENVIRONMENT=production`) + `Unmarshal` + `Validate()`.
- `Validate()` rejeita: senha DB <16 chars em prod, secret keys <64 chars em prod, placeholders `CHANGE_ME_*` em prod, `Port` fora [1..65535], `TraceSampleRate` fora [0..1], `Environment` ∉ {local, staging, production}.
- `DBConfig.DSN()` retorna `postgres://user:pwd@host:port/name?sslmode=...`; `SafeDSN()` retorna o mesmo com senha mascarada `***` — único permitido em logs.
- `.env.example` na raiz com TODAS as chaves + defaults conforme D-21: `CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173`, `OTEL_TRACE_SAMPLE_RATE=1.0`, `ENVIRONMENT=local`, demais placeholders `CHANGE_ME_*`.
- `configs/insecure.go` lista placeholders inseguros conhecidos.
- Teste table-driven cobrindo 100% dos cenários do `Validate()` + teste que `SafeDSN()` nunca contém o valor de `Password`.
</requirements>

## Subtarefas

- [ ] 2.1 `go get github.com/spf13/viper@v1.21.0`; atualizar `tools.go` se necessário.
- [ ] 2.2 Criar `configs/config.go` com tipos `Config`, `AppConfig`, `HTTPConfig`, `DBConfig`, `O11yConfig` (tags `mapstructure`).
- [ ] 2.3 Implementar `LoadConfig(path string) (*Config, error)` com pipeline Viper documentado.
- [ ] 2.4 Implementar `Config.Validate()` com cenários listados em RF-04 (ADR-009).
- [ ] 2.5 Implementar `DBConfig.DSN()` + `DBConfig.SafeDSN()`; documentar com comentário que `DSN()` é proibido em logs.
- [ ] 2.6 Criar `configs/insecure.go` com `[]string{"CHANGE_ME_USE_STRONG_PASSWORD", "CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS", "your_secret_key", "financial@password"}`.
- [ ] 2.7 Criar `.env.example` na raiz com TODAS as chaves e defaults conforme D-21.
- [ ] 2.8 Criar `configs/config_test.go` com testes table-driven cobrindo todos os branches de `Validate()`, `LoadConfig` (com fixture `.env`), `DSN()` vs `SafeDSN()` (assert que valor de Password não aparece em SafeDSN).
- [ ] 2.9 Configurar `forbidigo` em `.golangci.yml` (criado nesta task ou em 1.0) bloqueando uso de `.DSN()` fora de `configs/` e `internal/infrastructure/database/`.

## Detalhes de Implementação

Ver techspec §"Interfaces Chave" (esboço de assinatura) + ADR-009 §Decisão (pipeline Viper, grupos, Validate). Não duplicar — referenciar.

## Critérios de Sucesso

- `go build ./configs/...` compila.
- `go test ./configs/... -coverprofile=cov.out && go tool cover -func cov.out` mostra 100% nas funções do `Validate()`.
- Teste `TestSafeDSN_NeverContainsPassword` verde com 5 senhas distintas geradas randomicamente.
- `LoadConfig("./testdata/valid")` retorna `*Config` sem erro com `.env` válido.
- `LoadConfig(".")` em dev sem `.env` retorna erro explícito; em prod sem `.env` retorna `*Config` lendo env vars.
- Cobre RF-04 integralmente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: `configs/config_test.go` cobre `Validate()` (≥10 cenários table-driven), `LoadConfig` (3 cenários: dev sem .env → erro; dev com .env válido → ok; prod sem .env → ok via env vars), `DSN`/`SafeDSN` (5 senhas randomicas validando ausência em SafeDSN).
- [ ] Testes de integração: n/a (sem dependência externa).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `configs/config.go`
- `configs/insecure.go`
- `configs/config_test.go`
- `configs/testdata/valid/.env`
- `configs/testdata/insecure-prod/.env`
- `.env.example` (raiz)
- `go.mod`, `go.sum`
- `.golangci.yml` (atualização de `forbidigo` para `.DSN()`)
