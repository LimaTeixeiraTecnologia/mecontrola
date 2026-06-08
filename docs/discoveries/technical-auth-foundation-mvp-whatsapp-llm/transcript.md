# Transcript do Discovery Técnico

## Contexto Inicial
- Tema: fundação de autenticação e autorização do MeControla para o MVP, com WhatsApp + LLM in-process como interface inicial.
- Entrada: bundle aprovado em `docs/discoveries/brainstorms/brainstorm-autenticacao-autorizacao-mvp-whatsapp/` (variante D' + 4 locks operacionais).
- Restrições inegociáveis confirmadas: Go (versão de `go.mod`), VPS Hostinger sem KMS, Postgres único storage, outbox obrigatório, zero dependência externa nova, MVP production-ready, LGPD.
- Skill obrigatória downstream: `go-implementation` com exemplos carregados sob demanda.
- Vetores de ameaça priorizados: cross-tenant leak, spoofing Meta, token leak, replay/hijack via LLM tool.
- Análise prévia do codebase identificou primitivos reusáveis em onboarding: `meta_signature.go` (HMAC SHA-256 current+next), `raw_body_buffer.go`, `rate_limit.go`, `whatsapp_inbound_handler.go` (parse + dedup WAMID), `find_user_by_whatsapp` em identity, `outbox.Publisher` completo, padrão de PK UUID em tabelas de domínio.

## Rodada 1
**P1.** Onde vive o webhook pós-ativação?
- Opções: estender handler de onboarding; novo módulo `internal/agent`; pacote compartilhado `internal/platform/whatsapp` + dispatcher único.
- **Resposta:** Pacote compartilhado `internal/platform/whatsapp` + dispatcher único.

**P2.** Pacote do contrato `auth.Principal`?
- Opções: `internal/identity/application/auth`; `internal/platform/auth`; `internal/auth` (novo BC).
- **Resposta:** `internal/identity/application/auth`.

**P3.** Granularidade do contrato `auth.Principal`?
- Opções: tipo concreto minimal; tipo concreto + session anexa; interface.
- **Resposta:** Tipo concreto minimal (`{UserID uuid.UUID; Source string}`, imutável por valor, helpers `WithPrincipal`/`FromContext`).

**P4.** Critérios do middleware `RequireUser` (multi-select)?
- **Resposta:** 401 imediato se Principal ausente + linter custom proibindo handler ler `X-User-ID`. Rejeitados: injeção automática em log/span, deny-list de revogação ativa.

## Rodada 2
**P1.** Schema de `auth_events`?
- **Resposta:** Compacto (5 colunas mínimas) seguindo o padrão de PK UUID das outras tabelas (confirmado em `users`: `id UUID + CONSTRAINT users_pkey PRIMARY KEY (id) + created_at`). Schema final: `id UUID`, `occurred_at TIMESTAMPTZ`, `user_id UUID NULL`, `kind TEXT`, `source TEXT` + índices `(user_id, occurred_at DESC) WHERE user_id IS NOT NULL` e `(occurred_at DESC) WHERE kind='failed'`.

**P2.** Cache `wa_id → user_id`?
- Opções: LRU 10k+TTL 60s; map TTL; sem cache.
- **Resposta:** Sem cache no MVP, query direta. Aceito risco de p99 sob pico com gatilho de revisão em métrica.

**P3.** Volumetria-alvo?
- Opções: conservadora <500; mediana 500-5000; agressiva 5k-50k.
- **Resposta:** Mediana 500-5000 usuários ativos, pico ~500 msg/min, p99 webhook <300ms, SLO 99.5%.

**P4.** Integração LLM dispatcher ⇄ Principal?
- Opções: dispatcher lê ctx e passa para tool via parâmetro Go; tools com user_id no construtor; LLM recebe user_id no prompt (anti-pattern).
- **Resposta:** Dispatcher lê Principal de `ctx` e passa para tools como parâmetro de função Go. Anti-pattern explicitamente rejeitado.

## Rodada 3
**P1.** Tensão "mediana sem cache" vs p99?
- Opções: aceito risco com gatilho de revisão; adicionar cache LRU; reduzir volumetria.
- **Resposta:** Aceito risco com gatilho de revisão — métrica `auth_resolve_wa_duration_seconds` p99 com alerta em 100ms; se sustentado por 3 dias, abre PR de cache LRU (código desenhado mas não ativado).

**P2.** Rotação `app_secret` Meta (multi-select)?
- **Resposta:** Documentar runbook `docs/runbooks/auth-meta-secret-rotation.md`. Rejeitados: alerta automático em rotated>24h, rejeitar X-Hub-Signature ausente como ação extra (já é coberto pelo middleware existente), replay protection por timestamp (Meta não envia header oficial).

**P3.** Estratégia rate-limit?
- Opções: token bucket por user com sync.Map + cleanup; reusar rate_limit.go existente; sliding window.
- **Resposta:** Token bucket por `user_id` com `sync.Map` + goroutine cleanup TTL 5min, bucket 60 tokens + refill 1/seg + burst 60. Excedente publica `auth.failed{reason='rate_limited'}` e retorna 200 OK ao Meta (evita retry storm).

**P4.** Rollout/rollback (multi-select)?
- **Resposta:** Nada criado em produção ainda — rollout limpo. Rejeitados: feature flag, parallel run com X-User-ID, smoke test obrigatório (será smoke test no PR de implementação, não como gate de rollout). Migration down preserva via rename (regra padrão).

## Decisões Registradas
- D-001: Variante D' confirmada — `auth.Principal` in-process via `context.Context` + ADR documentando boundary HTTP futura.
- D-002: Pacote `auth` reside em `internal/identity/application/auth` como sub-pacote.
- D-003: `Principal` é struct concreta minimal por valor `{UserID uuid.UUID, Source PrincipalSource}`.
- D-004: `RequireUser` retorna 401 com `{"message":"unauthorized"}`, sem fallback, sem revelar motivo, sem injeção automática em log/span.
- D-005: Linter custom (depguard + forbidigo) bloqueia handler ler `X-User-ID` e proíbe `os.Getenv` direto para secrets de auth.
- D-006: Webhook único em `internal/platform/whatsapp` com dispatcher que roteia entre onboarding (pré-ativação) e agent (pós-ativação com Principal).
- D-007: Extração via Strangler Fig do `meta_signature`, `raw_body_buffer`, parse Meta, dedup WAMID — onboarding continua funcional durante migração.
- D-008: Tabela `auth_events` com schema compacto (5 colunas) e PK UUID conforme padrão. Retenção mínima 90 dias.
- D-009: Eventos via `outbox.Publisher` na mesma TX do `EstablishPrincipal`: `auth.principal_established`, `auth.failed{reason}`, `auth.unknown_user`. Consumer `auth_events_consumer` projeta para `auth_events`. Idempotência por `event_id`.
- D-010: Sem cache `wa_id → user_id` no MVP. Gatilho de revisão: p99 `auth_resolve_wa_duration_seconds` > 100ms por 3 dias consecutivos.
- D-011: Volumetria-alvo: 500–5000 usuários ativos, pico ~500 msg/min, p99 webhook < 300ms, SLO 99.5%.
- D-012: Rate-limit token bucket por `user_id` com `sync.Map` + goroutine cleanup TTL 5min, cancelável via `ctx` do shutdown cooperativo.
- D-013: Rotação `META_APP_SECRET` via runbook `docs/runbooks/auth-meta-secret-rotation.md` aproveitando suporte `current+next` já existente em `meta_signature.go`.
- D-014: Métricas OTel com cardinalidade controlada — `user_id` JAMAIS é label. PII mascarada via VO `WhatsAppNumber.Masked()` em todos os logs e spans.
- D-015: LLM tools recebem `ctx` com Principal e leem `user_id` exclusivamente de `auth.FromContext(ctx)`. Args do LLM são DTOs puros de negócio sem `user_id`.
- D-016: Rollout sem feature flag e sem parallel run — nada em produção depende ainda. Migration down preserva `auth_events` via rename, nunca DROP.
- D-017: 5 épicos planejados conforme decomposição no `discovery.md`.
- D-018: Itens em aberto registrados em `## Itens em Aberto` do `discovery.md` (teto de retenção de `auth_events`, nome final do diretório `internal/platform/whatsapp`, validação empírica de 60 msg/min, estratégia de anonimização user_id em delete, premissa LLM in-process, nome de `internal/agent`).
