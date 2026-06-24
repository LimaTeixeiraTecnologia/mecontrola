<!-- spec-hash-prd: 8097e774290bb9f84f64e0f08a0adcfbac263aafe5d7065c531cc3b84d808e72 -->

# Especificação Técnica — Ativação UX

## Resumo Executivo

A melhoria de UX da ativação envolve **cinco arquivos de produção** em dois repositórios e
**zero mudança** no caminho de processamento do token (dispatcher, regex, consume). As alterações
são cirúrgicas: (1) o usecase `SendActivationEmail` passa a construir o `WaMeURL` e o template
HTML troca o CTA da URL da página web para o deep link wa.me; (2) o usecase `GetTokenState` e seu
handler passam a expor o `reason` e, para tokens consumidos, o `WaMeURL` na resposta JSON; (3) o
frontend (`mecontrola-landingpage`) atualiza `activate.js` e `activate.astro` para mostrar mensagens
específicas por estado, adicionar contagem regressiva de 3s e tratar a conta já ativa como sucesso;
(4) a rota `/ativar` passa a ser a página canonical e `/activate` vira redirect 301.

A restrição `[HARD]` de zero regressão é garantida pelo escopo: nenhum dos arquivos tocados está no
caminho `consume_magic_token → whatsapp_message_processor → dispatcher`.

## Arquitetura do Sistema

### Componentes Modificados

| Repositório | Arquivo | Tipo de mudança |
|---|---|---|
| `mecontrola` | `internal/onboarding/application/usecases/send_activation_email.go` | Adicionar `botNumber`, construir `WaMeURL` |
| `mecontrola` | `internal/onboarding/infrastructure/email/templates/activation.html.tmpl` | Trocar CTA, remover fallback URL |
| `mecontrola` | `internal/onboarding/application/dtos/output/get_token_state_output.go` | Adicionar `Reason`, `WaMeURL` para consumed |
| `mecontrola` | `internal/onboarding/application/usecases/get_token_state.go` | Preencher `Reason` e `WaMeURL` no resultado |
| `mecontrola` | `internal/onboarding/infrastructure/http/server/handlers/token_state_handler.go` | Serializar `reason` e `wa_me_url` no JSON |
| `mecontrola` | `internal/onboarding/module.go` | Passar `waCfg.BotNumberE164` ao `NewSendActivationEmail` |
| `mecontrola-landingpage` | `src/pages/ativar.astro` | Tornar canonical (trazer HTML real) |
| `mecontrola-landingpage` | `src/pages/activate.astro` | Virar redirect 301 para `/ativar` |
| `mecontrola-landingpage` | `public/js/activate.js` | Reason-aware errors, countdown, timeout, consumed |

### Fluxo de Dados — Mobile (caminho feliz pós-mudança)

```
Email enviado com WaMeURL no botão CTA
  → Usuário clica "Ativar MeControla"
  → wa.me abre WhatsApp com texto "ATIVAR {token}" pré-preenchido
  → Usuário pressiona Enviar
  → dispatcher → onboardingRoute → HandleActivation → ConsumeMagicToken
     (sem qualquer mudança neste trecho)
```

### Fluxo de Dados — Desktop (caminho bridge)

```
Usuário clica no email (wa.me abre WhatsApp Web) ou acessa /ativar manualmente
  → ativar.astro serve a página bridge
  → activate.js: GET /api/v1/onboarding/tokens/{token}/state (timeout 5s)
  → TokenStateHandler → GetTokenState.Execute
  → se ready_to_activate: true  → countdown 3s → window.location = wa_me_url
  → se reason=consumed          → estado "conta ativa" + botão WhatsApp
  → se reason=expired|pending|not_found → mensagem específica + link suporte
```

## Design de Implementação

### 1. `ActivationTemplateInput` — campos `WaMeURL` e `SupportURL`

```go
type ActivationTemplateInput struct {
    WaMeURL        string   // deep link com ATIVAR TOKEN pré-preenchido (botão CTA)
    SupportURL     string   // wa.me limpo sem texto (link de suporte)
    ExpiresInHours int
}
```

