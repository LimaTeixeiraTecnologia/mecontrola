# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Contrato `auth.Principal` in-process e boundary HTTP futura
- **Data:** 2026-06-08
- **Status:** Aceita
- **Decisores:** Jailton (arquitetura), product owner do MeControla
- **Relacionados:** `.specs/prd-auth-foundation/prd.md` (v3), `.specs/prd-auth-foundation/techspec.md`, `docs/discoveries/brainstorms/brainstorm-autenticacao-autorizacao-mvp-whatsapp/`, `docs/discoveries/technical-auth-foundation-mvp-whatsapp-llm/`, ADR-005-bis do `prd-identity-foundation`, ADR-009 do `prd-onboarding-magic-token`

## Contexto

Os PRDs `prd-card-crud-mvp`, `prd-categories-crud` e `prd-budgets-monthly` declaram um middleware `RequireUser` transitório por header `X-User-ID` cru, sem autenticação real. Isso viola a premissa de negócio inegociável "cada usuário é dono exclusivo das próprias informações financeiras". A interface inicial do produto é WhatsApp + LLM in-process; app móvel e web são evolução futura. O deploy é em VPS Hostinger sem KMS, e o `go.mod` não pode receber dependência externa nova (sem Auth0/Clerk/Keycloak/Supabase/Vault).

Restrições inegociáveis: Go, Postgres único storage, outbox obrigatório para eventos, LGPD com PII mascarada e direito à exclusão, R0–R7 da skill `go-implementation`. Vetores de ameaça priorizados no brainstorming: cross-tenant leak, spoofing de webhook Meta, token leak, replay/hijack via LLM tool.

O brainstorming decisório (bundle aprovado) avaliou 4 alternativas: (A) Principal in-process sem ADR; (B) JWT Ed25519 + JWKs no MVP; (C) Sessão opaca persistida em Postgres; (D) Boundary-explicit com contrato `Principal` + ADR documentando boundary futura. A variante D' (compromisso) foi escolhida: apenas `Principal` em `ctx` + ADR descrevendo a boundary HTTP futura, sem criar interface Go vazia.

## Decisão

Adotar `auth.Principal` como **tipo concreto Go imutável** transportado por `context.Context` como **única forma do domínio conhecer a identidade do usuário**. Nenhum dado de transporte (header, JWT cru, cookie, payload) entra em usecases ou repositórios.

**Contrato Go (estável, parte do ADR):**

```go
package auth

type PrincipalSource string

const SourceWhatsApp PrincipalSource = "whatsapp"
// Reservados (somente neste ADR, NÃO declarar como constante Go até existir consumidor):
//   SourceJWT    PrincipalSource = "jwt"
//   SourceSystem PrincipalSource = "system"

type Principal struct {
    UserID uuid.UUID
    Source PrincipalSource
}

func (p Principal) IsZero() bool
func WithPrincipal(ctx context.Context, p Principal) context.Context
func FromContext(ctx context.Context) (Principal, bool)
```

**Regras invariáveis:**

1. **Apenas boundary autenticada popula `Principal`**. No MVP, a única boundary é o webhook WhatsApp (`POST /api/v1/whatsapp/inbound`) que, após HMAC SHA-256 validado e `EstablishPrincipal` executado, faz `auth.WithPrincipal(ctx, p)` antes de chamar o handler do agent.
2. **Handlers HTTP e usecases jamais leem identificação fora do contrato**. Linter custom (`depguard` + `forbidigo`) torna a violação um erro de compilação/CI (RF-12).
3. **Middleware `RequireUser` é o ponto único de aplicação da autorização em fronteira HTTP**. Retorna 401 sem corpo descritivo quando `Principal` ausente em `ctx`.
4. **`Principal` é imutável por valor**. Helpers retornam cópia; chave de contexto é tipo privado não-exportado para evitar colisão.
5. **`PrincipalSource` é fechado por enum textual**. Schema `auth_events.source` faz CHECK por valores aceitos no MVP (`'whatsapp'` apenas); migrations futuras expandem o CHECK e a constante Go simultaneamente.

**Boundary HTTP futura (escopo de PRD próprio, não-implementada agora):**

Quando app móvel e web exigirem autenticação direta, será criada uma nova boundary em `internal/identity/infrastructure/http/server/middleware/jwt_boundary.go` (nome provisório) que:

- Valida JWT assinado com **Ed25519** (algoritmo assimétrico para suportar JWKs).
- Claims mínimos: `sub` (user_id UUID), `sid` (session_id UUID), `aud`, `iss`, `exp` (≤ 15 min), `iat`, `kid` (key id para rotação).
- Endpoint JWKs público `GET /.well-known/jwks.json` expõe chaves públicas com `kid` versionado.
- Refresh token rotativo persistido em tabela própria (`auth_sessions` ou similar), com revogação ativa.
- Após validação, executa `auth.WithPrincipal(ctx, Principal{UserID: sub, Source: SourceJWT})` e delega ao handler. Domínio **inalterado**.

