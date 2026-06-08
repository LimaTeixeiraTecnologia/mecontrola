# Transcript do Brainstorming Decisório

## Contexto Inicial
- Usuário pediu brainstorming decisório focado em autenticação e autorização seguras, MVP robusto production-ready inegociável, API Golang.
- Interface inicial: LLM + WhatsApp. Evolução prevista: app móvel e web.
- PRDs já existentes que dependem desta decisão: `.specs/prd-card-crud-mvp`, `.specs/prd-categories-crud`, `.specs/prd-budgets-monthly`.
- Premissa de negócio: usuário é dono das informações; nenhum dado deve ser visível a outro usuário.
- Jornada conforme infográfico MeControla: anúncio Kiwify → checkout → e-mail acesso → WhatsApp → onboarding (renda, cartões, categorias, percentuais) → uso diário via WhatsApp.
- Skill obrigatória downstream: `go-implementation` (carregar exemplos sob demanda).
- Análise prévia do codebase:
  - `internal/identity` já provisiona usuário (`UpsertUserByWhatsApp`, `FindUserByWhatsApp`, `MarkUserDeleted`, `ProjectSubscriptionEvent`, `DecideUserEntitlement`).
  - `internal/onboarding` já tem magic-token (base64url-sha256) para ativação via Meta WhatsApp Cloud API — token de ativação, não de sessão.
  - `internal/card` PRD declara middleware **transitório** `RequireUser` via header `X-User-ID` "enquanto módulo de autenticação não existir" → lacuna explícita confirmada.
  - Nenhum JWT, sessão, OAuth, RBAC ou middleware AuthZ em produção.
- ADRs do onboarding relevantes: ADR-009 (deploy VPS Hostinger com reverse proxy), ADR-011 (cross-module bind retry, secrets, log shipping).

## Rodada 1 - Entendimento do Problema
**P1.** Qual o problema central a ser resolvido agora?
- Opções: AuthZ+isolamento; AuthN ponta-a-ponta; Identidade canônica+sessão WhatsApp; Tudo em camadas.
- **Resposta:** Tudo acima como camadas — fundação com identidade canônica + sessão WhatsApp implementadas no MVP, JWT futuro deixado como interface documentada para app/web.

**P2.** Identidade canônica?
- Opções: user_id UUID; wa_id; híbrido com tabela `user_identities`.
- **Resposta:** user_id (UUID interno). Outros canais resolvem para user_id antes do handler.

**P3.** Modelo de ameaça prioritário (multi-select)?
- Opções: cross-tenant leak; spoofing webhook WhatsApp; token vazado; replay/hijack via LLM tool.
- **Resposta:** Todos os quatro vetores são prioritários.

## Rodada 2 - Escopo e Restrições
**P1.** Escopo do MVP antes de card/categories/budgets ganharem AuthZ definitivo?
- **Resposta:** Mínimo cirúrgico — middleware que resolve identidade→user_id e injeta em ctx; webhook WhatsApp resolve wa_id→user_id via Identity; LLM/tools in-process. Sem login web, sem refresh token, sem revogação ativa neste MVP.

**P2.** Restrições de infra/deploy (multi-select)?
- **Resposta:** VPS Hostinger permanece; Postgres único storage; Outbox obrigatório para eventos; zero dependência externa nova (sem Auth0/Clerk/Keycloak/Vault).

**P3.** Compliance/LGPD e auditoria não-negociáveis (multi-select)?
- **Resposta:** Audit log de auth events; PII mascarada em logs (R7); direito à exclusão (LGPD art. 18); rate-limit por user_id + global.

