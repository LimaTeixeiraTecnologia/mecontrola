# ADR-004 — Ordem da chain de middlewares no router de cards

## Metadados

- **Título:** Ordenação `RequireGatewayAuth → InjectPrincipal → RequireUser → idempotency`
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-01, RF-13; [techspec](techspec.md) seção Fluxo de Dados; `internal/card/infrastructure/http/server/router.go`

## Contexto

A ordem dos middlewares no chain decide:
- Quando o request é rejeitado (cedo = barato).
- Se um middleware tem acesso ao output do anterior (e.g. `RequireUser` depende de `Principal` no context).
- Custo CPU em request hostil (DoS).

Chain atual em `internal/card/.../router.go`:
```
sub.Use(InjectPrincipalFromHeaderWithO11y)
sub.Use(RequireUserWithO11y)
sub.With(idempotencyMiddleware) ...
```

`RequireGatewayAuth` precisa ser inserido. Posição ótima depende de trade-off entre defesa-em-profundidade (rejeitar cedo) vs observabilidade (cobrir falha em span/log).

## Decisão

Ordem na chain (top-down):

```
RequireGatewayAuth                  -- novo, rejeita 401 cedo
InjectPrincipalFromHeaderWithO11y   -- existente, lê X-User-ID
RequireUserWithO11y                 -- existente, garante Principal não-zero
Idempotency middleware (per-route)  -- existente, só em POST/PUT/DELETE
Handler
```

Tracer/logger globais permanecem fora do `sub.Route("/api/v1/cards")` — são montados no servidor raiz e cobrem o request inteiro, incluindo a falha 401 do gateway.

## Alternativas Consideradas

1. **tracer/logger → RequireGatewayAuth → ...** — colocar tracer dentro do sub. **Rejeitada**: o tracer já é global no servidor raiz; duplicar implica span aninhado desnecessário e custo extra em request hostil.
2. **rate-limit IP → RequireGatewayAuth → ...** — adicionar rate-limit antes. **Rejeitada**: rate-limit em `/cards` é item A10 do plano-fonte (fora deste PRD); implementar agora atrasa entrega. Pode ser inserido no mesmo ponto pós A10 sem mudar esta decisão.
3. **InjectPrincipal → RequireGatewayAuth → RequireUser** — ler Principal antes do gateway. **Rejeitada**: `RequireGatewayAuth` precisa do `X-User-ID` raw para canonicalização, e isso é independente do Principal injetado; injetar antes não traz benefício e desperdiça CPU em request hostil.

## Consequências

### Benefícios Esperados

- Request hostil rejeitado em < 50µs (parse de 3 headers + HMAC + comparação).
- `InjectPrincipal` e `RequireUser` não rodam para request hostil — defesa em profundidade real.
- Idempotency-Key só é gravada para request autêntica — não polui tabela com lixo.

### Trade-offs e Custos

- Span do tracer global cobre a falha 401 mas não tem o contexto do "Principal" (não foi injetado ainda). Aceito: o atributo `result` do span de gateway já carrega informação suficiente para forense.

### Riscos e Mitigações

- **R-01**: PR futuro insere middleware adicional entre `RequireGatewayAuth` e `InjectPrincipal`. **Mitigação**: gate de revisão M-09 (`lint-auth-bypass.sh`) verifica que `RequireGatewayAuth` está imediatamente antes de `InjectPrincipal` em todo router que use o injetor.

## Plano de Implementação

1. Editar `internal/card/infrastructure/http/server/router.go` para inserir `RequireGatewayAuth` antes de `InjectPrincipalFromHeaderWithO11y`.
2. Gate `lint-auth-bypass.sh`: grep que prova que onde `InjectPrincipalFromHeaderWithO11y` aparece, `RequireGatewayAuth` aparece imediatamente antes no mesmo bloco.

## Monitoramento e Validação

- Integration test cobre chain real e valida que 401 vem do `RequireGatewayAuth`, não dos middlewares posteriores.
- Smoke test do plano-fonte seção 9, item 1: `curl -H "X-User-ID: <uuid>" .../cards` retorna 401.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: diagrama da chain.
- README do módulo `internal/identity`: nota sobre ordem.

## Revisão Futura

Revisar quando:
- Implementação de A10 (rate-limit por user) — avaliar se rate-limit IP vai antes do gateway.
- Migração para JWT — pode unificar `RequireGatewayAuth` + `InjectPrincipal` em um único middleware.
- Data sugerida: 2027-06-12.