A migração para essa boundary **não exige refactor de usecases ou repositórios** — toda lógica de domínio já consome `auth.FromContext`. Apenas duas mudanças incrementais: (a) declarar `SourceJWT`/`SourceSystem` como constantes Go quando a primeira PR de boundary entrar; (b) migration que expanda o CHECK de `auth_events.source`.

## Alternativas Consideradas

### Alternativa A — Principal in-process sem ADR

- **Descrição**: Implementar `auth.Principal` exatamente como o D', mas sem documentação ADR.
- **Vantagens**: Menos um documento; rollout marginalmente mais rápido.
- **Desvantagens**: Drift arquitetural alto. Sem contrato explícito, devs futuros podem (a) ler header direto no handler "porque é mais simples", (b) reimplementar boundary HTTP de forma incompatível, (c) introduzir interface `Principal` paralela. Refactor pesado quando app/web chegarem.
- **Motivo de não escolha**: viola "fundação em camadas" decidida na Rodada 1 do brainstorming. Custo marginal de manter ADR é trivial (~30 linhas).

### Alternativa B — JWT Ed25519 + JWKs já no MVP

- **Descrição**: Identity emite JWT após resolver `wa_id→user_id`; LLM tool calls passam JWT no header `Authorization`; middleware valida assinatura e injeta Principal.
- **Vantagens**: Fundação "completa" desde o dia 1; app/web só plugam UI; semântica padrão da indústria.
- **Desvantagens**: Gestão de chaves Ed25519 em VPS sem KMS é vetor de erro humano (rotação manual com kid); JWKs endpoint adiciona overhead que nenhum cliente externo consome no MVP; refresh token persistido em PG cria nova tabela que ninguém usa; chaves em env file aumentam risco de leak; complexidade desnecessária num MVP onde LLM é in-process e não há cliente externo.
- **Motivo de não escolha**: over-engineering. Custo operacional alto sem ganho de segurança real (não há cliente externo a autenticar). Score 31 vs 42 (D) no scorecard.

### Alternativa C — Sessão opaca persistida em Postgres

- **Descrição**: Token aleatório 32 bytes (armazenado como SHA-256 hash); tabela `auth_sessions(token_hash, user_id, created_at, last_seen, expires_at, revoked_at, source)`; middleware valida via SELECT por `token_hash`.
- **Vantagens**: Revogação ativa nativa (`UPDATE revoked_at`); audit trivial; sem chave criptográfica para girar.
- **Desvantagens**: Não resolve elegantemente o caso LLM in-process (Principal in-process já existe sem precisar de token); acoplamento total a PG (se PG cai, auth cai); 1 SELECT por requisição autenticada; over-engineering para o MVP onde o canal é WhatsApp.
- **Motivo de não escolha**: complexidade injustificada. Score 33 vs 42 (D).

### Alternativa D — Boundary-explicit (escolhida na variante D')

- **Descrição**: `auth.Principal` em `ctx` + ADR documentando contrato e boundary futura.
- **Variante D' (escolhida)**: D sem criar interface Go vazia agora; apenas o contrato e o ADR.
- **Motivo de escolha**: mesmo custo MVP que (A), supera (A) em manutenibilidade; honra "fundação em camadas" da Rodada 1; alinha com restrições de VPS sem KMS; permite evolução para app/web sem refactor de domínio. Score 42 (líder).

## Consequências

### Benefícios Esperados

- **Isolamento por tenant é propriedade arquitetural**, não convenção; linter custom torna violação impossível por design.
- **Refactor zero quando app/web chegarem** — domínio já consome `auth.FromContext`; nova boundary apenas popula `Principal`.
- **Custo MVP mínimo** — ~60 linhas em `principal.go`, ~30 em `require_user.go`, sem chaves para gerir, sem token para persistir, sem JWKs para servir.
- **Segurança por desenho** contra os 4 vetores priorizados:
  - Cross-tenant leak: `WHERE user_id = $1` derivado obrigatoriamente de `Principal`; linter bloqueia bypass.
  - Spoofing Meta: HMAC SHA-256 obrigatório antes de qualquer parse; falha → 401 sem oracle.
  - Token leak: nenhum token no MVP; nada a vazar.
  - Replay/hijack LLM: tools recebem `ctx` com Principal e derivam `user_id` exclusivamente de `auth.FromContext`; prompt injection não atinge identidade.

### Trade-offs e Custos

