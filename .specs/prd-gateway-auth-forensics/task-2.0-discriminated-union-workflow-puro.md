# Tarefa 2.0: Discriminated union GatewayAuthResult + workflow puro VerifyGatewayRequest

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa o coração DMMF do gateway: discriminated union `GatewayAuthResult` (5 variantes sealed) + workflow puro `VerifyGatewayRequest` em `internal/identity/domain/services/`. Função totalmente determinística, sem IO, sem context, sem mocks no teste. Consome os VOs da tarefa 1.0.

<requirements>
- RF-04: canonical message `strings.ToLower(userIDRaw) + "." + timestampRaw` conforme ADR-001
- RF-05: aceita `Current` ou `Next` secrets; distingue resultado (`Valid` vs `Rotated`)
- RF-07: HMAC verificado com `hmac.Equal` (constant-time, inegociável)
- RF-11: DU com 5 variantes (`Valid`, `Rotated`, `InvalidSignature`, `StaleTimestamp`, `MissingHeader`); workflow puro
- RF-19, RF-20: skills + gates verdes
- Vetor de teste fixo cross-lang (input → hex esperado) reproduzível em Python
- Função pura: nenhuma chamada a `time.Now()`, `uuid.New()`, IO, logger
- Switch sobre `Kind` no consumidor deve ser exaustivo (preparar para `golangci-lint` exhaustive na tarefa 6.0)
</requirements>

## Subtarefas

- [ ] 2.1 Criar `internal/identity/domain/services/gateway_auth_result.go` com enum `GatewayAuthResultKind uint8` iniciado em 1 (R5.8) e struct `GatewayAuthResult{ Kind GatewayAuthResultKind }`. Método `IsAuthorized() bool` retornando true para `Valid|Rotated`.
- [ ] 2.2 Criar `internal/identity/domain/services/verify_gateway_request.go` com types `VerifyRequest{UserIDRaw, SignatureRaw, TimestampRaw string}`, `SecretPair{Current, Next []byte}`, função `VerifyGatewayRequest(req VerifyRequest, secrets SecretPair, now time.Time, window time.Duration) GatewayAuthResult`.
- [ ] 2.3 Sequência de verificação (ordem importa): missing → timestamp → signature parse → HMAC current → HMAC next → invalid.
- [ ] 2.4 Helper `canonical(userIDRaw, timestampRaw string) []byte` que retorna `[]byte` do canonical em uma alocação só (R5.20 + R5.36).
- [ ] 2.5 Teste table-driven em `verify_gateway_request_test.go` cobrindo 12 casos da matriz na techspec + vetor fixo reproduzível (input fixo → hex fixo).
- [ ] 2.6 Cobertura ≥ 95% no pacote `domain/services/` (apenas para os arquivos novos).

## Detalhes de Implementação

Ver techspec seção "Design de Implementação > Workflow puro". ADR-001 cravou canonical (sem path, sem body hash, sem method). Use a constante `_canonicalSeparator = "."`.

Para o vetor fixo no runbook (tarefa 8.0), gerar tabela inicial aqui:
```
user_id  = "00000000-0000-0000-0000-000000000000"
timestamp = "1700000000"
secret   = bytes("test-secret-32-bytes-padding-aaaa")
hex      = <gerar e fixar no teste>
```

## Critérios de Sucesso

- `go test ./internal/identity/domain/services/... -run "VerifyGatewayRequest" -v -cover` cobertura ≥ 95%, sem mocks.
- Vetor fixo do teste passa sem nenhum cálculo dinâmico (todos os bytes pré-computados).
- `grep` R-ADAPTER-001.1 sobre os arquivos novos retorna vazio.
- Função `VerifyGatewayRequest` não importa nenhum dos: `context`, `database/sql`, `log/slog`, `net/http`, qualquer infra. Validado por inspeção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário table-driven com 12+ casos da matriz da techspec
- [ ] Teste do vetor fixo (input pré-determinado → hex pré-computado)
- [ ] Teste de exaustividade: cada variante de `Kind` é retornada por ao menos um caso

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/domain/services/gateway_auth_result.go` (novo)
- `internal/identity/domain/services/verify_gateway_request.go` (novo)
- `internal/identity/domain/services/verify_gateway_request_test.go` (novo)
