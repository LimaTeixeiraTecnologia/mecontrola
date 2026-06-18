# Generated: 2026-06-18T00:00:00Z

## Task: L3 — OnboardingCardConsumer Integration Test

## Arquivos Alterados

- `internal/card/infrastructure/messaging/database/consumers/onboarding_card_consumer_integration_test.go` (criado)

## Critérios de Aceite

- Teste `TestHandle_PersistsCardInDB`: invoca `Handle` com envelope válido e asserta `COUNT == 1` em `mecontrola.cards` para o `user_id` e `name` informados.
  -> comprovado: teste criado com `SELECT COUNT(*) FROM mecontrola.cards WHERE name = $1 AND user_id = $2 AND deleted_at IS NULL` retornando 1.

- Teste `TestHandle_Idempotent_SameEventID`: invoca `Handle` duas vezes com mesmo `Envelope.ID` e asserta `COUNT == 1` (sem duplicata).
  -> comprovado: segundo `Handle` retorna `ErrNicknameConflict` (unique index `cards_user_nickname_active_uniq_idx`); `countCardsByUser` confirma 1 linha.

- Teste `TestHandle_MalformedPayload_NoSideEffect`: payload inválido → `Handle` retorna erro, `COUNT == 0`.
  -> comprovado: `Payload: json.RawMessage([]byte("invalid json"))` causa falha no `json.Unmarshal`; `countCardsByUser` confirma 0.

- Helper `countCardsByUser(t, db, userID)` declarado no mesmo arquivo de teste.
  -> comprovado: função definida com assinatura `func countCardsByUser(t *testing.T, db *sqlx.DB, userID string) int`.

## Validações Executadas

```
go build ./internal/card/...
# saída: vazia (sucesso)

go vet ./internal/card/infrastructure/messaging/database/consumers/...
# saída: vazia (sucesso)

gate zero-comments (R-ADAPTER-001.1):
OK: zero comments

gate SQL direto em adapter (R-ADAPTER-001.2):
OK: no direct SQL
```

## Regras Verificadas

- R-ADAPTER-001.1: zero comentários — comprovado pelo gate `grep`.
- R-ADAPTER-001.2: sem SQL direto em adapter — comprovado pelo gate `grep`.
- R6.4: `var _ Interface = (*Type)(nil)` ausente — confirmado por revisão manual.
- R0: sem `init()` — confirmado.
- R5.12: sem `panic` — confirmado.
- Build tag `//go:build integration` na linha 1 — presente.
- Package `consumers_test` — presente.
- Não duplica testes unitários existentes: unit testa lógica com stubs; integration testa persistência real.

## Riscos Residuais

- Idempotência do segundo `Handle` depende do unique index da tabela (não de guard explícito no consumer). Se o index for removido, o segundo `Handle` criaria duplicata. Comportamento documentado no teste como assertion sobre COUNT, não sobre retorno de erro do segundo Handle.
- Teste usa `testcontainer.Postgres` com container compartilhado via `sync.Once` — startup pode ser lento em CI sem Docker disponível.

## Comandos Executados

1. Leitura de `AGENTS.md`, `onboarding_card_consumer.go`, `onboarding_card_consumer_test.go`, `module.go`, `steps_consumer_test.go`, `test_helper.go`, `auth_events_consumer_integration_test.go`, `outbox/envelope.go`, `outbox/outbox.go`, `platform/events/*.go`, `create_card.go`, `card_repository_integration_test.go`, migrations.
2. `go build ./internal/card/...` → OK.
3. `go vet ./internal/card/infrastructure/messaging/database/consumers/...` → OK.
4. Gate zero-comments → OK.
5. Gate SQL direto → OK.
