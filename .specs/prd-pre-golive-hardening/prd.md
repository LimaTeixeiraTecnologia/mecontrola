# Documento de Requisitos do Produto (PRD) — Pre Go-Live Hardening (itens diretos)

<!-- spec-version: 1 -->

<!--
Histórico de versões:
- v1 (2026-06-12): consolidação dos 8 itens diretos do plano-fonte `docs/planos/2026-06-11-auditoria-seguranca-pre-golive.md` que NÃO exigem techspec dedicada (B1+A5 estão em `.specs/prd-gateway-auth-forensics/`). Itens cobertos: B2 (timestamp WhatsApp), B3 (Caddyfile hardening), B4 (restore de backup), B5 (firewall VPS), B6 (CORS guard), B7 (rate limit WhatsApp), A2/A4 (headers globais), A10 (rate limit por user). Foco: MVP robusto, eficiente, econômico, production-ready/proof, sem falso positivo, inegociável. Skill `go-implementation` obrigatória em qualquer alteração Go.
-->

## Visão Geral

O plano de auditoria pré go-live (`docs/planos/2026-06-11-auditoria-seguranca-pre-golive.md`) enumera 13 itens (10 bloqueantes + 3 não-bloqueantes). O escopo crítico de identity/auth (B1 + A5) já tem PRD/techspec/8 ADRs próprios em `.specs/prd-gateway-auth-forensics/`. Este PRD recorta os **8 itens restantes**, agrupando-os por proximidade (4 de infraestrutura/runtime de produção + 4 de código Go enxuto) para entrega coordenada e auditável.

Não exigem techspec dedicada nem ADR — todas as decisões materiais estão cravadas no plano-fonte e foram aprovadas na análise crítica `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md` seção 3. Cada item tem ≤ 1 arquivo de alteração e teste óbvio.

### Itens cobertos

| ID | Origem | Severidade | Natureza |
|---|---|---|---|
| B2 | plano-fonte §5 | bloqueante | Go: timestamp WhatsApp anti-replay |
| B3 | plano-fonte §5 | bloqueante | Caddyfile: TLS + security headers + strip + admin block |
| B4 | plano-fonte §5 | bloqueante | Shell + runbook: restore de backup automatizado |
| B5 | plano-fonte §5 | bloqueante | Shell + runbook: ufw VPS firewall |
| B6 | plano-fonte §5 | bloqueante | Go: CORS guard em Config.Validate |
| B7 | plano-fonte §5 | bloqueante | Go: rate limit webhook WhatsApp |
| A2/A4 | plano-fonte §4 | médio | Go: hardening fallback CORS |
| A10 | plano-fonte §4 | médio | Go: rate limit por user_id em rotas autenticadas |

### Itens Fora de Escopo

- B1 (Gateway Auth) e A5 (forensics): cobertos em `.specs/prd-gateway-auth-forensics/`.
- Itens da seção 6 "Pós go-live" do plano-fonte (RLS Postgres, JWT, audit imutável, Redis, Vault, etc.): pós go-live, requer PRDs próprios.
- Item A5 da análise crítica (4 queries budgets sem user_id): já corrigido via skill `bugfix` em `.specs/prd-gateway-auth-forensics/bugfix_report.md`.

## Objetivos

- **OBJ-01**: fechar a superfície de ataque de webhook WhatsApp contra replay (timestamp) e DoS (rate limit) antes do go-live.
- **OBJ-02**: blindar a borda HTTP em produção via Caddy: TLS forte, security headers, bloqueio de admin endpoints, strip de headers de gateway.
- **OBJ-03**: garantir defesa em profundidade na borda do VPS via firewall `ufw` com regras explícitas e idempotentes.
- **OBJ-04**: assegurar que backup criptografado pode ser restaurado (backup que não restaura não é backup).
- **OBJ-05**: fechar CORS em `production` por validação de boot, eliminando wildcard silencioso.
- **OBJ-06**: aplicar rate limit por `user_id` em rotas autenticadas (defesa contra abuso de conta legítima).
- **OBJ-07**: nenhuma decisão de design pendente; todas cravadas no plano-fonte e validadas na análise crítica.
- **OBJ-08**: skill `go-implementation` carregada em toda alteração Go (R0–R7, R-ADAPTER-001). Zero comentário em `.go` de produção.

