# ADR-004 — Erros tipados em `internal/identity/application/errors.go`

## Metadados

- **Título:** Estrutura e localização dos erros tipados do módulo identity
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-10, F-06
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-04**
  - Regras Go aplicáveis: R5.10 (`errors.New`/`fmt.Errorf("ctx: %w", err)`/sentinels), R7.6 (`errors.Join`).

## Contexto

O port `UserRepository` (RF-10) precisa expor erros tipados para os casos de:

- usuário não encontrado;
- violação de unicidade de `whatsapp_number`;
- (eventualmente) violação de unicidade de `email`.

Restrições:

- `domain` não importa `application` (regra hexagonal); errors usados pelo port `UserRepository` (que vive em `application`) precisam morar em `application` ou em pacote auxiliar que `application` possa importar.
- `infrastructure` (postgres) precisa **produzir** o erro tipado quando detecta `pgerrcode.UniqueViolation`.
- Use cases em `application` precisam **traduzir** ou propagar para handlers HTTP em `infrastructure/http/server`, que então mapeia para status code.
- Handlers de HTTP usam `errors.Is(err, ...)` para mapear código (R5.10).

O working tree não tem precedente exato em `internal/identity`. O padrão em `internal/platform/outbox` usa sentinels exportados (`ErrNoActiveTransaction`).

## Decisão

Os erros sentinels são declarados em **um único arquivo** `internal/identity/application/errors.go`, exportados:

```go
package application

import "errors"

var (
    // ErrUserNotFound é retornado por UserRepository.FindByID e FindByWhatsAppNumber
    // quando o usuário não existe ou está soft-deletado.
    ErrUserNotFound = errors.New("identity: user not found")

    // ErrWhatsAppNumberInUse é retornado por UserRepository.UpsertByWhatsAppNumber
    // e demais operações que detectam violação de unicidade do número.
    ErrWhatsAppNumberInUse = errors.New("identity: whatsapp number already in use")

    // ErrEmailInUse é retornado quando há violação de unicidade do email.
    ErrEmailInUse = errors.New("identity: email already in use")
)
```

**Wrapping obrigatório em toda camada:**

- Repository (postgres) detecta `pgerrcode.UniqueViolation` e retorna:
  ```go
  return User{}, fmt.Errorf("upsert user: %w", application.ErrWhatsAppNumberInUse)
  ```
- Use case propaga sem re-wrapper desnecessário (`return err`) ou adiciona contexto curto quando muda de camada.
- Handler HTTP mapeia via `errors.Is`:
  ```go
  switch {
  case errors.Is(err, application.ErrUserNotFound):
      writeStatus(w, http.StatusNotFound)
  case errors.Is(err, application.ErrWhatsAppNumberInUse),
       errors.Is(err, application.ErrEmailInUse):
      writeStatus(w, http.StatusConflict)
  default:
      writeStatus(w, http.StatusInternalServerError)
  }
  ```

**Agregação:** quando o repository precisa devolver múltiplos erros (e.g., rollback + erro original), usa `errors.Join` (R7.6), preservando os sentinels no início da cadeia.

## Alternativas Consideradas

### A) Tipos struct customizados (`type UserNotFoundError struct{ ID string }`)

- **Vantagens:** carregam contexto extra (ID, número, etc.); permitem `errors.As` para extrair.
- **Desvantagens:**
  - Mais código para benefício marginal nos casos do MVP.
  - Handlers HTTP raramente precisam do contexto adicional (status code é a única decisão).
- **Motivo de não escolher:** YAGNI no MVP. Pode evoluir sob demanda — sentinel atual permanece compatível.

### B) Erros no pacote `domain` em vez de `application`

- **Vantagens:** disponíveis para qualquer consumidor sem cross-layer.
- **Desvantagens:**
  - `ErrUserNotFound` é semântica de port (repositório), não de invariante de domínio.
  - Polui domínio com conceitos de infraestrutura ("não encontrado" só faz sentido com persistência).
- **Motivo de não escolher:** confunde fronteiras hexagonais.

### C) Pacote dedicado `internal/identity/errors`

- **Vantagens:** isola erros.
- **Desvantagens:**
  - Cria pacote raso só para constantes.
  - `application` precisaria importar `errors`, e `infrastructure` precisaria importar `errors` — dois imports onde um basta.
- **Motivo de não escolher:** overhead estrutural sem ganho.

## Consequências

### Benefícios Esperados

- **Mapeamento HTTP determinístico** via `errors.Is`.
- **Padrão Go idiomático** (R5.10 explicit: sentinels quando consumidor usa `errors.Is`).
- **Zero overhead de tipo** — sentinels são variáveis globais const-like.
- **Fácil extensão:** novo erro → nova `var Err... = errors.New(...)`.

### Trade-offs e Custos

- Carregar metadata (ID do usuário, número) exige re-wrapping com `fmt.Errorf` ou evolução para struct no futuro.
- Sentinels exportados ficam disponíveis para qualquer consumidor — `internal/billing` poderia importar e checar (cross-module por interface no consumidor). Aceito conscientemente; alternativa seria expor uma interface "stable error code", overkill.

### Riscos e Mitigações

- **Risco:** repository retorna `application.ErrXxx` sem wrapping (`return err`), perdendo contexto de operação no log.
  - **Mitigação:** convenção `return fmt.Errorf("upsert user: %w", application.ErrXxx)`; revisão de PR cobre.
- **Risco:** novo erro adicionado sem mapeamento no handler HTTP cai em 500 silencioso.
  - **Mitigação:** testes de handler cobrem cada `Err...` declarado; CA-04 inclui caminho de unicidade.

## Plano de Implementação

1. Criar `internal/identity/application/errors.go` com os 3 sentinels.
2. Repository postgres (`internal/identity/infrastructure/repositories/postgres/user_repository.go`) detecta `pgerrcode.UniqueViolation` via `errors.As(err, &*pgconn.PgError{})` e retorna o sentinel apropriado wrapped.
3. Use cases em `internal/identity/application/usecases/` propagam sem alterar tipo do erro (R5.10: tratar erro **uma única vez**).
4. Handlers HTTP em `internal/identity/infrastructure/http/server/handlers/` mapeiam via `errors.Is` para status code.
5. Testes:
   - `repository_test.go` (integração): valida que cada caso retorna o sentinel correto.
   - `handler_test.go`: valida que cada sentinel mapeia para o status code esperado.

## Monitoramento e Validação

- **Validação imediata:** `go test -race -count=1 ./internal/identity/...`.
- **Lint:** `golangci-lint run` valida wrap (`errorlint` analyzer já habilitado no working tree).
- **Sinal de drift:** logs com `error: identity: user not found` aparecendo em 500 indicam mapeamento faltante — alerta em Loki/Datadog para taxa anômala de 500 com payload contendo sentinel.

## Impacto em Documentação e Operação

- `internal/identity/doc.go` lista os sentinels e seu mapeamento HTTP.
- README do módulo (após F-12) documenta o contrato de erros.

## Revisão Futura

- Revisitar se algum consumidor (E2/E3) precisar de metadata estruturada (ID, número) — promover para struct com `Unwrap()`.
- Revisitar se surgir necessidade de códigos de erro estáveis para resposta API (introduzir `type Code string` separado dos sentinels).
- Revisitar se o número de sentinels crescer além de ~8 (agrupar por sub-pacote ou avaliar i18n).