`ActivateURL` é removido do struct. O template não precisa mais da URL da página web.

### 2. `SendActivationEmail` — constructor e `Execute`

```go
func NewSendActivationEmail(
    sender        appinterfaces.EmailSender,
    template      ActivationTemplate,
    botNumber     string,   // novo: waCfg.BotNumberE164
    fromAddress   string,
    fromName      string,
    replyTo       string,
    tokenTTL      time.Duration,
    o11y          observability.Observability,
) *SendActivationEmail
```

O campo `activateURL string` é removido do struct; `botNumber string` é adicionado.

Em `Execute`, substituir:
```go
activate := buildActivateURL(uc.activateURL, in.ClearToken)
// ...
html, text, err := uc.template.Render(ActivationTemplateInput{
    ActivateURL:    activate,
    ExpiresInHours: expiresHours,
})
```

Por:
```go
waMe    := fmt.Sprintf("https://wa.me/%s?text=ATIVAR%%20%s", sanitizeE164(uc.botNumber), in.ClearToken)
support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))
html, text, err := uc.template.Render(ActivationTemplateInput{
    WaMeURL:        waMe,
    SupportURL:     support,
    ExpiresInHours: expiresHours,
})
```

`buildActivateURL` e `sanitizeE164` — a função `sanitizeE164` já existe em `get_token_state.go`.
Mover para um helper interno do pacote `usecases` (arquivo `e164.go` ou `helpers.go`) para
evitar duplicação entre os dois usecases do mesmo pacote.

### 3. `template.go` — corpo plain text

O método `Render` em `internal/onboarding/infrastructure/email/template.go` gera o corpo
plain text hardcoded usando `in.ActivateURL`. Substituir por `in.WaMeURL`:

```go
text := fmt.Sprintf(
    "Bem-vindo(a) ao MeControla!\n\nAtive sua conta abrindo este link no celular:\n%s\n\nEste link expira em %d horas.",
    in.WaMeURL,
    in.ExpiresInHours,
)
```

### 4. `module.go` — wiring atualizado

```go
sendActivationEmail: usecases.NewSendActivationEmail(
    emailSender,
    activationTemplate,
    waCfg.BotNumberE164,   // novo argumento
    emailCfg.FromAddress,
    emailCfg.FromName,
    emailCfg.ReplyTo,
    deps.runtimeCfg.TokenTTL,
    o11y,
),
```

`emailCfg.ActivateURL` não é mais necessário nesta chamada. O campo `ActivateURL` em
`configs.EmailConfig` e a variável de ambiente `EMAIL_ACTIVATE_URL` devem ser removidos
(confirmado: único uso é `module.go` linha ~296 e `template.go` linha 47 — ambos serão modificados).

### 4. `activation.html.tmpl` — template HTML

```html
<!-- ANTES: href="{{.ActivateURL}}" -->
<a href="{{.WaMeURL}}" style="...">Ativar MeControla</a>

<!-- ANTES: parágrafo de fallback com URL crua -->
<!-- REMOVIDO: <p>Se o botão não funcionar, copie e cole...</p> -->

<!-- NOVO: linha de suporte (SupportURL = wa.me limpo, sem ATIVAR TOKEN) -->
<p style="color:#64748b; font-size:13px; line-height:1.5;">
  Dificuldades?
  <a href="{{.SupportURL}}" style="color:#0f172a;">Fale conosco pelo WhatsApp</a>.
</p>
```

`ExpiresInHours` permanece no template na linha de rodapé existente — sem mudança.

### 6. `GetTokenStateOutput` DTO — campos adicionais

```go
type GetTokenStateOutput struct {
    ReadyToActivate  bool
    WaMeURL          string
    TelegramDeepLink string
    BotNumberDisplay string
    Reason           string   // preenchido quando ReadyToActivate=false
    SupportURL       string   // sempre preenchido: "https://wa.me/{botNumber}" (sem texto)
}
```

`Reason` usa as constantes `TokenStateReason*` já definidas em `get_token_state.go`
(`"not_found"`, `"pending"`, `"expired"`, `"consumed"`).