### Métricas de Sucesso

- **M-01** (B2): replay de webhook WhatsApp com timestamp > 5 min é descartado silenciosamente (200 OK) e registra `auth_events` com `reason="stale_webhook"`. Testes unitários cobrindo dentro/fora da janela, ausência de timestamp, formato inválido.
- **M-02** (B3): `curl -I https://api.mecontrola.com.br/healthz` retorna `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, `X-Frame-Options: DENY`, `Permissions-Policy: ()`. `curl -I .../debug/pprof` → 404. `curl -I .../metrics` externo → 404. Headers `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp` externos são strippados.
- **M-03** (B4): `deployment/scripts/pg-restore-smoke.sh` executa em staging baixando dump cifrado, restaurando e validando smoke queries. Cron mensal agendado.
- **M-04** (B5): `nmap` externo retorna apenas 22, 80, 443 abertos. SSH com senha desabilitado.
- **M-05** (B6): boot do app em `Environment=production` falha quando `CORS_ALLOWED_ORIGINS` está vazio ou contém `*`. Boot em `development` aceita qualquer config.
- **M-06** (B7): `hey -n 1000 -c 50` no webhook WhatsApp atinge 429 antes de saturar CPU. Configurável via `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN`, `_BURST`.
- **M-07** (A10): cliente autenticado excedendo limit por user retorna 429; métrica `auth_rate_limit_exceeded_total{scope="user"}` incrementa.
- **M-08** (A2/A4): fallback de CORS no Go nunca retorna `*` em `production`. Servidor não vaza header `Server:`.
- **M-09**: 0 (zero) comentário introduzido em `.go` de produção em PRs deste escopo.
- **M-10**: `task lint && task test && task vulncheck` verde em todos os PRs deste escopo.

## Histórias de Usuário

- **US-01 — Replay de webhook não causa duplicidade silenciosa**
  Como operador do mecontrola, quero que um payload Meta válido reenviado após 5 minutos seja descartado e auditado, para que retries fora de prazo não criem registros duplicados nem deem oráculo de tempo a atacante.

- **US-02 — Cliente externo não consegue acessar admin/metrics**
  Como responsável por segurança, quero que `/debug/pprof`, `/metrics`, `/admin` retornem 404 quando acessados pela Internet, para que ferramentas de fingerprinting não obtenham informação privilegiada.

- **US-03 — Backup é validado, não apenas gerado**
  Como operador do mecontrola, quero rodar um script automatizado que prove que o último dump criptografado pode ser restaurado em Postgres limpo, para que o backup tenha valor operacional real.

- **US-04 — Servidor não responde fora das portas necessárias**
  Como responsável por infraestrutura, quero que `ufw` no VPS Hostinger bloqueie tudo exceto 22/80/443, para que serviços internos (Postgres, métricas) não sejam alcançáveis externamente.

- **US-05 — Boot trava se CORS estiver inseguro**
  Como desenvolvedor, quero que o app falhe no boot em `production` se `CORS_ALLOWED_ORIGINS` estiver vazio ou for `*`, para que erro de configuração não se propague silenciosamente para produção.

- **US-06 — Webhook não vira vetor de DoS**
  Como operador, quero rate limit no `/api/v1/whatsapp`, para que atacante não force validações HMAC em loop saturando CPU.

- **US-07 — Cliente abusivo é freado mesmo dentro do limit de IP**
  Como operador, quero rate limit por `user_id` em rotas autenticadas, para que abuso de conta legítima não consuma quota global.

## Requisitos Funcionais

### B2 — Timestamp WhatsApp anti-replay

- **RF-01**: O sistema MUST extrair `entry[].changes[].value.messages[].timestamp` do payload Meta após validação HMAC e antes de `EstablishPrincipal`.
- **RF-02**: Se `|now - timestamp| > 5min`, o handler MUST retornar **200 OK silencioso** (Meta não dispara retry) e registrar evento `auth_events` com `reason="stale_webhook"`.
- **RF-03**: Se timestamp ausente ou formato inválido (`strconv.ParseInt` falha), o handler MUST retornar 200 OK + `reason="invalid_webhook_timestamp"`.
- **RF-04**: `time.Now().UTC()` inline; sem `Clock` interface (regra de memória).
- **RF-05**: Testes unitários cobrindo: dentro da janela, +6min, -6min, ausente, formato inválido.

### B3 — Caddyfile hardening

- **RF-06**: `deployment/compose/Caddyfile` (versionar se ausente) MUST aplicar globalmente:
  - `Strict-Transport-Security: max-age=31536000; includeSubDomains`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: no-referrer`
  - `Permissions-Policy: ()`
  - `X-Frame-Options: DENY`
