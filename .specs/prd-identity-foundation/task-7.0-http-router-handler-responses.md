# Tarefa 7.0: HTTP — router placeholder + handler com devkit-go/pkg/responses

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o router placeholder `UserRouter` em `infrastructure/http/server/router.go` (item 4 do Padrão Obrigatório de Módulo — slot existe mesmo sem rotas reais no MVP) e o `UpsertUserByWhatsAppHandler` consumindo `github.com/JailtonJunior94/devkit-go/pkg/responses` (proibido reimplementar `writeJSON`/`writeError` locais — R6.10). Handler converte JSON → VOs antes de chamar o UC (RF-04 no boundary HTTP) e mapeia sentinels para HTTP status com códigos semânticos em `details` (`map[string]string{"code": "..."}`).

<requirements>
- RF-04: handler decodifica JSON cru, valida via `valueobjects.NewWhatsAppNumber`/`NewEmail` antes de entregar ao UC. UC nunca recebe `string` cru.
- RF-14: logs do handler usam PII mascarada (VO.Masked() + pii.MaskDisplayName).
- R6.10: usar `responses.JSON`, `responses.Error`, `responses.ErrorWithDetails` da devkit. Proibido `w.Header().Set`/`json.NewEncoder(w).Encode` direto no handler.
- ADR-005 (item 4 do Padrão): router implementa `chi_server.Router{ Register(chi.Router) }` mesmo que vazio no MVP.
- Mapeamento de erros: `ErrWhatsAppNumberInUse` → `409 Conflict` com `code: "whatsapp_in_use"`; `ErrEmailInUse` → `409 Conflict` com `code: "email_in_use"`; erro de VO → `400 Bad Request` com `code: "invalid_whatsapp"`/`"invalid_email"`; demais → `500 Internal Server Error` com `code` ausente + log estruturado.
- Tracer span `identity.handler.upsert_user_by_whatsapp`.
</requirements>

## Subtarefas

- [ ] 7.1 `internal/identity/infrastructure/http/server/router.go`:
  - Struct `UserRouter` com campo `upsertHandler *UpsertUserByWhatsAppHandler` (pode estar nil — sem rota registrada).
  - `NewUserRouter(upsert *UpsertUserByWhatsAppHandler) *UserRouter`.
  - Método `Register(r chi.Router)` — no MVP, registra `POST /api/v1/identity/users` apenas se `upsertHandler != nil`.
- [ ] 7.2 `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go`:
  - Struct + construtor (recebe UC + `o11y`).
  - DTOs locais `upsertUserRequest` (JSON) e `upsertUserResponse` (JSON).
  - `Handle(w, r)` com: tracer span, decode JSON, conversão para VOs, chamada `h.usecase.Execute(...)`, mapeamento de erros para `responses.Error`/`responses.ErrorWithDetails`, retorno via `responses.JSON(...)`.
- [ ] 7.3 Testes com `net/http/httptest` cobrindo: payload válido (200), JSON malformado (400 `invalid_payload` via `responses.Error`), whatsapp inválido (400 `invalid_whatsapp`), email inválido (400 `invalid_email`), `ErrWhatsAppNumberInUse` (409), `ErrEmailInUse` (409), erro interno (500 `internal_error`).
- [ ] 7.4 `internal/identity/infrastructure/http/client/doc.go` (placeholder vazio).
- [ ] 7.5 `internal/identity/infrastructure/jobs/handlers/doc.go` (placeholder vazio).

## Detalhes de Implementação

Referenciar:
- [`techspec.md` §Infrastructure HTTP](./techspec.md) — `UserRouter` placeholder.
- [Runbook §8](../../docs/runbooks/handler-usecase-uow-repository.md) — handler canônico com `pkg/responses`.

**Shape de resposta de erro (inegociável):**

```go
responses.ErrorWithDetails(w, http.StatusConflict, "número já vinculado a outra conta",
    map[string]string{"code": "whatsapp_in_use"})
```

- Mensagem humana em PT-BR.
- `code` em snake_case dentro de `details` — contrato estável para clientes mesmo com o pacote minimalista.

**Sem mascaramento manual:** se precisar logar identificador WhatsApp, usar `whatsapp.Masked()` direto via VO (já testado em 2.0).

## Critérios de Sucesso

- `go build ./...` verde.
- `go test -race -count=1 ./internal/identity/infrastructure/http/...` verde.
- `grep -nE "func writeError|func writeJSON|w\.Header\(\)\.Set\(\"Content-Type" internal/identity/infrastructure/http/` retorna 0 (R6.10).
- `grep -nE "responses\\.(JSON|Error|ErrorWithDetails)" internal/identity/infrastructure/http/server/handlers/` aparece em todo handler.
- `UserRouter` satisfaz `chi_server.Router` — validar implicitamente pela assinatura de `srv.RegisterRouters(NewUserRouter(handler))` compilar (R6.4 proíbe `var _ chi_server.Router = (*UserRouter)(nil)` como teste de compilação; ver `.claude/projects/.../memory/feedback_no_interface_assertion.md`).
- Handler nunca chama repository direto (cadeia Handler → UC inegociável).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] `upsert_user_by_whatsapp_handler_test.go` — table tests com `httptest.NewRecorder` + mock UC (gerado em 5.0) cobrindo todos os caminhos de status code + `code` em `details`.
- [ ] `router_test.go` — `Register(chi.Router)` sem `upsertHandler` não panica; com `upsertHandler` registra o endpoint corretamente (validar via roteamento).
- [ ] Sanidade: `chi.NewMux()` + `srv.RegisterRouters(NewUserRouter(handler))` mounted retorna handler.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/identity/infrastructure/http/server/router.go` (criar)
- `internal/identity/infrastructure/http/server/router_test.go` (criar)
- `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler.go` (criar)
- `internal/identity/infrastructure/http/server/handlers/upsert_user_by_whatsapp_handler_test.go` (criar)
- `internal/identity/infrastructure/http/client/doc.go` (criar — placeholder)
- `internal/identity/infrastructure/jobs/handlers/doc.go` (criar — placeholder)
- Dependências: `internal/identity/application/usecases/*` (5.0), `internal/identity/domain/valueobjects/*` (2.0), `internal/identity/application/errors.go` (4.0), `github.com/JailtonJunior94/devkit-go/pkg/responses`.
