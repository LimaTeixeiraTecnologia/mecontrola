# Pre Go-Live Hardening — Runbook

## CORS

`CORS_ALLOWED_ORIGINS` deve ser configurado com origens explícitas antes do deploy em production.

O `Config.Validate()` bloqueia o boot quando `CORS_ALLOWED_ORIGINS` estiver vazio ou igual a `*` em production.
A função `resolveCORSOrigins` não adiciona fallback `*` — delega inteiramente ao valor configurado.

Validação pós-deploy:

```bash
curl -I -H "Origin: https://app.mecontrola.com.br" https://<host>/health \
  | grep -i "access-control-allow-origin"
```

Resultado esperado: header presente com a origem configurada, nunca `*`.

## Server Header

O devkit-go `securityHeadersMiddleware` já suprime o header `Server:` via `DefaultSecurityHeaders` que chama `header.Del("Server")` em cada resposta.

Nenhuma configuração adicional é necessária. Validação pós-deploy:

```bash
curl -I https://<host>/health | grep -i "^server:"
```

Resultado esperado: ausência total do header `Server:`.

Se o header aparecer, investigar proxy reverso (Caddy) — o `Server:` pode ser injetado pelo Caddy e não pelo app Go. Nesse caso, aplicar o Caddyfile hardening descrito em task 6.0 (B3).