- **RF-07**: Caddy MUST bloquear `/admin`, `/debug/pprof`, `/metrics` para origem externa (404 ou 403). Endpoints internos continuam acessíveis via rede `backend`.
- **RF-08**: Caddy MUST strip `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp` de qualquer request externo antes de proxy ao upstream Go (defesa em profundidade ao B1 do PRD gateway-auth).
- **RF-09**: ACME email via env `CADDY_EMAIL`. TLS 1.2+ (default Caddy).
- **RF-10**: Smoke test scriptado (`deployment/scripts/caddyfile-smoke.sh` ou inline no runbook) confirma headers via `curl -I` em ambiente local antes do deploy.

### B4 — Restore de backup

- **RF-11**: Criar `deployment/scripts/pg-restore-smoke.sh` que:
  1. Baixa último dump cifrado via `rclone`.
  2. Descriptografa com `age`.
  3. Sobe container Postgres efêmero (`docker run --rm postgres:<versão-pinada>`).
  4. Restaura dump.
  5. Executa smoke queries: `SELECT count(*) FROM mecontrola.users`, `SELECT count(*) FROM mecontrola.cards`, etc. (mínimo 3 tabelas críticas).
  6. Encerra container e limpa volume temporário.
- **RF-12**: Script idempotente, exit code 0 se restore OK, ≠0 se falhar com mensagem clara.
- **RF-13**: Runbook `docs/runbooks/backup-restore.md` documentando: pré-requisitos, execução manual, agendamento cron mensal em staging, critério de sucesso, troubleshooting.
- **RF-14**: Cron mensal em staging configurado e testado.

### B5 — Firewall VPS

- **RF-15**: Runbook `docs/runbooks/vps-bootstrap.md` documentando:
  - `ufw default deny incoming`
  - `ufw default allow outgoing`
  - `ufw allow 22/tcp` (SSH com chave; senha desabilitada via `PasswordAuthentication no` em `/etc/ssh/sshd_config`)
  - `ufw allow 80/tcp`, `ufw allow 443/tcp`
  - `ufw enable`
- **RF-16**: Script idempotente `deployment/scripts/vps-firewall.sh` que aplica e valida regras (re-executar não duplica).
- **RF-17**: Validação manual com `nmap` externo: apenas 22/80/443 abertos. Resultado documentado no runbook.

### B6 — CORS guard em Config.Validate

- **RF-18**: Em `configs/config.go` função `Config.Validate()`, em `Environment="production"`:
  - Se `cfg.HTTP.CORSAllowedOrigins` vazio: retornar erro `"CORS_ALLOWED_ORIGINS obrigatorio em production"`.
  - Se qualquer elemento `== "*"`: retornar erro `"CORS_ALLOWED_ORIGINS=* proibido em production"`.
- **RF-19**: `.env.example` MUST documentar formato esperado: `CORS_ALLOWED_ORIGINS=https://app.mecontrola.com.br,https://checkout.mecontrola.com.br`.
- **RF-20**: Testes unitários cobrindo 4 cenários: production vazio (erro), production com `*` (erro), production lista válida (ok), development qualquer config (ok).

### B7 — Rate limit webhook WhatsApp

