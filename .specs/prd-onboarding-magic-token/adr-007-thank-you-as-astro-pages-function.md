# ADR-007 — Thank-you page como Astro Pages Function na landing + endpoint state JSON com resposta boolean única

## Metadados

- **Título:** Hospedagem e renderização da thank-you page (RF-04, RF-05, RF-17, RF-19)
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.2, §6.5, RF-04, RF-05, RF-17, RF-19, S-02, S-15

## Contexto

O PRD exige (RF-04) thank-you sob domínio MeControla, com botão `wa.me?text=ATIVAR+<token>` pré-preenchido e fallback de copy-paste visível, com auto-redirect mobile (RF-05). RF-17 exige defesa contra enumeração: mensagem única "Link inválido ou expirado" para qualquer estado interno inválido. RF-19 exige WCAG 2.1 AA.

A landing `mecontrola-landingpage` é Astro 5 + Tailwind v4 em Cloudflare Pages. CSP atual restritiva (`connect-src 'self' https://www.google-analytics.com ...`).

Opções:
1. **Astro Pages Function (SSR no edge) na landing** — rota `/obrigado/[token]` server-side faz fetch do estado e renderiza HTML.
2. **Endpoint Go `GET /obrigado?s={token}` em `cmd/server`** — `html/template` stdlib.
3. **SPA estática + JS fetch ao backend** — atualizar CSP da landing.

Defesa contra oracle (RF-17): o endpoint backend não pode vazar o motivo da falha no contrato HTTP.

## Decisão

### 1. Rota Astro com Pages Function
Rota nova no repo da landing: `src/pages/obrigado/[token]/index.astro` rodando como Cloudflare Pages Function (SSR no edge runtime). Pages Function faz fetch server-side:

```js
const response = await fetch(`${API_BASE}/v1/onboarding/tokens/${token}/state`, { method:'GET' });
const data = await response.json();
```

Resposta normalizada em **um único shape**:

```json
{
  "ready_to_activate": true|false,
  "wa_me_url": "https://wa.me/...?text=ATIVAR+...",     // só quando true
  "bot_number_display": "+55 11 9XXXX-XXXX"             // só quando true
}
```

Quando `ready_to_activate=false`, omite `wa_me_url` e `bot_number_display`. Renderização Astro:

```astro
{ready ? (
  <CTAActivate waUrl={waMeUrl} botDisplay={botNumberDisplay} token={token} />
) : (
  <ErrorBlock message="Link inválido ou expirado. Fale com nosso suporte." />
)}
```

### 2. Endpoint state JSON (RF-17 defensivo)
`GET /v1/onboarding/tokens/{token}/state` no Go server:
- Sempre `200 OK` (mesmo quando token inválido) com `ready_to_activate=false` — evita `404 vs 200` oracle.
- Cabeçalho `Cache-Control: no-store`.
- Sleep aleatório `0..3ms` quando `ready=false` (normaliza timing).
- Métrica server-side `ty_page_invalid_access_total{reason}` com motivo real (`not_found|pending|expired|consumed`) — **nunca** vai para a resposta.

### 3. Acessibilidade (RF-19)
- Botão CTA real `<a href="wa_me_url" role="button" aria-label="Abrir WhatsApp para ativar conta">`.
- Bloco fallback semântico: `<section aria-label="Ativação manual"><p>Caso o WhatsApp não abra, envie no número ... a mensagem ATIVAR {token}</p><CopyButton text="ATIVAR {token}"/></section>`.
- `<meta http-equiv="refresh" content="0.8; url={wa_me_url}">` renderizado **apenas em mobile** via UA hint server-side (no SSR da Pages Function); desktop não recebe (RF-05).
- `<noscript>` mantém instrução fallback visível.
- Contraste validado via tokens Tailwind (cores existing da landing — auditoria axe-core no pipeline da landing).
- Foco visível: `:focus-visible` no botão e na ação copy.

### 4. Configuração CSP
Atualizar CSP do `_headers` do Cloudflare Pages para incluir `connect-src` com `https://api.mecontrola.app.br` (mesmo que o fetch seja server-side, a Function compartilha CSP do site final). Trabalho coordenado em PR no repo da landing.

### 5. Roteamento Kiwify
Painel Kiwify configurado para redirect pós-pagamento em `https://www.mecontrola.app.br/obrigado/{tracking_sck}`. Fallback: Pages Function aceita também `?s={token}` na query, caso o painel não suporte substituição de placeholder no path.

## Alternativas Consideradas

1. **Endpoint Go `html/template`.** Recusada — divergente do design system Tailwind da landing; replicação de visual; superfície do server cresce; recriar componentes de UI já existentes (footer, header) é débito desnecessário.
2. **SPA estática + JS fetch.** Recusada — depende de JS habilitado (impacta WCAG e UX em browsers antigos); risco de flash de conteúdo errado durante carregamento; CSP precisa abrir `connect-src` para o backend (a versão SSR também precisa, mas o impacto de oracle é menor — fetch sem CORS server-side).

## Consequências

### Benefícios
- Reuso do design system, componentes Tailwind, header/footer da landing.
- CSP simétrica e controlada.
- SSR no edge garante TTFB baixo (Cloudflare PoP).
- Defesa contra oracle bem isolada (endpoint Go).

### Trade-offs
- Dois deploys coordenados (backend Go + landing Astro). Mitigação: contrato JSON estável documentado aqui.
- Pages Function consome unidades de execução do Cloudflare Workers (custo marginal).
- UA hint para meta-refresh mobile é heurístico (não tem 100% precisão em UA spoofing). Mitigação: `<noscript>` + botão manual cobrem o caso.

### Riscos e Mitigações
- **R:** Painel Kiwify não suporta placeholder no path. **M:** Fallback `?s={token}` na query.
- **R:** CSP da landing impede fetch server-side. **M:** Pages Function roda no servidor, CSP é aplicada ao HTML resultante, não à Function — verificar com smoke test.
- **R:** Componente CopyButton não funciona em iOS Safari antigo. **M:** Fallback `<input type="text" readonly>` selecionável.
- **R:** Cliente compartilha link `/obrigado/<token>` em rede social → token vaza. **M:** Comando `ATIVAR` é idempotente; reuso por outro número emite signal (RF-15) e bloqueia; risco aceito.

## Plano de Implementação
1. Endpoint Go `GET /v1/onboarding/tokens/{token}/state` com use case `GetTokenState`.
2. Pages Function Astro `src/pages/obrigado/[token]/index.astro` no repo da landing (PR separado).
3. Atualização CSP `_headers` no repo da landing.
4. Componentes Astro `<CTAActivate>` e `<ErrorBlock>` (no repo da landing).
5. Smoke test E2E: simular request à Pages Function → resposta correta → render esperado.
6. Auditoria axe-core no pipeline da landing.

## Monitoramento
- `ty_page_invalid_access_total{reason}` (server-side).
- Métrica Cloudflare Pages Function (latência, erros) — fora deste repo.
