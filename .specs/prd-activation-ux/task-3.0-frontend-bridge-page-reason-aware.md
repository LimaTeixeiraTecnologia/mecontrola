# Tarefa 3.0: Frontend — Página bridge canonical e `activate.js` reason-aware

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

No repositório `mecontrola-landingpage`: tornar `/ativar` a rota canonical da página de ativação
(movendo o HTML real para `ativar.astro`), transformar `activate.astro` em redirect 301, e
atualizar `activate.js` para mostrar mensagens específicas por estado (`reason`), exibir countdown
de 3s para redirect automático, adicionar timeout de 5s na chamada à API, tratar o estado
`consumed` como sucesso (conta já ativa) e usar `support_url` da API no botão de suporte.
Todos os estados de erro exibem link de suporte via `support_url`. Zero regressão na UX existente
para tokens válidos.

<requirements>
- RF-05: página lê token do query param e chama `GET /api/v1/onboarding/tokens/{token}/state`.
- RF-06: estado success → countdown 3s → redirect automático para `wa_me_url`.
- RF-07: botão "Abrir WhatsApp" visível imediatamente (sem aguardar countdown).
- RF-08: mensagens humanizadas por reason (expired, pending, not_found, consumed).
- RF-09: usar logo oficial presente no repositório — zero criação de novo asset.
- RF-10: rota canonical `/ativar`; `/activate` vira redirect 301.
- Techspec seções 9 (API contract), 10 (ativar.astro/activate.astro flip), 11 (activate.js), 12 (novos elementos HTML).
- Dependência: task 1.0 deve estar implantada ou mockada antes do smoke test final.
</requirements>

## Subtarefas

- [ ] 3.1 Copiar HTML completo de `src/pages/activate.astro` para `src/pages/ativar.astro`; atualizar canonical para `https://mecontrola.app.br/ativar`; adicionar `data-backend-url` do env `PUBLIC_BACKEND_URL`
- [ ] 3.2 Substituir conteúdo de `src/pages/activate.astro` por redirect 301: `return Astro.redirect('/ativar' + Astro.url.search, 301)`
- [ ] 3.3 Adicionar elemento `#activate-consumed` em `ativar.astro` (div com ícone check verde, título "Sua conta já está ativa!", texto explicativo e `#activate-consumed-wa-btn` oculto por default)
- [ ] 3.4 Adicionar `#activate-countdown` em `ativar.astro` dentro de `#activate-ready` (span com "3" decrescendo)
- [ ] 3.5 Adicionar `#activate-support-btn` e `#activate-error-detail` em `ativar.astro` dentro de `#activate-error` (link wa.me preenchido pelo JS com `support_url`)
- [ ] 3.6 Atualizar `setView` em `activate.js` para incluir o estado `'consumed'` (toggle de `#activate-consumed`)
- [ ] 3.7 Adicionar timeout 5s via `AbortController` no `fetchTokenState`
- [ ] 3.8 Adicionar mapa `ERROR_MESSAGES` com mensagens por `reason` (expired, pending, not_found); adicionar função `showErrorByReason(reason, supportUrl)`
- [ ] 3.9 Implementar estado `consumed` em `init()`: exibir `#activate-consumed`, preencher `#activate-consumed-wa-btn` com `data.wa_me_url` se disponível
- [ ] 3.10 Implementar countdown 3s em `init()` para `ready_to_activate: true`: decrementar `#activate-countdown` via `setInterval`, redirecionar para `data.wa_me_url` ao chegar a 0; `#activate-wa-btn` permanece clicável antes do countdown terminar
- [ ] 3.11 Preencher `#activate-support-btn` com `data.support_url` nos estados de erro (e consumed se aplicável)
- [ ] 3.12 Atualizar `activate.spec.ts`: adicionar cenários para `expired`, `pending`, `not_found`, `consumed`, timeout, countdown e botão de suporte; atualizar cenário genérico de erro para verificar mensagem específica quando `reason` presente
- [ ] 3.13 Executar `pnpm playwright test` — todos os cenários passam
- [ ] 3.14 Verificar redirect: acessar `/activate?token=test` confirma redirect 301 para `/ativar?token=test`
- [ ] 3.15 Verificar que o logo usado é o asset oficial existente no repo (nenhum asset novo criado)
- [ ] 3.16 Commit semântico: `feat(activate): página bridge canonical, reason-aware errors e countdown de redirect`

## Detalhes de Implementação

Ver techspec seções:
- **Seção 9** (API contract) — JSON esperado por estado; `support_url` sempre presente
- **Seção 10** (`ativar.astro` e `activate.astro`) — flip canonical/redirect
- **Seção 11** (`activate.js`) — timeout, ERROR_MESSAGES, consumed state, countdown
- **Seção 12** (`activate.astro` HTML) — novos elementos `#activate-consumed`, `#activate-countdown`, `#activate-support-btn`, `#activate-error-detail`

### Mapa de mensagens de erro

```js
const ERROR_MESSAGES = {
  expired:   'Seu link de ativação expirou. Fale conosco pelo WhatsApp para receber um novo link.',
  pending:   'Seu pagamento ainda está sendo processado. Aguarde alguns minutos e tente novamente.',
  not_found: 'Link inválido. Verifique o link do email ou fale conosco pelo WhatsApp.',
};
// reason === 'consumed' → estado positivo, NÃO usa ERROR_MESSAGES
// reason ausente/desconhecido → fallback genérico existente
```

### Timeout fetch

