# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Middleware HTTP `InjectPrincipalFromHeader` em `internal/identity` (transitório até JWT/OIDC)
- **Data:** 2026-06-09
- **Status:** Aceita (transitória — substituível por adapter JWT/OIDC sem mudar contrato)
- **Decisores:** Jailton (tech lead)
- **Relacionados:** `.specs/prd-card-crud-mvp/prd.md` (F-06, RF-27), `.specs/prd-auth-foundation/` (task 9.0), `.specs/prd-card-crud-mvp/techspec.md`

## Contexto

O PRD v2 do `card` declara que o middleware transitório `RequireUser` por `X-User-ID` será substituído pelo `RequireUser` canônico de `internal/identity/.../middleware`, que apenas verifica `auth.FromContext(ctx)`. Inspecionando o working tree:

- `internal/identity/application/auth/principal.go` define `Principal{UserID, Source}` e `WithPrincipal`/`FromContext`.
- `internal/identity/.../middleware/require_user.go` apenas valida presença e não popula nada.
- `EstablishPrincipal` é consumido somente pelo dispatcher WhatsApp (`internal/platform/whatsapp/dispatcher`).

Não existe middleware HTTP que injete `auth.Principal` na cadeia. Sem ele, `RequireUser` sempre devolve 401 em rotas HTTP públicas. O PRD precisa de uma ponte transitória.

## Decisão

Adicionar middleware `InjectPrincipalFromHeader` em `internal/identity/infrastructure/http/server/middleware/inject_principal_from_header.go`:

- Lê header `X-User-ID`.
- Valida que é UUID v4.
- Constrói `auth.Principal{UserID: parsed, Source: auth.SourceHeader}` e chama `auth.WithPrincipal(ctx, p)`.
- Em ausência/invalidez NÃO retorna 401 — apenas segue. O `RequireUser` canônico produz o 401 e mantém o ponto único de enforcement.

Mudança additive em `internal/identity/application/auth/principal.go`: adicionar constante `SourceHeader PrincipalSource = "header"`.

Encadeamento padrão do `CardRouter`:

```text
InjectPrincipalFromHeader → RequireUserWithO11y → idempotency.Middleware (POST/PUT/DELETE) → handler
```

Localizar em `internal/identity` (e não em `internal/card`) porque `auth.Principal` é tipo do bounded context identity; um middleware em `card` violaria a regra de fronteira de bounded contexts.

## Alternativas Consideradas

1. **Middleware dentro de `internal/card`** — Vantagens: ownership co-localizado com consumidor; Desvantagens: `card` passaria a depender e estender `auth.PrincipalSource`, conflitando com a regra "comunicação cross-module via interface declarada pelo consumidor"; outros módulos teriam que duplicar o middleware. Rejeitada.
2. **Pular `RequireUser` e validar header diretamente** — Vantagens: 1 middleware só; Desvantagens: contradiz o contrato canônico, dificulta migração futura para JWT (precisaria editar todos os roteadores), perde span "auth.require_user" já existente. Rejeitada.
3. **Esperar JWT/OIDC antes de entregar o módulo** — Vantagens: solução final imediata; Desvantagens: bloqueia OBJ-01 do PRD por trimestres. Rejeitada.

## Consequências

### Benefícios Esperados

- Preserva o contrato canônico `RequireUser`; JWT/OIDC futuro substitui apenas `InjectPrincipalFromHeader` por `InjectPrincipalFromJWT`, sem mudar handlers.
- Ownership correto: identity continua dono do `auth.Principal`.
- Reutilizável pelos próximos módulos (`transaction`, `budget` etc.) ainda no período transitório.

### Trade-offs e Custos

- Confiar no header `X-User-ID` exige que o gateway/balanceador antes do `mecontrola` autentique e injete o header (PRD S-07). Em ambiente público sem gateway → spoofing trivial. Mitigação operacional: exposição interna até gateway estar pronto.
- Adiciona uma constante a `PrincipalSource` (`SourceHeader`) — espaço para drift se não removida quando JWT/OIDC entrar.

### Riscos e Mitigações

- **Spoofing de `X-User-ID`** → restringir exposição até gateway garantir injeção confiável; documentar em runbook.
- **Drift entre `SourceHeader` e `SourceWhatsApp`** (analytics confundindo origens) → atributo `source` em métricas `auth_*` permite segmentar.
- **Esquecer de remover `InjectPrincipalFromHeader` ao chegar JWT** → ADR fica `Substituída` por ADR futura; checklist de migração inclui remoção do middleware e da constante.

## Plano de Implementação

1. Estender `internal/identity/application/auth/principal.go` com `SourceHeader`.
2. Criar `inject_principal_from_header.go` + teste unitário (header ausente, inválido, válido → ctx populado).
3. Encadear no `CardRouter.Register`.
4. Documentar no runbook e no `docs/runbooks/card-rollback.md` o caráter transitório.

Adoção concluída quando: 401 ocorre apenas via `RequireUser` canônico + testes do `CardRouter` cobrem header ausente/inválido/válido.

## Monitoramento e Validação

- Span `auth.require_user` já registra `result=pass|unauthorized`.
- Adicionar atributo `principal.source` quando span do middleware existir.
- Critério de revisão: 0 incidente de spoofing reportado durante exposição pública.

## Impacto em Documentação e Operação

- `AGENTS.md` (entrada "Layout Obrigatorio por Modulo") — sem mudança.
- `docs/runbooks/card-rollback.md` — listar como dependência transitória.
- Pré-condição operacional: gateway/balanceador injeta `X-User-ID` antes de expor publicamente (PRD S-07).

## Revisão Futura

Revisitar quando:

- Módulo de autenticação real (JWT/OIDC) entrar em produção — substituir middleware e marcar esta ADR como `Substituída`.
- Quaisquer 2 módulos não-identity precisarem do mesmo padrão — formalizar contrato comum.
