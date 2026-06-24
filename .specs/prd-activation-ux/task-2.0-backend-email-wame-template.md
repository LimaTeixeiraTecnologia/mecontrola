# Tarefa 2.0: Backend — UX do email de ativação — WaMeURL, SupportURL e template

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Substituir o CTA do email de ativação pela URL wa.me com o comando `ATIVAR {token}` pré-preenchido,
remover o fallback de URL crua do token, adicionar linha de suporte com link wa.me limpo (sem texto
pré-preenchido) e atualizar o corpo plain text. Remover o campo `EMAIL_ACTIVATE_URL` que se torna
órfão. Zero regressão no envio de email — apenas a URL do botão e o conteúdo do template mudam.

<requirements>
- RF-01: botão CTA aponta para `wa_me_url` (wa.me com ATIVAR TOKEN pré-preenchido).
- RF-02: NÃO exibir URL completa do token em nenhum texto do email.
- RF-03: linha de suporte com link wa.me limpo (sem texto pré-preenchido).
- RF-04: usecase `SendActivationEmail` constrói `WaMeURL` e `SupportURL` via `botNumber`.
- Techspec seções 1 (ActivationTemplateInput), 2 (SendActivationEmail), 3 (template.go plain text), 4 (module.go), 5 (activation.html.tmpl).
- Restrição [HARD]: zero regressão no envio de email; nenhum arquivo intocável tocado.
</requirements>

## Subtarefas

- [ ] 2.1 Atualizar `ActivationTemplateInput` em `send_activation_email.go`: substituir `ActivateURL string` por `WaMeURL string` e `SupportURL string`
- [ ] 2.2 Atualizar struct `SendActivationEmail`: substituir campo `activateURL string` por `botNumber string`; atualizar `NewSendActivationEmail` aceitando `botNumber string` em vez de `activateURL string`
- [ ] 2.3 Atualizar método `Execute` em `send_activation_email.go`: construir `waMe` e `support` usando `sanitizeE164` do `e164.go` (da task 1.0) e passar ao `ActivationTemplateInput`; remover chamada a `buildActivateURL`
- [ ] 2.4 Remover função `buildActivateURL` de `send_activation_email.go` (torna-se não utilizada)
- [ ] 2.5 Atualizar `template.go` (`internal/onboarding/infrastructure/email/template.go`): substituir `in.ActivateURL` por `in.WaMeURL` na geração do corpo plain text; atualizar texto para "Ative sua conta abrindo este link no celular:"
- [ ] 2.6 Atualizar `activation.html.tmpl`: botão `href` de `{{.ActivateURL}}` para `{{.WaMeURL}}`; remover parágrafo de fallback de URL crua; substituir por linha de suporte usando `{{.SupportURL}}`
- [ ] 2.7 Atualizar wiring em `module.go` (~linha 293): substituir `emailCfg.ActivateURL` por `waCfg.BotNumberE164` no call de `NewSendActivationEmail`
- [ ] 2.8 Remover `ActivateURL string` de `configs.EmailConfig` em `configs/config.go`; remover o `SetDefault("EMAIL_ACTIVATE_URL", ...)` e a entrada no mapa de log; remover `EMAIL_ACTIVATE_URL=...` de `.env.example`
- [ ] 2.9 Criar `internal/onboarding/application/usecases/send_activation_email_test.go` com suite testify/suite whitebox cobrindo os cenários especificados na techspec (seção Testes Unitários)
- [ ] 2.10 Executar `go build ./...` e `go test ./internal/onboarding/...` — zero erros
- [ ] 2.11 Executar gate de comentários e gate de arquivos intocáveis
- [ ] 2.12 Commit semântico: `feat(onboarding): substituir CTA do email por deep link wa.me e remover fallback de URL crua`

## Detalhes de Implementação

Ver techspec seções:
- **Seção 1** (`ActivationTemplateInput`) — campos `WaMeURL` e `SupportURL`
- **Seção 2** (`SendActivationEmail`) — constructor, `Execute`, construção de `waMe` e `support`
- **Seção 3** (`template.go`) — corpo plain text usando `in.WaMeURL`
- **Seção 4** (`module.go`) — wiring: `waCfg.BotNumberE164` no lugar de `emailCfg.ActivateURL`
- **Seção 5** (`activation.html.tmpl`) — HTML final esperado