```js
const controller = new AbortController();
const id = setTimeout(() => controller.abort(), 5000);
try {
  response = await fetch(url, { method: 'GET', headers: { Accept: 'application/json' }, signal: controller.signal });
} finally {
  clearTimeout(id);
}
// AbortError → showError('Não foi possível conectar ao servidor. Verifique sua conexão e tente novamente.')
```

### Countdown

```js
let remaining = 3;
const countEl = document.getElementById('activate-countdown');
if (countEl) countEl.textContent = String(remaining);
const interval = setInterval(() => {
  remaining -= 1;
  if (countEl) countEl.textContent = String(remaining);
  if (remaining <= 0) { clearInterval(interval); window.location.href = data.wa_me_url; }
}, 1000);
```

### `parseTokenState` — incluir `support_url` e `reason`

```js
const parseTokenState = (raw) => {
  if (!isRecord(raw)) return null;
  const ready = raw.ready_to_activate;
  if (typeof ready !== 'boolean') return null;
  const support = typeof raw.support_url === 'string' ? raw.support_url : '';
  if (ready) {
    const wa = raw.wa_me_url;
    const bot = raw.bot_number_display;
    if (typeof wa !== 'string' || typeof bot !== 'string') return null;
    const result = { ready_to_activate: true, wa_me_url: wa, bot_number_display: bot, support_url: support };
    if (typeof raw.telegram_deep_link === 'string') result.telegram_deep_link = raw.telegram_deep_link;
    return result;
  }
  const reason = typeof raw.reason === 'string' ? raw.reason : '';
  const waMe   = typeof raw.wa_me_url === 'string' ? raw.wa_me_url : '';
  const botD   = typeof raw.bot_number_display === 'string' ? raw.bot_number_display : '';
  return { ready_to_activate: false, reason, wa_me_url: waMe, bot_number_display: botD, support_url: support };
};
```

### Novos cenários E2E (`activate.spec.ts`)

| Cenário | Mock | Verificação |
|---|---|---|
| Token expirado | `{ ready_to_activate: false, reason: "expired", support_url: "..." }` | `#activate-error` visível, texto contém "expirou" |
| Token pendente | `{ ready_to_activate: false, reason: "pending", support_url: "..." }` | `#activate-error` visível, texto contém "processado" |
| Token inválido | `{ ready_to_activate: false, reason: "not_found", support_url: "..." }` | `#activate-error` visível, texto contém "inválido" |
| Conta já ativa | `{ ready_to_activate: false, reason: "consumed", wa_me_url: "...", support_url: "..." }` | `#activate-consumed` visível, `#activate-error` oculto, `#activate-consumed-wa-btn` com href correto |
| Timeout API | `route.abort()` após 5001ms | `#activate-error` visível, texto contém "conexão" |
| Countdown visível | `ready_to_activate: true` | `#activate-countdown` mostra "3" antes de terminar |
| Botão imediato | `ready_to_activate: true` | `#activate-wa-btn` visível antes do countdown terminar |
| Support button | qualquer erro com `support_url` | `#activate-support-btn` visível, `href === support_url` |
| Redirect 301 | GET /activate?token=test | resposta 301 com Location: /ativar?token=test |

## Critérios de Sucesso

- `pnpm playwright test` passa 100% incluindo todos os novos cenários.
- `/activate?token=test` → redirect 301 para `/ativar?token=test` verificado no teste E2E.
- `#activate-consumed` é exibido (não `#activate-error`) para estado `consumed`.
- `#activate-countdown` decrementa de 3 para 0 e redireciona para `wa_me_url`.
- `#activate-wa-btn` está visível imediatamente ao receber `ready_to_activate: true` (antes do countdown terminar).
- `#activate-support-btn` exibe `href === support_url` nos estados de erro.
- Zero asset de logo criado — verificar que nenhum arquivo de imagem foi adicionado ao repositório.
- Template HTML `ativar.astro` usa logo existente do repo sem modificação.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `semantic-commit` — tarefa encerra com commit semântico estruturado cobrindo a nova UX da página bridge e o flip de rota canonical

## Testes da Tarefa

- [ ] E2E `expired` — `#activate-error` visível, mensagem contém "expirou"
- [ ] E2E `pending` — `#activate-error` visível, mensagem contém "processado"
- [ ] E2E `not_found` — `#activate-error` visível, mensagem contém "inválido"
- [ ] E2E `consumed` — `#activate-consumed` visível, `#activate-error` oculto
- [ ] E2E `consumed` — `#activate-consumed-wa-btn` href == `wa_me_url` do mock
- [ ] E2E timeout — `#activate-error` visível com mensagem de conexão
- [ ] E2E countdown — `#activate-countdown` inicia em "3" e é visível durante contagem
- [ ] E2E botão imediato — `#activate-wa-btn` visível antes do countdown terminar
- [ ] E2E support button — `#activate-support-btn` href == `support_url` do mock
- [ ] E2E redirect 301 — `/activate?token=x` → `/ativar?token=x`
- [ ] `pnpm playwright test` — 100% pass

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

**Repositório:** `mecontrola-landingpage`

**Modificados:**
- `src/pages/ativar.astro` (recebe HTML completo — torna-se canonical)
- `src/pages/activate.astro` (vira redirect 301)
- `public/js/activate.js`
- `tests/playwright/activate.spec.ts`

**Referência (read-only — logo oficial):**
- `public/` — verificar assets de logo existentes antes de qualquer referência