## Rodada 3 - Alternativas
Quatro alternativas comparáveis foram apresentadas:
- **(A) Principal in-process via ctx** — auth como contrato Go (`auth.Principal` em ctx), webhook como única boundary autenticadora hoje. Sem JWT, sem token persistido no MVP.
- **(B) JWT Ed25519 curto + JWKs** — assinatura assimétrica, claims mínimos, kid+rotação, sem refresh no MVP cirúrgico, deny-list em PG para revogação.
- **(C) Sessão opaca persistida em PG** — token aleatório, `auth_sessions(token_hash, user_id, ...)`, revogação ativa nativa, custo PG por requisição.
- **(D) Boundary-explicit (ctx + interface JWT)** — A + interface Go vazia `JWTBoundary` + ADR documentando contrato. Domínio nunca vê primitivas de transporte.

**Resposta inicial do usuário:** sinalizou (A). Mas a Rodada 1 escolheu "tudo em camadas", o que se alinha com (D). Tensão sinalizada para Rodada 4.

## Rodada 4 - Trade-offs
Scorecard preenchido em `option-scorecard.md`:
- A=40, B=31, C=33, D=42.
- A e D empatam em custo MVP e tempo de entrega.
- D supera A em manutenibilidade e em risco de drift arquitetural (handlers não podem ler header direto se Principal é contrato).
- B perde por gestão de chaves Ed25519+JWKs em VPS sem KMS.
- C perde por acoplamento total a PG e por não resolver o caso LLM in-process.

**P1.** Tensão A vs D — qual prevalece?
- Opções: (D) com interface + ADR; (A) sem interface; (D') só `auth.Principal` + ADR.
- **Resposta:** (D') Só `auth.Principal` + ADR descrevendo a boundary HTTP futura. Sem criar a interface Go vazia agora. Custo: ~10 linhas + 1 doc.

**P2.** Locks operacionais (multi-select):
- **Resposta:** Todos os 4 selecionados:
  1. Resolver `wa_id→user_id` apenas no webhook (cache TTL curto; falha = 401 + outbox event).
  2. Outbox event para auth lifecycle + audit projector → tabela `auth_events`.
  3. `RequireUser` substitui `X-User-ID` transitório do PRD card.
  4. Rate-limit 60 msg/min/user in-memory + dívida documentada para migrar a PG sliding window.

## Rodada 5 - Seleção de Direção
**P.** Confirma direção (D') + 4 locks operacionais?
- **Resposta:** Confirmado. Direção preliminar fechada para handoff ao discovery técnico.

## Decisões Registradas
- D-001: Alternativa recomendada é **(D') Principal in-process via ctx + ADR de fronteira HTTP futura**.
- D-002: Identidade canônica é `user_id` (UUID interno). `wa_id` e `email` são identificadores resolvíveis, nunca chave de tenancy.
- D-003: Webhook WhatsApp é a única boundary autenticadora no MVP; resolve `wa_id→user_id` via `Identity.FindUserByWhatsApp` + cache TTL curto.
- D-004: Middleware `RequireUser` lê `auth.Principal` do `context.Context`; 401 imediato se ausente. Substitui o `X-User-ID` transitório do PRD card antes da implementação de card/categories/budgets avançar.
- D-005: Usecases jamais recebem `wa_id`; apenas `user_id` derivado de `auth.FromContext(ctx)`.
- D-006: Eventos de auth (`auth.principal_established`, `auth.failed`, `auth.unknown_user`) publicados via `outbox.Publisher` na mesma transação; consumidor projeta para tabela `auth_events`.
- D-007: Rate-limit 60 msg/min/user em memória + dívida controlada para migrar a PG sliding window quando escalar horizontalmente.
- D-008: HMAC do webhook Meta (`X-Hub-Signature-256`) + verify_token são pré-requisitos invioláveis antes de resolver identidade.
- D-009: Não usar dependência externa nova (Auth0/Clerk/Keycloak/Vault) — apenas stdlib + libs Go já no go.mod.
- D-010: PRD futuro para app/web especificará a boundary HTTP (provável JWT Ed25519 + refresh), implementando o contrato documentado pelo ADR sem refactor de domínio.
