# Tarefa 1.0: Backend — Estender API de estado do token com `reason` e `support_url`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender o endpoint `GET /api/v1/onboarding/tokens/{token}/state` para expor o motivo de rejeição
(`reason`) e o link de suporte limpo (`support_url`) em todas as respostas. Para o estado
`consumed`, incluir também `wa_me_url` e `bot_number_display`. Zero regressão no comportamento
existente — o jitter de timing, o `Cache-Control: no-store` e o callback `invalidAccess` permanecem
intocados.

<requirements>
- RF-11: endpoint DEVE retornar estado distinto para token já consumido com `wa_me_url` incluído.
- Techspec seções 1, 6, 7, 8 (numeração pós-atualização): DTO, get_token_state, handler.
- Restrição [HARD]: zero mudança em consume_magic_token, dispatcher, activation_command, magic_token_repository.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `internal/onboarding/application/usecases/e164.go` — mover `sanitizeE164` de `get_token_state.go` para este helper de pacote; remover a função de `get_token_state.go` e atualizar a chamada local
- [ ] 1.2 Adicionar campos `Reason string` e `SupportURL string` a `GetTokenStateOutput` em `internal/onboarding/application/dtos/output/get_token_state_output.go`
- [ ] 1.3 Atualizar `get_token_state.go`: preencher `SupportURL` em **todos** os estados (ready e não-ready); preencher `Reason` nos estados não-ready; preencher `WaMeURL` + `BotNumberDisplay` no estado `consumed`
- [ ] 1.4 Atualizar `tokenStateResponse` em `token_state_handler.go`: adicionar campos `Reason` e `SupportURL` com `json:"...,omitempty"`; serializar em ambos os blocos (ready e não-ready)
- [ ] 1.5 Atualizar `get_token_state_test.go`: adicionar cenários para estado `consumed` (verifica `WaMeURL`, `BotNumberDisplay`, `Reason`, `SupportURL`) e verificar que `SupportURL` está presente em todos os 4 estados não-ready
- [ ] 1.6 Atualizar `token_state_handler_test.go`: adicionar verificação de `reason` e `support_url` nos cenários de estados não-ready; adicionar verificação de `support_url` no cenário ready; verificar `wa_me_url` presente apenas no estado `consumed`
- [ ] 1.7 Executar `go test ./internal/onboarding/...` — todos os testes devem passar
- [ ] 1.8 Executar gates de regressão (ver Critérios de Sucesso)
- [ ] 1.9 Commit semântico: `feat(onboarding): expor reason e support_url no endpoint de estado do token`

## Detalhes de Implementação

Ver techspec seções:
- **Seção 1** (`ActivationTemplateInput`) — `sanitizeE164` a ser movida para `e164.go`
- **Seção 6** (`GetTokenStateOutput`) — campos `Reason` e `SupportURL`
- **Seção 7** (`get_token_state.go`) — blocos ready e não-ready completos
- **Seção 8** (`token_state_handler.go`) — struct `tokenStateResponse` e os dois blocos de resposta
- **Seção 9 — API Contract** — exemplos exatos de JSON para cada estado

### Contrato da API após a task

```json
// ready_to_activate: true
{ "ready_to_activate": true, "wa_me_url": "...", "bot_number_display": "...", "support_url": "https://wa.me/{bot}" }

// not_found / expired / pending
{ "ready_to_activate": false, "reason": "expired", "support_url": "https://wa.me/{bot}" }

// consumed
{ "ready_to_activate": false, "reason": "consumed", "wa_me_url": "...", "bot_number_display": "...", "support_url": "https://wa.me/{bot}" }
```

### Regras obrigatórias (R-ADAPTER-001, R-TESTING-001)

- Zero comentários em todos os arquivos `.go` modificados (exceto `//go:`, `//nolint:`, `// Code generated`).
- `get_token_state_test.go` usa testify/suite whitebox, `fake.NewProvider()`, `dependencies` struct com IIFE por mock.
- `token_state_handler_test.go` é `package handlers_test` (blackbox) — exceção documentada da R-TESTING-001 (handler test).

## Critérios de Sucesso

- `go build ./...` passa sem erros.
- `go test ./internal/onboarding/...` passa 100% sem skip.
- Gate de comentários: `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" internal/onboarding/application/dtos/output/ internal/onboarding/application/usecases/e164.go internal/onboarding/application/usecases/get_token_state.go internal/onboarding/infrastructure/http/server/handlers/token_state_handler.go | grep -Ev "(//go:|//nolint:|// Code generated)"` → retorna vazio.
- Gate de arquivos intocáveis: `git diff --name-only HEAD | grep -E "consume_magic_token|whatsapp_message_processor|dispatcher|activation_command|magic_token_repository"` → retorna vazio.
- Resposta JSON do handler para estado `consumed` contém `reason`, `wa_me_url`, `bot_number_display` e `support_url`.
- Resposta JSON do handler para `expired`, `pending`, `not_found` contém `reason` e `support_url`; não contém `wa_me_url`.
- Resposta JSON para `ready_to_activate: true` contém `support_url`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `semantic-commit` — tarefa encerra com commit semântico estruturado (feat/fix) cobrindo as mudanças de API e DTO

## Testes da Tarefa

- [ ] `get_token_state_test.go` — cenário `consumed` retorna `WaMeURL`, `Reason="consumed"`, `SupportURL` não-vazio
- [ ] `get_token_state_test.go` — todos os 4 estados não-ready retornam `SupportURL` não-vazio
- [ ] `get_token_state_test.go` — estado ready retorna `SupportURL` não-vazio
- [ ] `token_state_handler_test.go` — JSON para `consumed` inclui `reason`, `wa_me_url`, `support_url`
- [ ] `token_state_handler_test.go` — JSON para `expired`, `pending`, `not_found` inclui `reason` e `support_url`; não inclui `wa_me_url`
- [ ] `token_state_handler_test.go` — JSON para ready inclui `support_url`
- [ ] `go test ./internal/onboarding/...` — 100% pass

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

**Criados:**
- `internal/onboarding/application/usecases/e164.go` (novo helper)

**Modificados:**
- `internal/onboarding/application/dtos/output/get_token_state_output.go`
- `internal/onboarding/application/usecases/get_token_state.go`
- `internal/onboarding/infrastructure/http/server/handlers/token_state_handler.go`
- `internal/onboarding/application/usecases/get_token_state_test.go`
- `internal/onboarding/infrastructure/http/server/handlers/token_state_handler_test.go`

**Intocáveis (zero mudança):**
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/application/services/whatsapp_message_processor.go`
- `internal/platform/whatsapp/dispatcher/dispatcher.go`
- `internal/platform/channels/activation_command.go`
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
