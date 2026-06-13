# Tarefa 6.0: B3 — Caddyfile hardening (TLS + headers + strip + admin block)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Versionar `deployment/compose/Caddyfile` (se ausente) com hardening completo de borda HTTP: TLS 1.2+ via ACME, security headers globais, strip de headers de gateway externos (defesa em profundidade ao B1), bloqueio de admin/debug endpoints. Pré-requisito para rollout do PRD gateway-auth-forensics.

<requirements>
- RF-06: headers globais — Strict-Transport-Security 31536000+includeSubDomains; X-Content-Type-Options nosniff; Referrer-Policy no-referrer; Permissions-Policy (); X-Frame-Options DENY
- RF-07: bloquear `/admin`, `/debug/pprof`, `/metrics` (404 ou 403) para origem externa
- RF-08: strip `X-User-ID`, `X-Gateway-Auth`, `X-Gateway-Timestamp` de origem externa
- RF-09: ACME email via `CADDY_EMAIL`; TLS 1.2+ (default)
- RF-10: smoke test scriptado valida headers + bloqueios
- Sem comentários soltos no Caddyfile que não agreguem informação operacional
</requirements>

## Subtarefas

- [ ] 6.1 Localizar `deployment/compose/Caddyfile` atual ou criar.
- [ ] 6.2 Configurar bloco global com headers de segurança (RF-06).
- [ ] 6.3 Adicionar matcher `@admin` cobrindo `/admin*`, `/debug/pprof*`, `/metrics*` com `respond 404`.
- [ ] 6.4 Adicionar `header_up -X-User-ID -X-Gateway-Auth -X-Gateway-Timestamp` no reverse_proxy para limpar de origem externa.
- [ ] 6.5 Criar `deployment/scripts/caddyfile-smoke.sh` que sobe Caddy local, faz `curl -I` em `/healthz`, `/debug/pprof`, `/metrics` e valida headers + status codes.
- [ ] 6.6 Atualizar `compose.yml` se necessário para garantir bind correto de Caddyfile.
- [ ] 6.7 Documentar em `docs/runbooks/caddyfile.md` decisões + procedimento de troca de domínio.

## Detalhes de Implementação

Ver techspec seção "Visão Geral dos Componentes > Caddyfile" + plano-fonte §5 B3. Coordenar rollout com tarefa 7.0 do PRD gateway-auth-forensics (cabeamento) para que o cutover seja atômico (Caddy + app + cliente LLM).

## Critérios de Sucesso

- `curl -I https://<staging>/healthz` mostra todos os 5 security headers.
- `curl -I https://<staging>/debug/pprof` → 404.
- `curl -I https://<staging>/metrics` externo → 404.
- `curl -H "X-User-ID: <uuid>" -X POST https://<staging>/api/v1/cards` → request chega no app SEM `X-User-ID` (defesa em profundidade ao B1).
- `bash deployment/scripts/caddyfile-smoke.sh` exit 0 em local.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Smoke local com Caddy em container
- [ ] Inspeção `curl -I` para cada rota crítica
- [ ] Confirmação de strip de headers externos via logs do app

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/compose/Caddyfile` (novo ou modificado)
- `deployment/compose/compose.yml` (modificado se necessário)
- `deployment/scripts/caddyfile-smoke.sh` (novo)
- `docs/runbooks/caddyfile.md` (novo)
