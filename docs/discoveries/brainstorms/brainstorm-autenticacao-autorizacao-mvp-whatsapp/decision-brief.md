# DECISION BRIEF — Autenticação e Autorização do MeControla (MVP WhatsApp + LLM)

## Problema
O MeControla precisa entregar três módulos de domínio inegociáveis (`internal/card`, `internal/categories`, `internal/budgets`) cujos PRDs declaram um middleware `RequireUser` **transitório** baseado em header `X-User-ID` cru — sem qualquer autenticação real. Sem um módulo de autenticação e autorização, qualquer cliente HTTP pode se passar por qualquer usuário simplesmente trocando o header, violando o requisito de negócio inegociável de que "usuário é dono das suas informações e nenhum outro usuário pode vê-las". A interface inicial é WhatsApp + LLM (in-process), mas a arquitetura precisa ser fundação para app móvel e web sem refactor de domínio.

## Objetivo
Entregar a fundação de autenticação e autorização em camadas, com o MVP cirúrgico implementando apenas a camada WhatsApp + LLM in-process, e documentando o contrato para que app/web futuros plughem sem alterar usecases ou domínio. Critério de sucesso preliminar: card/categories/budgets podem implementar AuthZ definitivo (substituindo o `X-User-ID` transitório) sem nova rodada de decisão arquitetural, e qualquer tentativa de cross-tenant read/write retorna 401/403 com auditoria registrada.

## Escopo Inicial
Inclui:
- Validador HMAC do webhook Meta (`X-Hub-Signature-256`) + verify_token.
- Resolver `wa_id → user_id` no webhook via `Identity.FindUserByWhatsApp` + cache TTL curto (60s).
- Tipo `auth.Principal{UserID, Source, ...}` e helpers `auth.WithPrincipal(ctx, p)` / `auth.FromContext(ctx)`.
- Middleware `RequireUser` que lê Principal do `context.Context` e retorna 401 quando ausente.
- Eventos `auth.principal_established`, `auth.failed`, `auth.unknown_user` via `outbox.Publisher`.
- Audit projector → tabela `auth_events(user_id, kind, source, ts, ip_masked)` com retenção mínima de 90 dias.
- Rate-limit token bucket in-memory por `user_id` (60 msg/min default, configurável).
- ADR documentando o contrato de boundary HTTP futura (JWT Ed25519 + refresh quando app/web chegarem).
- Substituição do `X-User-ID` transitório do PRD card pelo novo `RequireUser` antes da implementação de card.

Exclui:
- Login web e fluxos UI (PRD futuro).
- JWT, JWKs, kid rotation, refresh token (PRD futuro).
- Sessões persistidas em Postgres (`auth_sessions`).
- Provedores OIDC externos (Auth0, Clerk, Keycloak, Supabase).
- mTLS interno entre componentes.
- Rate-limit por IP global (foco no MVP é por user_id).
- RBAC e autorização baseada em papéis (todos os usuários têm o mesmo papel hoje).

## Restrições
- Deploy permanece em VPS Hostinger (ADR-009 onboarding); sem KMS gerenciado.
- Postgres é o único storage; sem Redis/Memcached/Vault.
- `outbox.Publisher` é obrigatório para eventos de auth com garantia transacional.
- Zero dependência externa nova; apenas stdlib + libs Go já no `go.mod`.
- LGPD: PII mascarada em logs (R7); audit log com retenção ≥ 90 dias; revogação de sessões ao marcar usuário deletado.
- R0–R7 da skill `go-implementation` (sem `init()`, sem `panic` em produção, `context.Context` em fronteiras de IO, `errors.Join`/`%w` para erros, goroutines canceláveis).
- Padrão Obrigatório de Módulo do `AGENTS.md` para `internal/identity` (escopo onde a fundação de auth provavelmente residirá).