### `ActivationTemplateInput` final

```go
type ActivationTemplateInput struct {
    WaMeURL        string
    SupportURL     string
    ExpiresInHours int
}
```

### Construção em `Execute`

```go
waMe    := fmt.Sprintf("https://wa.me/%s?text=ATIVAR%%20%s", sanitizeE164(uc.botNumber), in.ClearToken)
support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))
html, text, err := uc.template.Render(ActivationTemplateInput{
    WaMeURL:        waMe,
    SupportURL:     support,
    ExpiresInHours: expiresHours,
})
```

### Plain text final em `template.go`

```go
text := fmt.Sprintf(
    "Bem-vindo(a) ao MeControla!\n\nAtive sua conta abrindo este link no celular:\n%s\n\nEste link expira em %d horas.",
    in.WaMeURL,
    in.ExpiresInHours,
)
```

### Suite de testes obrigatória (`send_activation_email_test.go`)

Padrão R-TESTING-001: whitebox (`package usecases`), testify/suite, `fake.NewProvider()`,
`dependencies` struct com IIFE por mock.

Cenários mínimos:
| Cenário | Verificação |
|---|---|
| Token e email válidos | `ActivationTemplateInput.WaMeURL` contém `wa.me/{botNumber}?text=ATIVAR%20{token}` |
| Token e email válidos | `ActivationTemplateInput.SupportURL == "https://wa.me/{botNumber}"` (sem query param) |
| Email vazio | retorna `nil` sem chamar template nem sender |
| Token vazio | retorna erro sem chamar template nem sender |
| Render falha | propaga erro com wrapping `"render template"` |
| Send falha | incrementa contador `send_failed`; retorna erro com wrapping `"send"` |

## Critérios de Sucesso

- `go build ./...` passa sem erros após remover `ActivateURL` de `EmailConfig`.
- `go test ./internal/onboarding/...` passa 100%.
- Gate de comentários: arquivos modificados e novo test file retornam vazio no grep de comentários proibidos.
- Gate de arquivos intocáveis: `git diff --name-only HEAD | grep -E "consume_magic_token|whatsapp_message_processor|dispatcher|activation_command|magic_token_repository"` → vazio.
- Template HTML renderizado não contém a string `token=` em nenhum texto visível.
- `ActivationTemplateInput.SupportURL` não contém `?text=` (link limpo sem texto pré-preenchido).
- `configs/config.go` não contém mais referência a `ActivateURL` nem `EMAIL_ACTIVATE_URL`.
- `suite.Suite` com `fake.NewProvider()` (não `noop`) no novo arquivo de teste.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `semantic-commit` — tarefa encerra com commit semântico estruturado cobrindo as mudanças de UX do email e remoção do campo órfão

## Testes da Tarefa

- [ ] `send_activation_email_test.go` — cenário token+email válidos: `WaMeURL` contém `wa.me/?text=ATIVAR%20`
- [ ] `send_activation_email_test.go` — cenário token+email válidos: `SupportURL` não contém `?text=`
- [ ] `send_activation_email_test.go` — email vazio: retorna nil sem chamar template
- [ ] `send_activation_email_test.go` — token vazio: retorna erro
- [ ] `send_activation_email_test.go` — render falha: erro propagado
- [ ] `send_activation_email_test.go` — send falha: contador `send_failed` incrementado
- [ ] Template HTML renderizado: verificar manualmente ausência de texto contendo `token=`
- [ ] `go test ./internal/onboarding/...` — 100% pass

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

**Modificados:**
- `internal/onboarding/application/usecases/send_activation_email.go`
- `internal/onboarding/infrastructure/email/template.go`
- `internal/onboarding/infrastructure/email/templates/activation.html.tmpl`
- `internal/onboarding/module.go`
- `configs/config.go`
- `.env.example`

**Criados:**
- `internal/onboarding/application/usecases/send_activation_email_test.go`

**Consumido (read-only):**
- `internal/onboarding/application/usecases/e164.go` (criado na task 1.0 — usar `sanitizeE164` deste arquivo)

**Intocáveis (zero mudança):**
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/application/services/whatsapp_message_processor.go`
- `internal/platform/whatsapp/dispatcher/dispatcher.go`
- `internal/platform/channels/activation_command.go`
