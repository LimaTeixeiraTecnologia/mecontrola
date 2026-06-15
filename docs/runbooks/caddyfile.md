# Runbook: Caddyfile Hardening

## Visão Geral

O Caddyfile em `deployment/caddy/Caddyfile` é o ponto único de borda HTTP para o mecontrola em produção. Ele provê:

- TLS automático via ACME (Let's Encrypt) — TLS 1.2+ por padrão no Caddy 2.x.
- Security headers globais em todas as respostas.
- Bloqueio de endpoints administrativos para origem externa.
- Strip de headers de gateway para defesa em profundidade (B1 do PRD gateway-auth-forensics).

## Variáveis de Ambiente Obrigatórias

| Variável | Descrição | Exemplo |
|---|---|---|
| `CADDY_EMAIL` | Email para registro ACME / Let's Encrypt | `ops@mecontrola.com.br` |
| `APP_DOMAIN` | Domínio público do app | `api.mecontrola.com.br` |
| `PORT` | Porta do serviço Go upstream | `8080` (default) |

## Headers de Segurança Aplicados

| Header | Valor | Referência |
|---|---|---|
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | HSTS — força HTTPS por 1 ano |
| `X-Content-Type-Options` | `nosniff` | Previne MIME sniffing |
| `Referrer-Policy` | `no-referrer` | Não vaza URL de origem em requests externos |
| `Permissions-Policy` | `camera=(), microphone=(), geolocation=()` | Desabilita camera, microphone e geolocation |
| `X-Frame-Options` | `DENY` | Previne clickjacking |
| `Content-Security-Policy` | `default-src 'none'; connect-src 'self'; frame-ancestors 'none'` | Política restritiva padrão |
| `Server` | *(removido)* | Não revela stack |

## Endpoints Bloqueados Externamente (RF-07)

`/admin*`, `/debug/pprof*`, `/metrics*` retornam **404** para qualquer request externo.

Atenção: se o Prometheus scrape estiver fora da rede `backend`, o scraping de `/metrics` será bloqueado. O Prometheus deve estar na mesma rede Docker `frontend` ou `backend` e acessar o app diretamente (não via Caddy). Verificar topologia antes do deploy.

## Strip de Headers de Gateway (RF-08)

Os seguintes headers são removidos de qualquer request externo antes de proxy ao Go:

- `X-User-ID`
- `X-Gateway-Auth`
- `X-Gateway-Timestamp`

Isso garante que mesmo que um atacante injete esses headers, o app nunca os receberá pela borda pública. O enforce definitivo é feito pela middleware `RequireGatewayAuth` (B1 — PRD gateway-auth-forensics).

## Decisão: Authorization passa pelo gateway

`X-User-ID`, `X-Gateway-Auth` e `X-Gateway-Timestamp` são removidos para evitar
injeção externa (RF-08). `Authorization` é PRESERVADO pois a aplicação valida
o token (Bearer/JWT) no middleware `require_gateway_auth`. Externos que injetem
`Authorization` inválido caem em 401 normalmente.

## Validação TLS em produção

O smoke local roda com `auto_https off` para isolar testes de header/strip.
Validação TLS deve ser executada manualmente após deploy em produção:

```bash
openssl s_client -connect <APP_DOMAIN>:443 -servername <APP_DOMAIN> </dev/null \
    | openssl x509 -noout -issuer -dates
```

Critérios de aceite:
- Cadeia válida emitida por Let's Encrypt (ou ZeroSSL).
- TLS 1.2 ou superior negociado.
- HSTS efetivo via curl: `curl -sI https://<APP_DOMAIN>/ | grep -i strict-transport-security`.

Registrar saída no runbook após cada deploy.

## Smoke Test Local

```bash
bash deployment/scripts/caddyfile-smoke.sh
```

O script:
1. Sobe um container nginx stub como upstream.
2. Sobe Caddy com o Caddyfile real em modo HTTP local.
3. Verifica todos os 5 security headers.
4. Verifica 404 em `/admin`, `/debug/pprof`, `/metrics`.
5. Verifica strip dos 3 headers de gateway.
6. Exit 0 se todos os checks passam.

Pré-requisito: Docker disponível localmente.

## Procedimento de Troca de Domínio

1. Atualizar `APP_DOMAIN` no `.env` de produção.
2. Verificar que `CADDY_EMAIL` está configurado corretamente (usado para registro ACME).
3. Parar o serviço Caddy: `docker compose stop caddy`.
4. Remover dados TLS antigos se necessário: `docker volume rm mecontrola_caddy-data` (perda de certificado — será re-emitido automaticamente).
5. Subir Caddy: `docker compose up -d caddy`.
6. Verificar logs: `docker compose logs -f caddy` — aguardar emissão do certificado.
7. Validar: `curl -I https://<novo-domínio>/healthz` e confirmar headers presentes.

## Cutover Coordenado com gateway-auth-forensics

O deploy do Caddyfile (strip de headers) deve ser feito junto com o deploy do Pacote A (gateway-auth-forensics — tarefa 7.0), para que:

1. O strip de `X-User-ID`/`X-Gateway-Auth` no Caddy e o enforcement de `RequireGatewayAuth` no app aconteçam atomicamente.
2. O token HMAC emitido pelo cliente (LLM ou frontend) esteja presente desde o primeiro request pós-cutover.

Sequência recomendada:
1. Deploy do app com `RequireGatewayAuth` em modo `audit` (log only, não rejeita).
2. Deploy do Caddyfile hardened (strip de headers externos).
3. Validar smoke: `curl -I https://api.mecontrola.com.br/healthz`.
4. Ativar `RequireGatewayAuth` em modo `enforce`.

## Troubleshooting

| Sintoma | Causa provável | Ação |
|---|---|---|
| Certificado não emitido | `CADDY_EMAIL` não configurado ou domínio sem DNS | Verificar `.env` e DNS |
| `/metrics` retorna 404 no Prometheus | Prometheus acessa via Caddy | Mover scrape para acesso direto ao app por rede interna |
| Headers ausentes na resposta | Caddy não recarregou config | `docker compose exec caddy caddy reload --config /etc/caddy/Caddyfile` |
| Strip não funcionando | Versão antiga do Caddy | Verificar `caddy:2-alpine` — mínimo Caddy 2.4 |
