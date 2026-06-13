# Tarefa 1.0: Config + smart constructors GatewaySignature/GatewayTimestamp

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Cria os fundamentos de tipo do gateway HMAC: campos de configuração para `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT/NEXT` com validação em `production`, e dois value objects DMMF (`GatewaySignature`, `GatewayTimestamp`) com smart constructors que enforçam invariante na construção, prevenindo entrada inválida de chegar ao domínio.

<requirements>
- RF-06: validação non-empty + entropia ≥ 32 bytes em `production` (Config.Validate)
- RF-11 (parcial): smart constructors `NewGatewaySignature(hex string)` e `NewGatewayTimestamp(raw string, now time.Time, window time.Duration)` em `internal/identity/domain/valueobjects/`
- RF-19: skill `go-implementation` carregada (R0–R7, R-ADAPTER-001)
- RF-20: `task lint && task test && task vulncheck` verde
- RF-23: zero nova dependência em `go.mod` — apenas `crypto/hmac`, `crypto/sha256`, `encoding/hex`, `strconv`, `time`
- DMMF: invariante validada **no construtor**, não em middleware
- Zero comentário em `.go` de produção (R-ADAPTER-001.1)
- Sem abstração de tempo: `now` recebido por argumento; não introduzir `Clock` interface (memória `feedback_no_time_abstraction`)
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar campos `GatewaySharedSecretCurrent []byte`, `GatewaySharedSecretNext []byte`, `GatewayAuthWindow time.Duration` em `IdentityConfig` (`configs/config.go`).
- [ ] 1.2 Atualizar `Config.Validate()`: em `Environment="production"`, `GatewaySharedSecretCurrent` não-vazio e `len >= 32`. `NEXT` opcional. `Window` default `60 * time.Second`.
- [ ] 1.3 Criar `internal/identity/domain/valueobjects/gateway_signature.go` com `GatewaySignature struct{ raw []byte }`, `NewGatewaySignature(hex string) (GatewaySignature, error)`, `Bytes() []byte`, `IsZero() bool`. Invariante: hex lowercase, 64 chars, charset `[0-9a-f]`.
- [ ] 1.4 Criar `internal/identity/domain/valueobjects/gateway_timestamp.go` com `GatewayTimestamp struct{ at time.Time }`, `NewGatewayTimestamp(raw string, now time.Time, window time.Duration) (GatewayTimestamp, error)`, `Time() time.Time`, `Raw() string`. Invariante: unix seconds parseable, `|now - parsed| <= window`.
- [ ] 1.5 Testes table-driven cobrindo ≥ 95% de cada VO + happy/sad paths do `Config.Validate`.

## Detalhes de Implementação

Ver techspec seção "Design de Implementação > Interfaces Chave" e "Modelos de Dados > Config". Adapter middleware (tarefa 6.0) **consumirá** estes VOs sem reimplementar invariante.

ADRs aplicáveis: ADR-001 (canonical), ADR-002 (rotação `current/next`). Vetor de teste fixo é gerado em tarefa 2.0 e referenciado aqui.

## Critérios de Sucesso

- `go test ./internal/identity/domain/valueobjects/... -run "Gateway" -v` PASS com tabela cobrindo todos os casos de falha.
- `go test ./configs/... -v` PASS para Validate em `production` (ambos os campos requeridos), `development` (sem requisitos), entropia insuficiente (< 32 bytes → erro).
- `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" internal/identity/domain/valueobjects/gateway_*.go configs/config.go | grep -Ev "(//go:|//nolint:|// Code generated)"` retorna vazio.
- `go vet ./...` PASS. `task lint` PASS. `task vulncheck` PASS.
- Diff de `go.mod`: zero adições.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários table-driven dos dois VOs cobrindo ≥ 95% de linhas
- [ ] Testes unitários do `Config.Validate` para os 3 cenários (production OK, production faltando secret, production com entropia < 32)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go` (modificado)
- `configs/config_test.go` (modificado)
- `internal/identity/domain/valueobjects/gateway_signature.go` (novo)
- `internal/identity/domain/valueobjects/gateway_signature_test.go` (novo)
- `internal/identity/domain/valueobjects/gateway_timestamp.go` (novo)
- `internal/identity/domain/valueobjects/gateway_timestamp_test.go` (novo)
- `.env.example` (modificado, documentar envs novas)
