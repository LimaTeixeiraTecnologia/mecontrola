# Tarefa 4.0: Forensics extractors RequestID/ClientIP + EstablishPrincipal update

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa value objects `RequestID` e `ClientIP` com smart constructors DMMF (sanitização inline, sem `Clock`, sem abstrações desnecessárias) e atualiza o use case `EstablishPrincipal` (e variantes Failed/UnknownUser) para aceitar e persistir os campos forensics. Sanitização do `X-Forwarded-For` segue ADR-008 (último IP da lista, validado com `net.ParseIP`).

<requirements>
- RF-16: use case `EstablishPrincipal` recebe `RequestID` + `ClientIP` por argumento; persistência via repo
- RF-17: sanitização XFF retorna **último elemento** validado; NULL aceito quando ausente ou inválido
- RF-18: novos campos em `slog.Attr` estruturados (`slog.String("request_id", ...)` etc.); nunca concat de mensagem
- DMMF: smart constructors `NewRequestID(raw string)`, `NewClientIP(xForwardedFor string)` validam invariante
- Zero comentário em `.go` produção
- Sem nova dep em `go.mod` (`net` para `net.ParseIP` já é stdlib)
</requirements>

## Subtarefas

- [ ] 4.1 Criar `internal/identity/domain/valueobjects/request_id.go` com `RequestID struct{ raw string }`, `NewRequestID(raw string) (RequestID, error)`, `String() string`, `IsZero() bool`. Invariante: non-empty após trim; max-len configurável (default 128 chars para evitar abuso).
- [ ] 4.2 Criar `internal/identity/domain/valueobjects/client_ip.go` com `ClientIP struct{ ip net.IP }`, `NewClientIP(xForwardedFor string) (ClientIP, error)`, `String() string`, `IsZero() bool`. Invariante: split por vírgula, trim, último elemento, `net.ParseIP` non-nil. Erro se parse falhar; vazio aceito como `ClientIP{}` (não erro).
- [ ] 4.3 Atualizar DTO de input `internal/identity/application/dtos/input/establish_principal.go` (ou equivalente) com campos `RequestID string`, `ClientIPRaw string`.
- [ ] 4.4 Atualizar use case `internal/identity/application/usecases/establish_principal.go` para consumir os campos novos, construir VOs e propagar para repo + outbox.
- [ ] 4.5 Repetir 4.4 para variantes `RecordAuthFailed`, `RecordUnknownUser` se existirem como use cases separados.
- [ ] 4.6 Testes table-driven cobrindo `NewClientIP`: vazio → `IsZero`; `"1.2.3.4"` → IP válido; `"evil, 1.2.3.4"` → `1.2.3.4`; `"evil"` → erro; `"1.2.3.4, ::1"` → `::1`; `"1.2.3.4,1.2.3.5,1.2.3.6"` → `1.2.3.6`. Cobertura ≥ 95%.
- [ ] 4.7 Testes do use case `EstablishPrincipal` cobrindo: input com request_id + client_ip → persistido; input com client_ip vazio → NULL na linha (`IsZero`).

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados" e ADR-008. Reuso do contrato outbox do `prd-auth-foundation` para publicar `auth.principal_established` com os novos campos (consumer `ProjectAuthEvent` já mapeia; valida sua extensão na tarefa 5.0).

## Critérios de Sucesso

- `go test ./internal/identity/domain/valueobjects/... -run "RequestID|ClientIP" -cover` ≥ 95%.
- `go test ./internal/identity/application/usecases/... -run "EstablishPrincipal" -v` PASS para os novos cenários.
- `grep` R-ADAPTER-001.1 sobre arquivos novos retorna vazio.
- Sanitização XFF passa todos os 6 cenários da tabela.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste unitário table-driven dos 2 VOs
- [ ] Teste do use case EstablishPrincipal com novos campos (mock do repo)
- [ ] Teste de regressão: input sem request_id/client_ip continua funcionando (compat com chamadores que ainda não enviam)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/domain/valueobjects/request_id.go` (novo)
- `internal/identity/domain/valueobjects/request_id_test.go` (novo)
- `internal/identity/domain/valueobjects/client_ip.go` (novo)
- `internal/identity/domain/valueobjects/client_ip_test.go` (novo)
- `internal/identity/application/dtos/input/establish_principal.go` (modificado)
- `internal/identity/application/usecases/establish_principal.go` (modificado)
- `internal/identity/application/usecases/establish_principal_test.go` (modificado)