`SupportURL` é a URL limpa do bot **sem** query param `text=` — presente em **todas** as
respostas (ready e não-ready) para que o frontend sempre tenha acesso ao link de suporte.

### 7. `get_token_state.go` — preencher `Reason`, `WaMeURL` consumed e `SupportURL` sempre

**Bloco do estado ready — adicionar `SupportURL`:**
```go
return GetTokenStateResult{
    Output: output.GetTokenStateOutput{
        ReadyToActivate:  true,
        WaMeURL:          waMe,
        TelegramDeepLink: tgLink,
        BotNumberDisplay: uc.botNumberDisplay,
        SupportURL:       fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber)),
    },
}, nil
```

**Bloco do estado não-ready — substituir:**
```go
reason := reasonFromStatus(magicToken.Status(), magicToken.IsExpiredAt(now))
return GetTokenStateResult{
    Output: output.GetTokenStateOutput{ReadyToActivate: false},
    Reason: reason,
}, nil
```

**Por:**
```go
reason := reasonFromStatus(magicToken.Status(), magicToken.IsExpiredAt(now))
support := fmt.Sprintf("https://wa.me/%s", sanitizeE164(uc.botNumber))
out := output.GetTokenStateOutput{
    ReadyToActivate: false,
    Reason:          string(reason),
    SupportURL:      support,
}
if reason == TokenStateReasonConsumed {
    out.WaMeURL = fmt.Sprintf("https://wa.me/%s?text=ATIVAR%%20%s",
        sanitizeE164(uc.botNumber), clearToken)
    out.BotNumberDisplay = uc.botNumberDisplay
}
return GetTokenStateResult{Output: out, Reason: reason}, nil
```

### 8. `token_state_handler.go` — serializar `reason` e `support_url` no JSON

```go
type tokenStateResponse struct {
    ReadyToActivate  bool   `json:"ready_to_activate"`
    WaMeURL          string `json:"wa_me_url,omitempty"`
    TelegramDeepLink string `json:"telegram_deep_link,omitempty"`
    BotNumberDisplay string `json:"bot_number_display,omitempty"`
    Reason           string `json:"reason,omitempty"`       // novo
    SupportURL       string `json:"support_url,omitempty"`  // novo: sempre presente
}
```

Bloco ready (atualizado):
```go
responses.JSON(w, http.StatusOK, tokenStateResponse{
    ReadyToActivate:  true,
    WaMeURL:          result.Output.WaMeURL,
    TelegramDeepLink: result.Output.TelegramDeepLink,
    BotNumberDisplay: result.Output.BotNumberDisplay,
    SupportURL:       result.Output.SupportURL,
})
```

Bloco não-ready (atualizado):
```go
responses.JSON(w, http.StatusOK, tokenStateResponse{
    ReadyToActivate:  false,
    Reason:           string(result.Reason),
    WaMeURL:          result.Output.WaMeURL,          // preenchido apenas para consumed
    BotNumberDisplay: result.Output.BotNumberDisplay, // preenchido apenas para consumed
    SupportURL:       result.Output.SupportURL,       // sempre preenchido
})
```

O jitter (`cryptoJitter`) **permanece** antes do JSON para estados não-ready — sem mudança.

### 8. API Contract — resposta do endpoint

`GET /api/v1/onboarding/tokens/{token}/state`

**Caso `ready_to_activate: true`** (atualizado — adiciona `support_url`):
```json
{
  "ready_to_activate": true,
  "wa_me_url": "https://wa.me/5511999999999?text=ATIVAR%20TOKEN",
  "bot_number_display": "+55 11 9XXXX-XXXX",
  "telegram_deep_link": "https://t.me/bot?start=ATIVAR_TOKEN",
  "support_url": "https://wa.me/5511999999999"
}
```

**Caso `ready_to_activate: false` com reason (novo — sempre inclui `support_url`)**:
```json
{ "ready_to_activate": false, "reason": "expired",   "support_url": "https://wa.me/5511999999999" }
{ "ready_to_activate": false, "reason": "pending",   "support_url": "https://wa.me/5511999999999" }
{ "ready_to_activate": false, "reason": "not_found", "support_url": "https://wa.me/5511999999999" }
```

