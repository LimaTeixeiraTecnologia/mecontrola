# ADR-009 — Deployment em VPS Hostinger com reverse proxy + cron fixo em UTC

## Metadados

- **Título:** Topologia de deployment, extração de IP real e schedule fixo de jobs
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §6.3, §6.4, §8.4, RF-02, RF-09, RF-11

## Contexto

A landing roda em Cloudflare Pages, mas o backend Go (server + worker) é hospedado em VPS Hostinger. Implicações:
- Sem `CF-Connecting-IP` (header exclusivo Cloudflare).
- TLS termina no servidor da VPS (não no edge).
- Sem proteção DDoS edge nativa — rate limit defensivo + firewall da VPS são a única camada.
- Reinícios do binário (deploys, manutenção) podem ocorrer em qualquer hora; cron `@every Nh` do `robfig/cron/v3` conta a partir do startup, gerando schedule não determinístico.

Requisitos a cobrir:
1. RF-02: rate limit 10/min por IP **real** — não pode ser o IP do reverse proxy.
2. RF-09 / RF-11: cadências horária e diária previsíveis para operação.
3. Headers de IP forjados por cliente malicioso devem ser ignorados.

## Decisão

### 1. Topologia
- **Reverse proxy:** `nginx` ou `caddy` (decisão operacional, ambos suportam o contrato).
- **TLS:** terminado no proxy via Let's Encrypt (renovação automática).
- **Backend Go:** binário escuta em `127.0.0.1:8080` (loopback only). `cmd/server` e `cmd/worker` em processos separados (systemd units).
- **Banco:** Postgres na mesma VPS ou managed (Hostinger DB). Conexão via DSN, não exposta externamente.

### 2. Extração de IP real
Middleware lê em ordem:
1. `X-Real-IP` (single value injetado pelo proxy).
2. Primeiro IP de `X-Forwarded-For` (`split(",")[0].trim()`).
3. `r.RemoteAddr` (host part).

Headers em (1) e (2) são respeitados **somente** se `r.RemoteAddr` pertence a `OnboardingConfig.TrustedProxies` (default `127.0.0.1/32`, `::1/128`). Configuração:

```yaml
ONBOARDING_TRUSTED_PROXIES: "127.0.0.1/32,::1/128"
```

Requisições de IP não confiável caem para `RemoteAddr` direto, mesmo se houver `X-Forwarded-For`. Defesa contra spoofing.

### 3. Schedule fixo de jobs
Substituir `@every Nh` por expressões cron deterministicas em UTC:

| Job | Schedule (UTC) | Horário BRT |
| --- | --- | --- |
| `OutreachJob` | `5 * * * *` | 5 min após hora cheia |
| `TokenExpirationJob` | `0 3 * * *` | 00:00 BRT |
| `MetaProcessedMessagesCleanup` | `30 3 * * *` | 00:30 BRT |

Reinícios em qualquer hora **não** disparam ticks extras (robfig/cron com cron expression espera o próximo tick agendado). Múltiplas instâncias por engano são **seguras** porque todas as operações são idempotentes (locking otimista + `FOR UPDATE SKIP LOCKED`).

### 4. Configuração systemd
Cada processo como unit:
- `mecontrola-server.service` — `cmd/server` + `Restart=always` + `User=mecontrola` + `EnvironmentFile=/etc/mecontrola/server.env`.
- `mecontrola-worker.service` — `cmd/worker` + idem.
- `mecontrola-server.service` depende de `network-online.target` e `postgresql.service` (ou `mecontrola-db-tunnel` se DB managed).

## Alternativas Consideradas

1. **Cloudflare Tunnel (`cloudflared`) para o backend.** Recusada — adiciona dependência operacional; ganho de DDoS edge não justifica para MVP; user escolheu VPS pura.
2. **Backend exposto diretamente sem reverse proxy.** Recusada — sem terminação TLS gerenciada; ACME no Go (`autocert`) requer porta 80 livre e cria acoplamento desnecessário ao binário; rotação difícil.
3. **Cron externo (`cron` do sistema operacional).** Recusada — perde observabilidade (sem `slog` integrado), perde idempotência por timer, complica deploy (script + binário).
4. **`@every Nh` do robfig/cron.** Recusada — schedule não determinístico após reinícios.

## Consequências

### Benefícios
- TLS gerenciado fora do binário (Let's Encrypt no proxy).
- IP real seguro contra spoofing.
- Schedule previsível facilita alertas baseados em horário esperado.
- Processos separados (server vs worker) permitem reiniciar um sem impactar o outro.

### Trade-offs
- Operação manual de provisioning na VPS (não é IaC nativo). Mitigação: documento de runbook fora desta techspec.
- Sem DDoS edge — rate limit + firewall da VPS são a única defesa. Aceitável para MVP de baixo volume; reavaliação após crescimento.
- TLS renewal depende de cron do nginx/caddy (não da aplicação). Monitor externo do certificado.

### Riscos e Mitigações
- **R:** Proxy mal configurado não injeta `X-Real-IP` → todo mundo cai como IP loopback (1 limiter compartilhado) e dispara 429 cedo. **M:** Health check de smoke valida que `X-Real-IP` chega no app; alerta se taxa de 429 sobe inesperadamente.
- **R:** `TrustedProxies` mal preenchido aceita IP de cliente como confiável → cliente envia `X-Real-IP: 1.2.3.4` falso. **M:** Default seguro (`127.0.0.1` only); documentação clara; revisão de config no PR.
- **R:** Worker e server em VPS travam simultaneamente. **M:** Outbox garante eventual consistency; restart automático via systemd; healthcheck Postgres remoto detecta queda.
- **R:** Cron fixo `0 3 * * *` coincide com janela de backup do Postgres. **M:** Horários escolhidos fora da janela típica de manutenção brasileira (00:00 BRT é horário de baixo tráfego).

## Plano de Implementação
1. `OnboardingConfig.TrustedProxies` carregado do env, parseado para `[]netip.Prefix`.
2. Helper `realClientIP(r *http.Request, trusted []netip.Prefix) string` em `internal/onboarding/infrastructure/http/server/middleware/`.
3. Test unitário cobrindo: proxy confiável + `X-Real-IP`; proxy confiável + `X-Forwarded-For`; proxy não confiável (ignorar headers); ambos ausentes.
4. Jobs em `internal/onboarding/infrastructure/jobs/handlers/` declaram `Schedule()` retornando cron expr.
5. Documento de provisioning VPS (runbook) — fora desta techspec, criar em `docs/runbooks/deployment-vps-hostinger.md` em tarefa separada.

## Monitoramento
- Alerta se nenhum tick de `OutreachJob` rodar em uma janela de 70 min (esperado 60 min).
- Alerta se nenhum tick de `TokenExpirationJob` rodar em uma janela de 26h.
- Métrica `onboarding_checkout_rate_limited_total` por IP (top 10) para detectar abuso ou misconfig.