- **RF-21**: Reusar middleware `internal/onboarding/.../middleware/rate_limit.go` aplicado em `composeWhatsAppWebhookRouter()` em `cmd/server/server.go`.
- **RF-22**: Configuração via novos envs:
  - `WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN` (default 600)
  - `WHATSAPP_WEBHOOK_RATE_LIMIT_BURST` (default 100)
- **RF-23**: Testes de integração: 429 antes do burst esgotar; reset após janela.
- **RF-24**: Opcional (documentado, não implementado neste PRD): whitelist de IPs públicos da Meta Cloud API para limiter mais frouxo.

### A2/A4 — Hardening fallback CORS + headers do servidor

- **RF-25**: Confirmar que `cmd/server/server.go` `resolveCORSOrigins()` nunca retorna `[]string{"*"}` em `production`. Se a função tem fallback, alterar para retornar erro de boot (delegando a B6).
- **RF-26**: Confirmar que `devkit-go` não injeta header `Server:` revelando versão. Se sim, override com header vazio em middleware global.

### A10 — Rate limit por user_id

- **RF-27**: Generalizar `internal/onboarding/.../middleware/rate_limit.go` (ou criar variante) para aceitar `keyExtractor func(*http.Request) string`. Hoje extrai IP; novo modo extrai `principal.UserID.String()` lendo do context.
- **RF-28**: Plugar limiter por `user_id` nas rotas autenticadas (cards, e quando budgets/categories/transactions adotarem `Principal`).
- **RF-29**: Envs novas: `AUTH_RATE_LIMIT_PER_USER_PER_MIN`, `AUTH_RATE_LIMIT_PER_USER_BURST`.
- **RF-30**: Testes unitários: extractor por IP continua funcionando; extractor por user_id retorna chave correta; sem Principal no context, fallback para IP.
- **RF-31**: Métrica Prometheus `auth_rate_limit_exceeded_total{scope}` com `scope` ∈ {`ip`, `user`} para observabilidade. Sem `user_id` como label (cardinalidade).

### Cross-Cutting

- **RF-32**: Toda alteração Go MUST carregar `.claude/skills/go-implementation/SKILL.md` (Etapas 1–5, R0–R7, R-ADAPTER-001). Zero comentário em `.go` de produção.
- **RF-33**: Toda alteração MUST passar `task lint && task test && task vulncheck` antes de merge.
- **RF-34**: Sem nova dependência externa em `go.mod`. Reuso de stdlib + libs já em uso.

## Riscos e Mitigações

- **R-01** (B2): janela de 5 min muito apertada gera 200 silencioso legítimo se relógio da VPS dessincronizar. Mitigação: NTP no host (já padrão no Hostinger); alerta operacional se `reason="stale_webhook"` > 1% em 10 min.
- **R-02** (B3): bloqueio de `/metrics` no Caddy quebra scraping do Prometheus se este estiver fora da rede `backend`. Mitigação: validar topologia antes do deploy; documentar no runbook.
- **R-03** (B4): script de restore exige `rclone` + `age` instalados no host de execução. Mitigação: documentar pré-requisitos no runbook.
- **R-04** (B5): erro na regra ufw deixa SSH sem acesso. Mitigação: script só altera com `--force` explícito; runbook recomenda manter sessão SSH aberta para rollback.
- **R-05** (B6): boot fail bloqueia deploy se `.env` em produção ainda tem `*`. Mitigação: deploy gating: aplicar B6 com `.env` correto pronto antes do deploy.
- **R-06** (B7): limit muito baixo bloqueia tráfego legítimo da Meta. Mitigação: defaults conservadores + métrica de 429 observada após deploy.
- **R-07** (A10): chave user_id no rate-limit precisa ler context após `InjectPrincipalFromHeader`. Mitigação: ordem na chain documentada e gate de revisão.

## Critério de Aceitação Global

- Todos os 34 RFs implementados e validados pelas 10 métricas M-01–M-10.
- Plano de rollout para B3+B5 documentado em runbook único.
- `nmap` externo + `curl -I` evidenciam M-02 e M-04 em screenshot anexado ao PR de deploy.
- `task lint && task test && task vulncheck` verde em todos os PRs do escopo.
- Sem nova dependência em `go.mod`.