**Caso consumed (novo — conta já ativa, inclui wa.me com token e support_url)**:
```json
{
  "ready_to_activate": false,
  "reason": "consumed",
  "wa_me_url": "https://wa.me/5511999999999?text=ATIVAR%20TOKEN",
  "bot_number_display": "+55 11 9XXXX-XXXX",
  "support_url": "https://wa.me/5511999999999"
}
```

`Cache-Control: no-store` permanece em todas as respostas.

### 9. Frontend — `ativar.astro` (canonical) e `activate.astro` (redirect)

**`ativar.astro`** recebe o HTML completo atualmente em `activate.astro` (copiar integralmente,
ajustar canonical para `https://mecontrola.app.br/ativar`).

**`activate.astro`** passa a emitir apenas redirect 301:
```astro
---
return Astro.redirect('/ativar' + Astro.url.search, 301);
---
```

### 10. Frontend — `activate.js` — reason-aware + countdown + timeout

**Contrato de mudanças:**

a) **Timeout 5s via `AbortController`:**
```js
const controller = new AbortController();
const id = setTimeout(() => controller.abort(), 5000);
try {
  response = await fetch(url, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    signal: controller.signal,
  });
} finally {
  clearTimeout(id);
}
```

b) **Mensagens por `reason`:**
```js
const ERROR_MESSAGES = {
  expired:   'Seu link de ativação expirou. Fale conosco pelo WhatsApp para receber um novo link.',
  pending:   'Seu pagamento ainda está sendo processado. Aguarde alguns minutos e tente novamente.',
  consumed:  null,   // tratado como estado positivo (conta já ativa)
  not_found: 'Link inválido. Verifique o link do email ou fale conosco pelo WhatsApp.',
};
```

c) **Estado `consumed` como sucesso:**

Quando `reason === 'consumed'`:
- Exibir `#activate-consumed` (novo elemento) com título "Sua conta já está ativa!"
- Se `wa_me_url` presente: preencher `#activate-consumed-wa-btn` e exibir botão
- **Não** exibir `#activate-error`

d) **Countdown de 3s para `ready_to_activate: true`:**
```js
const REDIRECT_DELAY_MS = 3000;
let remaining = 3;
const el = document.getElementById('activate-countdown');
if (el) el.textContent = `${remaining}`;
const interval = setInterval(() => {
  remaining -= 1;
  if (el) el.textContent = `${remaining}`;
  if (remaining <= 0) {
    clearInterval(interval);
    window.location.href = data.wa_me_url;
  }
}, 1000);
```

O botão `#activate-wa-btn` é preenchido e visível **imediatamente** — o countdown é apenas visual.
Se o usuário clicar antes dos 3s, o `clearInterval` não é necessário (navegação ocorre).

e) **Erro de rede (timeout/fetch falhou):**
```js
showError('Não foi possível conectar ao servidor. Verifique sua conexão e tente novamente.');
```

### 11. Frontend — `activate.astro` — novos elementos HTML

Dentro de `#activate-card`, adicionar:

**Estado consumed (`#activate-consumed`):**
```html
<div id="activate-consumed" class="mt-8 hidden flex-col items-center text-center" aria-live="polite">
  <div class="h-12 w-12 rounded-full bg-green-100 flex items-center justify-center mb-3" aria-hidden="true">
    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#16a34a" stroke-width="2.5">
      <polyline points="20 6 9 17 4 12"></polyline>
    </svg>
  </div>
  <p class="text-base font-semibold text-slate-900">Sua conta já está ativa!</p>
  <p class="mt-1 text-sm text-slate-500">Abra o WhatsApp e envie uma mensagem para o MeControla.</p>
  <a id="activate-consumed-wa-btn" href="#" target="_blank" rel="noopener noreferrer"
     class="mt-4 w-full inline-flex items-center justify-center gap-2 rounded-xl px-5 py-3.5 font-semibold text-white shadow-sm hover:opacity-90 transition hidden"
     style="background-color: #25D366">
    Abrir WhatsApp
  </a>
</div>
```

