# ADR-005 — Validação dual: `go-playground/validator` no boundary + Value Objects no domain

## Metadados

- **Título:** Validação dual com schema no boundary e VOs auto-validados no domain
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-04](./prd.md), [techspec §Estratégia de Erros, §Modelagem de Domínio](./techspec.md), [security-app.md §Input Validation](../../.agents/skills/agent-governance/references/security-app.md), [R-DDD-001 §Value Objects](../../.agents/skills/agent-governance/references/ddd.md), [ADR-009 (config Validate())](./adr-009-viper-configs-validate.md)

> **Cross-reference:** esta ADR cobre validação de **input externo em runtime** (HTTP body, CLI args). A validação de **configuração no startup** (env/.env) é coberta pela ADR-009 via `configs.Config.Validate()`. As duas camadas são complementares: ADR-009 garante que o serviço nunca sobe com config inseguro; esta ADR garante que toda request que entra é validada na fronteira antes de tocar Domain.

## Contexto

R-DDD-001 exige VOs auto-validados no domínio. `security-app.md` exige validação no boundary com schema (não validação manual). Aplicar uma OU outra produz lacunas:
- Só VO: pior UX (erros 400 vagos; primitivo cru atravessa N camadas antes de falhar).
- Só validator no boundary: domain recebe primitivo; perde invariantes em tempo de uso programático (call site fora de HTTP).

A escolha precisa harmonizar as duas regras sem duplicar validação.

## Decisão

**Validação em duas camadas, com responsabilidades distintas:**

1. **Boundary (HTTP handler / CLI args)**: usa `go-playground/validator/v10` com tags em DTOs (formato, presença, range, regex). Erros traduzidos para `ProblemDetails` 400 com detalhamento por campo.
2. **Domain (Value Objects)**: construtores `func NewX(...) (X, error)` validam invariantes de negócio. Domain **não aceita primitivos crus**; toda função pública recebe VO.
3. **Adapters** convertem DTO → VO via `application.Mapper` (função pura, testada).

Validador e VO **não duplicam regra**: validator cobre forma sintática (e-mail bem formado, string ≤255); VO cobre semântica de negócio (e-mail único por tenant, valor monetário > 0 em BRL).

## Alternativas Consideradas

1. **Apenas VOs no domain (DDD-puro)**.
   - Vantagens: regra única; mais aderente ao DDD canônico.
   - Desvantagens: handler recebe `{"email": "lixo"}` e só descobre erro 4 camadas depois; mensagem de erro vaga; viola security-app.md ("validar na fronteira de entrada").
2. **Apenas validator no boundary**.
   - Vantagens: simples; tags + helper são padrão Go.
   - Desvantagens: domain recebe `string` cru; viola R-DDD-001 §Value Objects; programar call site fora de HTTP perde invariantes.
3. **Schema único compartilhado (e.g. JSON Schema declarado fora do código)**.
   - Vantagens: validação coerente entre stack diferentes.
   - Desvantagens: overhead de duas fontes de verdade; lib Go imatura para JSON Schema; complexidade injustificada para foundation.

## Consequências

### Benefícios Esperados

- Mensagens de erro 400 ricas e localizáveis (campo + razão).
- Domain seguro: nenhum primitivo cru entra (catch em compile-time via assinatura).
- Conformidade com R-DDD-001 + security-app.md sem violação.
- Object Calisthenics #3 (encapsular primitivos) e #9 (sem getters/setters mecânicos) preservados.

### Trade-offs e Custos

- Dois "lugares" de validação: equipe precisa entender a divisão (mitigado por README + ADR).
- Tags de validator podem se acumular em DTOs grandes (sinal para quebrar DTO).

### Riscos e Mitigações

- **Risco:** duplicação acidental de regra (validator + VO checam a mesma coisa).
  - **Mitigação:** convenção: validator só cobre forma sintática; VO só cobre semântica; review de PR enforça.
- **Risco:** VO sem cobertura ⇒ regra de negócio quebra silenciosa.
  - **Mitigação:** testes unitários table-driven obrigatórios para cada VO (R-TEST-001 §Validadores de input).
- **Risco:** validator pode mudar mensagem em upgrade quebrando testes que assert texto.
  - **Mitigação:** testes asserem código de campo, não texto.

## Plano de Implementação

1. Adicionar `github.com/go-playground/validator/v10` ao `go.mod`.
2. `internal/infrastructure/http`: helper `Bind(r *http.Request, dst any) error` faz JSON decode + validate; retorna `validator.ValidationErrors`.
3. `internal/infrastructure/errors/problem.go`: mapear `validator.ValidationErrors` → ProblemDetails 400 com `extensions.fields = [{field, code, message}]`.
4. Convenção para PRDs subsequentes: DTO em `adapters/dto/`, VO em `domain/`, mapper em `application/mapper.go`.

## Monitoramento e Validação

- Métrica `http.server.request.error.count{status="400"}` (alerta se >10% sustentado em 1h).
- Logs de validação em `info` (não `error`) para evitar ruído.

## Impacto em Documentação e Operação

- `AGENTS.md`: padrão DTO → VO → Domain documentado.
- README de cada módulo (scaffold): exemplo de DTO + VO + mapper.

## Revisão Futura

- Revisitar se a equipe reportar dúvidas frequentes sobre "onde validar X".
- Considerar gerador de DTO a partir de VO (codegen) se o boilerplate crescer demais.