## Hipóteses
- LLM + WhatsApp bridge rodam in-process com o backend Go (H1 — confirmada por arquitetura monolítica modular).
- `internal/identity` já expõe `FindUserByWhatsApp` apto a resolver `wa_id→user_id` (H2 — confirmada).
- O `RequireUser` via `X-User-ID` no PRD card é transitório por design (H3 — confirmada).
- VPS Hostinger não possui KMS, justificando adiar JWT (H5 — confirmada).
- 60 msg/min por user é teto adequado (H7 — não validada, valida pós-lançamento).
- Rate-limit in-memory sustenta única instância VPS por 12 meses (H8 — não validada).
- LLM jamais sairá do mesmo processo Go no MVP (H10 — não validada, reavaliar se ADR de "LLM out-of-process" surgir).

## Alternativas Avaliadas
### Alternativa 1 - Principal in-process via ctx
Resumo:
Auth como contrato Go (`auth.Principal` em `context.Context`). Webhook WhatsApp é a única boundary autenticadora. LLM e tools rodam in-process e herdam ctx. Usecases leem `user_id` exclusivamente de `auth.FromContext(ctx)`, nunca do payload. Sem JWT, sem token persistido.

Viabilidade:
Técnica alta — usa apenas stdlib + identity já existente. Operacional alta — nada para girar/manter. Financeira alta — zero infraestrutura nova. Risco: contrato ad-hoc; quando app/web chegarem, refactor de domínio é provável porque não há interface explícita separando boundary de domínio.

### Alternativa 2 - JWT Ed25519 curto + JWKs
Resumo:
Identity emite JWT Ed25519 com claims mínimos (`sub=user_id`, `sid`, `aud`, `exp=15min`) após resolver `wa_id→user_id`. LLM tool calls passam o JWT no header `Authorization`. Middleware valida assinatura e injeta Principal no ctx. JWKs endpoint pronto. Sem refresh token no MVP. Revogação via deny-list em Postgres.

Viabilidade:
Técnica média — exige libs JWT auditadas + gestão de `kid` + rotação manual em VPS sem KMS. Operacional baixa — rotação manual é vetor de erro humano em ambiente sem secret manager. Financeira média — JWKs endpoint adiciona overhead. Risco: complexidade excessiva para MVP cirúrgico onde LLM é in-process; chaves em env file aumentam risco de leak.

### Alternativa 3 - Sessão opaca persistida em Postgres
Resumo:
Token aleatório de 32 bytes (armazenado como SHA-256 hash). Tabela `auth_sessions(token_hash, user_id, created_at, last_seen, expires_at, revoked_at, source)`. Middleware valida via `SELECT` por `token_hash`. Revogação ativa nativa (`UPDATE revoked_at = now()`).

Viabilidade:
Técnica alta — apenas Postgres. Operacional média — 1 lookup PG por requisição autenticada (mitigável com cache in-mem). Financeira alta. Risco: se PG cai, auth cai; não resolve elegantemente o caso LLM in-process (Principal in-process já existe sem precisar de token); over-engineering para o MVP.