**Countdown dentro de `#activate-ready`** (após o subtítulo):
```html
<p class="mt-1 text-xs text-slate-400">
  Abrindo WhatsApp em <span id="activate-countdown">3</span>s...
</p>
```

**Mensagens de erro por estado** — `#activate-error-msg` continua sendo preenchido dinamicamente
pelo JS; a tag de texto estático "Verifique o link recebido..." passa a ser preenchida por `setErrorDetail`:
```html
<p id="activate-error-detail" class="mt-3 text-xs text-slate-500"></p>
```

O link de suporte (botão wa.me do bot) na área de erro:
```html
<a id="activate-support-btn" href="#" target="_blank" rel="noopener noreferrer"
   class="mt-4 inline-flex items-center gap-1 text-xs text-green-700 underline">
  Falar com o suporte pelo WhatsApp
</a>
```

O `href` de `#activate-support-btn` é preenchido pelo JS com o `support_url` retornado
pela API — zero env var novo no frontend. O campo `support_url` está presente em todas as
respostas do endpoint (ready e não-ready), incluindo o estado consumed.

## Pontos de Integração

- **API backend → frontend**: `GET /api/v1/onboarding/tokens/{token}/state` — contrato estendido
  com `reason` e `wa_me_url` para consumed. Backward-compatible: campos novos são `omitempty`,
  frontend já testa `typeof wa !== 'string'` antes de usar.
- **Email → WhatsApp**: o deep link `wa.me` é gerado pelo backend com o número do bot já
  configurado (`waCfg.BotNumberE164`). Nenhuma dependência externa nova.

## Abordagem de Testes

### Testes Unitários — Backend

**`send_activation_email_test.go`** (novo arquivo, padrão testify/suite, whitebox):

| Cenário | Verificação |
|---|---|
| Execute com token e email válidos | `ActivationTemplateInput.WaMeURL` contém `wa.me/{botNumber}?text=ATIVAR%20{token}`, `SupportURL` contém `wa.me/{botNumber}` sem query param |
| Execute com email vazio | retorna `nil` sem chamar template (comportamento atual mantido) |
| Execute com token vazio | retorna erro sem chamar template (comportamento atual mantido) |
| Render do template falha | propaga erro com wrapping |
| Send falha | incrementa contador `send_failed` |

**`get_token_state_test.go`** — adicionar cenários:

| Cenário | Verificação |
|---|---|
| Token consumed | `Output.Reason == "consumed"`, `Output.WaMeURL` não-vazio, `Output.BotNumberDisplay` não-vazio |
| Token consumed tem wa.me correto | `Output.WaMeURL` contém `wa.me/{botNumber}?text=ATIVAR%20{clearToken}` |
| Todos os estados têm SupportURL | `Output.SupportURL == "https://wa.me/{botNumber}"` para ready, not_found, expired, pending, consumed |

**`token_state_handler_test.go`** — adicionar cenários:

| Cenário | Verificação |
|---|---|
| Estado consumed | `response["reason"] == "consumed"`, `response["wa_me_url"]` não-nil, `response["support_url"]` não-nil |
| Estado expired | `response["reason"] == "expired"`, `response["wa_me_url"]` nil, `response["support_url"]` não-nil |
| Estado pending | `response["reason"] == "pending"`, `response["support_url"]` não-nil |
| Estado not_found | `response["reason"] == "not_found"`, `response["support_url"]` não-nil |
| Estado ready | `response["support_url"]` não-nil |

Atualizar cenário "deve omitir campos quando nao estiver pronto" para verificar `reason` e `support_url` não-vazios.

### Testes E2E — Frontend

**`activate.spec.ts`** — adicionar cenários (Playwright):

