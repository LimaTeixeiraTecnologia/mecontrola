# ADR-001: Autenticação via RequireUser em todos os endpoints de categories

## Metadados

- **Título:** Autenticação via RequireUser em endpoints de categories
- **Data:** 2026-06-09
- **Status:** Aceita
- **Decisores:** Time de engenharia MeControla
- **Relacionados:** PRD v7 (categories), `prd-auth-foundation`, RT-08

## Contexto

O PRD de categories em RT-08 afirma que o módulo "serve requisições anônimas dentro da rede interna confiável". Contudo, o PRD v7 (bump pós-`prd-auth-foundation` task 9.0) determina que "endpoints autenticados usarão o `RequireUser` canônico de `internal/identity/infrastructure/http/server/middleware`". Essa contradição exige uma decisão explícita de arquitetura.

## Decisão

Todos os endpoints públicos do módulo `internal/categories` (`GET /v1/categories`, `GET /v1/categories/{id}`, `GET /v1/category-dictionary`, `GET /v1/category-dictionary/search`) exigem autenticação via middleware `RequireUser`.

O middleware é aplicado no `CategoryRouter` durante o registro de rotas. Não há duplicação de lógica de auth no módulo; apenas importação do middleware já existente.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Anônimo na rede interna (RT-08 literal) | Menor latência, zero dependência de identity | Quebra contrato do PRD v7; gateway/mesh sozinho não garante identidade do usuário para futuras features | PRD v7 é posterior e expressamente manda usar RequireUser |
| Autenticação opcional (middleware condicional) | Flexível para consumidores internos | Complexidade desnecessária; nenhum consumidor do MVP precisa de acesso anônimo | Over-engineering para escopo fechado |
| Header transitivo `X-User-ID` | Simples de implementar | Removido explicitamente pelo PRD v7; não reutiliza infraestrutura de auth existente | Viola decisão arquitetural do auth foundation |

## Consequências

### Benefícios Esperados

- Alinhamento com `prd-auth-foundation` e padrão de auth do ecossistema.
- `auth.Principal` disponível no `context.Context` para futuras evoluções (ex: audit log, rate limit por usuário).
- Gateway/mesh continua garantindo camada de transporte; auth garante camada de aplicação.

### Trade-offs e Custos

- Adiciona ~1-2 ms de latência por request (lookup de contexto).
- Módulo categories adquire dependência de import do pacote `internal/identity/infrastructure/http/server/middleware`. Essa é uma importação cross-module na camada de infraestrutura, permitida por AGENTS.md para middlewares compartilhados.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Identity indisponível → auth falha | Categories fica inacessível | `RequireUser` é stateless e local (só lê contexto); não depende de identity em runtime |
| Loop de import entre identity e categories | Build failure | Categories importa apenas middleware de identity; identity não importa categories |

## Plano de Implementação

1. No `CategoryRouter.Register`, aplicar `.With(middleware.RequireUser)` nas rotas.
2. Adicionar import de `internal/identity/infrastructure/http/server/middleware` no router.
3. Validar em teste de integração que request sem `Principal` no contexto retorna 401.

## Monitoramento e Validação

- Métrica de requests 401 no router de categories.
- Teste de handler que valida rejeição quando `auth.FromContext` retorna `false`.