### Alternativa 4 - Boundary-explicit (ctx + interface JWT documentada)
Resumo:
Idêntica a (A) em runtime MVP, mas declara explicitamente `auth.Principal` como o ÚNICO modo do domínio conhecer identidade, e documenta via ADR o contrato de boundary HTTP futura (JWT) que app/web implementarão. Variante (D') escolhida pelo usuário: apenas `auth.Principal` + ADR, sem interface Go vazia agora.

Viabilidade:
Técnica alta. Operacional alta. Financeira alta. Mesmo custo MVP de (A), mas evita refactor de domínio no futuro porque o contrato já existe. Exige disciplina arquitetural — devs devem entender que handlers nunca leem header direto.

## Trade-offs
- (D' escolhida): aceita ~10 linhas extras e 1 ADR para evitar refactor de domínio quando app/web chegarem.
- Rate-limit in-memory: aceita dívida controlada para evitar carga PG hoje, com gatilho documentado para migrar a PG sliding window quando escalar horizontalmente.
- Sem revogação ativa de sessão no MVP: aceita janela de exposição de 60s (TTL do cache `wa_id→user_id`) em caso de incidente, em troca de complexidade reduzida.
- Sem refresh token: aceita re-resolução de Principal a cada mensagem WhatsApp (custo PG: 1 SELECT por TTL expirado), em troca de não precisar gerir refresh.
- LLM in-process: aceita acoplamento entre orquestrador LLM e backend; reavaliar se ADR de "LLM out-of-process" surgir.

## Riscos
- Risco: cross-tenant data leak por bug em query (`WHERE user_id` ausente).
  Impacto: alto (violação direta da premissa de negócio + LGPD).
  Probabilidade: média (mitigada parcialmente por contratos, mas erros humanos acontecem).
  Mitigação: testes de isolamento obrigatórios em cada usecase; depguard/linter custom proibindo handler ler payload `user_id`; em uma fase futura, avaliar RLS Postgres.

- Risco: webhook Meta spoofing (POST forjado).
  Impacto: alto (atacante registra ações como usuário arbitrário).
  Probabilidade: baixa (Meta assina, vazamento do app_secret é precondição).
  Mitigação: HMAC `X-Hub-Signature-256` validado antes de qualquer parse; `verify_token` checado; rotação documentada do `app_secret`.

- Risco: LLM tool call recebe `user_id` injetado por prompt do usuário.
  Impacto: alto (replay/session hijack).
  Probabilidade: média (depende de como tools recebem parâmetros).
  Mitigação: tools obrigatoriamente derivam `user_id` de `auth.FromContext(ctx)`, nunca de argumentos do prompt; teste de regressão.

- Risco: `wa_id` em logs (PII leak).
  Impacto: médio (LGPD).
  Probabilidade: baixa (existem VOs `Masked()`, mas dev pode esquecer).
  Mitigação: linter custom + teste que inspeciona output do logger configurado (já existe padrão R7).

- Risco: rate-limit in-memory falha em escala horizontal sem migrar.
  Impacto: médio (abuso permitido).
  Probabilidade: baixa enquanto VPS única.
  Mitigação: dívida documentada + alerta em métrica `rps_total > 50`.

- Risco: troca de número de WhatsApp causa staleness durante TTL do cache.
  Impacto: baixo (60s).
  Probabilidade: baixa.
  Mitigação: invalidar cache via outbox event quando usecase futuro `UpdateUserWhatsApp` existir.

## Custos
Estimativa relativa:
baixa

Drivers de custo:
- 1 pacote novo Go (`internal/identity/.../auth` ou `internal/platform/auth` — decidir em discovery técnico).
- 1 ADR.
- 1 tabela `auth_events` + migration + projector.
- 1 middleware `RequireUser` + 1 helper de validação HMAC Meta.
- 1 token bucket in-memory.
- Testes table-driven + integração para isolamento de tenant.

## Impactos Operacionais
- Runbook novo: rotação de `app_secret` Meta e `verify_token` (passos manuais via env file em VPS).
- Dashboard Grafana: painel "Auth Module" com métricas `auth_principal_established_total`, `auth_failed_total`, `auth_rate_limit_hits_total`, `auth_unknown_wa_id_total`.
- Alerta: `auth_failed_total > N/min` indica possível ataque; `auth_unknown_wa_id_total > M` indica vazamento de webhook URL.
- PRD card desbloqueia para implementação (passa a depender do `RequireUser` definitivo, não do `X-User-ID` transitório).
- PRD categories e budgets passam a depender de auth antes da implementação.
- Deploy não muda — segue VPS Hostinger via reverse proxy.

## Segurança
- Tenant isolation: contrato `auth.Principal` torna impossível para usecase derivar `user_id` que não seja do Principal corrente.
- Webhook signing: HMAC SHA-256 do payload com `app_secret` Meta validado antes de qualquer parse.
- PII masking: `wa_id`/email/IP via VOs `Masked()` em todos os logs e spans.
- Audit log: `auth_events` com retenção ≥ 90 dias; preservado anonimizado em direito à exclusão.
- Rotação: `app_secret` Meta e `verify_token` rotacionáveis via env file + restart graceful.
- Rate-limit: 60 msg/min por user (in-memory token bucket), configurável.
- LGPD art. 18 (exclusão): `MarkUserDeleted` invalida cache `wa_id→user_id` via outbox event; audit log preservado por reten legal.

## Observabilidade
- Métricas (OTel): `auth_principal_established_total{source}`, `auth_failed_total{reason}`, `auth_unknown_wa_id_total`, `auth_rate_limit_hits_total{user_id_hash}`, `auth_webhook_signature_invalid_total`.
- Logs: estruturados com `wa_id_masked`, `user_id`, `source`, `correlation_id`. Nenhum `wa_id` cru.
- Traces: span `auth.resolve_principal` com atributos `source`, `user_id` (nunca `wa_id`).
- Eventos outbox: `auth.principal_established`, `auth.failed`, `auth.unknown_user` para projeção em `auth_events`.
- Dashboard "Auth Module" em Grafana, panels para cada métrica acima.
- Alertas: `auth_failed_total / 5min > threshold`; `auth_webhook_signature_invalid_total > 0` (possível tentativa de spoofing).

## Escalabilidade
- Capacidade MVP: única instância VPS Hostinger; 60 msg/min por user; teto estimado de centenas de usuários simultâneos.
- Gargalos previstos: rate-limit in-memory (não compartilhado entre instâncias); resolução `wa_id→user_id` (1 SELECT por TTL de cache expirado, mitigável com cache LRU).
- Crescimento esperado: até ~1000 usuários ativos sem mudança; >1000 exige reavaliar rate-limit + introduzir cache distribuído opcional.
- Limite operacional: quando escalar horizontalmente, migrar rate-limit para PG sliding window (dívida documentada); quando app/web chegarem, implementar JWT boundary conforme ADR.

## Alternativa Recomendada
**Alternativa 4 - Boundary-explicit (ctx + interface JWT documentada)** na variante D' (apenas `auth.Principal` em `context.Context` + ADR documentando o contrato de boundary HTTP futura, sem criar a interface Go vazia agora).

## Justificativa
Empata com (A) em custo MVP (zero infraestrutura nova, mesmo tempo de entrega) e supera (A) em manutenibilidade, pois um ADR documenta o contrato `auth.Principal` como o único modo do domínio conhecer identidade. Quando app/web chegarem, o JWT boundary é implementação nova; domínio e usecases ficam inalterados. (B) JWT no MVP é over-engineering (chaves em env file sem KMS aumentam risco de leak) e adiciona complexidade desnecessária num MVP onde LLM é in-process. (C) Sessão opaca é over-engineering (não resolve o caso in-process e cria acoplamento total a PG). (D') é a única que honra simultaneamente "fundação em camadas" (Rodada 1) + "mínimo cirúrgico" (Rodada 2) + zero dependência nova, alinhando com o teto de risco operacional de uma VPS sem KMS.

## Decisões Pendentes
- Pacote final: `internal/identity/.../auth`, `internal/platform/auth` ou `internal/auth` — decidir em `technical-discovery-production`.
- Política de retenção exata para `auth_events` (90d mínimo confirmado; teto e arquivamento ainda abertos).
- Validar H7 (60 msg/min) com métricas reais pós-lançamento.
- Validar H10 (LLM permanece in-process) a cada decisão de arquitetura nova.

## Próximo Passo Recomendado
`technical-discovery-production` com objetivo de:
1. Decidir pacote/módulo destino do novo código (`internal/identity` vs `internal/platform/auth` vs `internal/auth`).
2. Especificar contrato `auth.Principal` (campos, helpers, regras de imutabilidade, regras de logging).
3. Detalhar fluxo HMAC Meta + verify_token (algoritmo, headers, edge cases).
4. Especificar schema `auth_events` + projector + migrations.
5. Definir métricas/OTel + dashboard + alertas.
6. Produzir ADR documentando o contrato de boundary HTTP futura.

Pós discovery técnico, gerar PRD dedicado ao módulo de auth via `create-prd`, depois `create-technical-specification`, depois `create-tasks`. Card/categories/budgets aguardam `RequireUser` definitivo antes de execução.