- **Sem revogação ativa de sessão no MVP** — janela de exposição = tempo até próxima re-resolução por mensagem (sub-segundo na prática, mas teoricamente até a próxima mensagem). Mitigado por `MarkUserDeleted` que faz soft-delete imediato em `users.deleted_at`, fazendo `EstablishPrincipal` retornar `ErrUnknownUser` na próxima mensagem.
- **Sem JWT no MVP** significa que app/web precisarão de PRD próprio com novo PR para criar a boundary HTTP. Mitigado pela invariância documentada: domínio não muda.
- **Disciplina arquitetural exigida** — devs precisam entender que `Principal` é sempre via `ctx`, nunca via header. Mitigado por linter custom (transforma erro de revisão em erro de compilação).
- **`PrincipalSource` como string** (não enum tipado mais forte) — escolhido pela simetria com o CHECK constraint de schema; alternativa de `type Source int` com `String()` rejeitada por overhead sem ganho.

### Riscos e Mitigações

- **Risco**: dev cria handler em pacote novo sem aplicar `RequireUser` e expõe rota sem auth.
  **Impacto**: Alto (leak cross-tenant).
  **Mitigação**: code review obrigatório + checklist no PR template; futura regra `depguard` que valida composição de middleware (não trivial — fica como follow-up).

- **Risco**: prompt injection convence LLM a passar `user_id` arbitrário como argumento de tool.
  **Impacto**: Alto (replay/hijack).
  **Mitigação**: tools NUNCA aceitam `user_id` como argumento; derivam exclusivamente de `auth.FromContext(ctx)`; teste de regressão com prompt adversarial (escopo do PRD do `internal/agent`).

- **Risco**: futuro PRD de boundary HTTP esquece de implementar a invariância e popula `Principal` com dados não-validados.
  **Impacto**: Crítico (auth quebrada).
  **Mitigação**: este ADR é referenciado obrigatoriamente em qualquer PRD que envolva nova boundary; review de arquitetura obrigatório.

- **Risco**: chave de contexto privada colide com outra biblioteca que use `struct{}` igualmente.
  **Impacto**: Baixo (colisão impossível na prática — tipo privado).
  **Mitigação**: `principalCtxKey struct{}` é tipo distinto por package, garantindo unicidade.

### Plano de Rollback

- Não aplicável — feature greenfield, sem código anterior a regredir.

## Plano de Implementação

1. **PR 1**: criar `internal/identity/application/auth/principal.go` + `_test.go` + microbenchmark; publicar este ADR.
2. **PR 2**: criar `internal/identity/infrastructure/http/server/middleware/require_user.go` + `_test.go`.
3. **PR 3**: criar `auth_events` (migration + repo) e `EstablishPrincipal` usecase.
4. **PR Strangler Fig (RF-28)**: detalhado em ADR-002.
5. **PR 4**: criar `Dispatcher` + `Limiter` + integrar `EstablishPrincipal`; popular `Principal` no `ctx` antes do agent handler stub.
6. **PR 5**: `.golangci.yml` com `depguard`/`forbidigo`.

Critério de adoção concluída: load test k6 (RF-29) verde + smoke task verde (RF-30) + linter custom barrando handler de teste artificial.

## Monitoramento e Validação

- Métrica `auth_principal_established_total{source="whatsapp"}` > 0 em produção (RF-17).
- Métrica `auth_failed_total{reason="invalid_signature"}` próxima de 0 (alerta > 0 em 5 min).
- Smoke task `task auth:smoke` verde no CI a cada merge.
- Auditoria periódica (trimestral): grep no codebase confirma que nenhum handler ou usecase lê header de identificação ou consome `user_id` de payload externo.

## Impacto em Documentação e Operação

- **PRDs `prd-card-crud-mvp`/`prd-categories-crud`/`prd-budgets-monthly`**: spec-version bump no primeiro PR de implementação removendo o `X-User-ID` transitório (RF-21).
- **`AGENTS.md`**: nota opcional no Padrão Obrigatório de Módulo direcionando para este ADR.
- **`CLAUDE.md`**: nota opcional referindo `auth.Principal` como contrato canônico de identidade.
- **Dashboard Grafana "Auth Module"**: criar conforme RF-20.
- **Runbook `auth-incident-response.md`**: cobre alertas de `auth_failed_total`.
- **Onboarding interno (devs novos)**: este ADR é leitura obrigatória.

## Revisão Futura

Esta ADR deve ser revisitada quando:

- PRD de app móvel ou web for proposto (gatilho explícito de implementação da boundary HTTP futura).
- LLM deixar de rodar in-process (gatilho de reavaliação completa — pode exigir JWT ou mTLS entre webhook e LLM).
- Volumetria ultrapassar 5.000 usuários ativos (revisão de cache, possível introdução de sessão para reduzir lookup PG).
- Surgir requisito de revogação ativa imediata (gatilho de tabela `auth_sessions` ou deny-list).
- Auditoria LGPD demandar campos extras em `auth_events` (gatilho de evolução do schema e do consumer).