| Cenário | Mock da API | Verificação |
|---|---|---|
| Token expirado | `{ ready_to_activate: false, reason: "expired" }` | `#activate-error` visível, mensagem contém "expirou" |
| Token pendente | `{ ready_to_activate: false, reason: "pending" }` | `#activate-error` visível, mensagem contém "processado" |
| Token não encontrado | `{ ready_to_activate: false, reason: "not_found" }` | `#activate-error` visível, mensagem contém "inválido" |
| Conta já ativa (consumed) | `{ ready_to_activate: false, reason: "consumed", wa_me_url: "...", bot_number_display: "..." }` | `#activate-consumed` visível, `#activate-consumed-wa-btn` com href correto, `#activate-error` oculto |
| Timeout da API | simular demora > 5s via `route.abort()` | `#activate-error` visível com mensagem de conexão |
| Support button presente | qualquer estado de erro com `support_url` na resposta | `#activate-support-btn` visível com `href == support_url` |
| Countdown visível | `ready_to_activate: true` | `#activate-countdown` começa em "3" e decrementa |
| Botão imediato | `ready_to_activate: true` | `#activate-wa-btn` visível antes do countdown terminar |

Atualizar cenário `renderiza mensagem de erro quando ready_to_activate=false` (sem reason):
manter backward-compatible → exibir mensagem genérica quando `reason` ausente.

**`/ativar` → redirect tests** — verificar que `/activate?token=test` redireciona para `/ativar?token=test` com status 301.

### Gate de Regressão Funcional

Executar antes de merge em ambos os repos:

```bash
# Backend: zero mudança nos componentes intocáveis
git diff --name-only HEAD | grep -E \
  "consume_magic_token|whatsapp_message_processor|dispatcher|activation_command|magic_token_repository" \
  && echo "FAIL: arquivo intocável foi modificado" && exit 1 || true

# Backend: zero comentários (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "^[[:space:]]*//" \
  internal/onboarding/application/usecases/send_activation_email.go \
  internal/onboarding/application/dtos/output/ \
  internal/onboarding/application/usecases/get_token_state.go \
  internal/onboarding/infrastructure/http/server/handlers/token_state_handler.go \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL: comentários proibidos" && exit 1 || true
```

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Backend — DTO + usecase `GetTokenState`** (sem dependências): adicionar `Reason` ao output,
   preencher para consumed. Testes primeiro (TDD: adicionar cenários consumed ao suite existente).
2. **Backend — handler `TokenStateHandler`**: serializar `reason` e `wa_me_url` consumed no JSON.
   Atualizar testes do handler.
3. **Backend — usecase `SendActivationEmail` + `template.go`**: adicionar `botNumber`,
   construir `WaMeURL` e `SupportURL`, atualizar `ActivationTemplateInput`. Atualizar
   plain text em `template.go`. Escrever suite de testes do usecase.
4. **Backend — `module.go` + `configs/config.go`**: passar `waCfg.BotNumberE164` ao
   constructor. Remover `emailCfg.ActivateURL` de `configs.EmailConfig` e `EMAIL_ACTIVATE_URL`
   de `configs/config.go` e `.env.example`.
5. **Backend — template HTML**: trocar CTA para `{{.WaMeURL}}`, link de suporte para
   `{{.SupportURL}}`, remover fallback URL crua. Verificar render manual.
6. **Frontend — `ativar.astro` e `activate.astro`**: flip canonical/redirect. Testar redirect 301.
7. **Frontend — `activate.js`**: timeout, reason-aware, consumed state, countdown.
8. **Frontend — `activate.astro`**: novos elementos HTML (consumed, countdown, support btn).
9. **Frontend — testes E2E**: adicionar todos os cenários novos; rodar `pnpm playwright test`.
10. **Gate de regressão**: executar checklist antes de abrir PRs.

### Dependências Técnicas

- Passo 7 depende do passo 1–2 (API já expondo `reason` e `support_url`) — em dev local, pode mockar a API.
- Passos 5–9 são independentes dos passos 1–4; podem ser desenvolvidos em paralelo.
- Nenhuma variável de ambiente nova no frontend (`support_url` vem da API).

## Monitoramento e Observabilidade

Sem novos instrumentos de observabilidade além dos já existentes:

- **Métrica existente**: `onboarding_activation_email_dispatched_total{result="sent|send_failed|no_email|no_token"}` — sem mudança de cardinalidade.
- **Span existente**: `onboarding.usecase.send_activation_email` — sem mudança.
- **Span existente**: `onboarding.usecase.get_token_state` — sem mudança.
- **`invalidAccess(reason)`** callback no handler: já instrumentado para métricas de tentativas inválidas — sem mudança.

Nenhum novo label de métrica introduzido (R-TXN-004 / R-AGENT-WF-001.5 não se aplicam diretamente,
mas a política de cardinalidade é respeitada por omissão).

## Considerações Técnicas

### Decisões Chave

- **ADR-001**: Expor `reason` na resposta JSON do endpoint de estado do token.
  Ver `adr-001-expor-reason-token-state.md`.
- **ADR-002**: Email CTA aponta para deep link wa.me, não para a página web.
  Ver `adr-002-email-cta-wame.md`.

### Riscos Conhecidos

| Risco | Probabilidade | Impacto | Mitigação |
|---|---|---|---|
| `emailCfg.ActivateURL` / `EMAIL_ACTIVATE_URL` órfão | Baixa | Compilação/config drift | Remover de `configs/config.go` e `.env.example` junto com a mudança |
| `sanitizeE164` duplicada entre usecases | Baixa | Inconsistência futura | Mover para `internal/onboarding/application/usecases/e164.go` |
| `support_url` ausente por bug no backend → suporte sem link | Baixa | UX degradada, não quebra | Gate de teste: handler_test verifica `support_url` em todos os estados |
| wa.me abre WhatsApp Web no desktop mas usuário não tem conta | Baixa | Usuário fica sem ativar | Fora do escopo MVP; documentar como limitação conhecida |

### Conformidade com Padrões

- **R-ADAPTER-001.1** `[HARD]`: zero comentários em todos os `.go` modificados.
- **R-ADAPTER-001.2** `[HARD]`: handler permanece fino — apenas serialização, sem lógica de domínio nova.
- **R-DTO-VALIDATE-001**: `ActivationTemplateInput` não é input DTO de fronteira de aplicação — isento.
- **R-TESTING-001** `[HARD]`: novo arquivo de teste `send_activation_email_test.go` deve usar
  testify/suite, whitebox package, `fake.NewProvider()` e dependências struct com IIFE.
- **R-TESTING-001.3**: `SetupTest` deve usar `fake.NewProvider()`, não `noop.NewProvider()`.

### Arquivos Relevantes e Dependentes

**`mecontrola`:**
- `internal/onboarding/application/usecases/send_activation_email.go`
- `internal/onboarding/application/usecases/e164.go` (novo helper — mover `sanitizeE164`)
- `internal/onboarding/infrastructure/email/template.go` (plain text body)
- `internal/onboarding/application/dtos/output/get_token_state_output.go`
- `internal/onboarding/application/usecases/get_token_state.go`
- `internal/onboarding/infrastructure/http/server/handlers/token_state_handler.go`
- `internal/onboarding/infrastructure/email/templates/activation.html.tmpl`
- `internal/onboarding/module.go` (wiring — linha ~293)
- `configs/config.go` (remover `ActivateURL` de `EmailConfig`)
- `.env.example` (remover `EMAIL_ACTIVATE_URL`)
- `internal/onboarding/application/usecases/get_token_state_test.go` (atualizar)
- `internal/onboarding/infrastructure/http/server/handlers/token_state_handler_test.go` (atualizar)
- `internal/onboarding/application/usecases/send_activation_email_test.go` (novo)

**`mecontrola-landingpage`:**
- `src/pages/ativar.astro` (canonical → recebe HTML de activate.astro)
- `src/pages/activate.astro` (vira redirect 301)
- `public/js/activate.js` (reason-aware, countdown, timeout)
- `tests/playwright/activate.spec.ts` (novos cenários)
- `.env` / `wrangler.toml` (variável `PUBLIC_WA_BOT_NUMBER`)

**Intocáveis (zero mudança):**
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/application/services/whatsapp_message_processor.go`
- `internal/platform/whatsapp/dispatcher/dispatcher.go`
- `internal/platform/channels/activation_command.go`
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
