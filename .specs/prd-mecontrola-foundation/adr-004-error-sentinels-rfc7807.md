# ADR-004 — Modelo de erros: sentinels por módulo + RFC 7807 no boundary

## Metadados

- **Título:** Erros sentinel por módulo, wrapping com `fmt.Errorf %w`, tradução para Problem Details RFC 7807 no boundary HTTP
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RNF-001+, §Restrições](./prd.md), [techspec §Estratégia de Erros](./techspec.md), [R-ERR-001](../../.agents/skills/agent-governance/references/error-handling.md)

## Contexto

O `devkit-go/pkg/http_server` já entrega middleware de Problem Details RFC 7807. Falta padronizar como cada módulo expressa erros para que o middleware faça a tradução correta sem mapeamento ad-hoc.

R-ERR-001 manda: sentinels ou tipos bem definidos por módulo, wrapping preservando cadeia (`errors.Is`/`errors.As`), apresentação traduzida ao usuário, nunca expor stack/SQL.

## Decisão

- **Cada módulo define seus próprios sentinels** em `<modulo>/domain/errors.go` (ex.: `var ErrUserNotFound = errors.New("identity: user not found")`).
- **Adapters wrappam erros de driver** com `fmt.Errorf("inserting user %q: %w", id, err)`.
- **Application propaga sem reescrever** mensagem (preserva intenção do domain).
- **Middleware HTTP** (`internal/infrastructure/errors/problem.go`) traduz sentinel conhecido → `ProblemDetails` específico; default 500 com mensagem genérica.
- **Sem sentinels globais cross-module**: rejeitado para preservar modularização.

## Alternativas Consideradas

1. **Error types ricos (struct com Code/Message/Cause) por módulo**.
   - Vantagens: mais expressivo; carrega contexto para apresentação.
   - Desvantagens: encoraja domain a conhecer códigos HTTP (anti-pattern); mais boilerplate; violação latente de Object Calisthenics #9.
2. **Sentinels globais centralizados em `internal/errors`**.
   - Vantagens: lista única, fácil de auditar.
   - Desvantagens: viola modularização hexagonal; toda mudança em um módulo força commit em pacote compartilhado (acoplamento estático).
3. **Apenas `fmt.Errorf` sem sentinels** (string match no boundary).
   - Vantagens: sem boilerplate.
   - Desvantagens: violação explícita de R-ERR-001 §Comparação ("não comparar erro por string"); frágil em refactor.

## Consequências

### Benefícios Esperados

- Conformidade total com R-ERR-001.
- Isolamento por módulo (Epic 04 não força mudanças em Epic 07).
- Mapper centraliza a política de exposição (sem stack/SQL vazando).
- `errors.Is`/`errors.As` em testes garante asserções estáveis.

### Trade-offs e Custos

- Cada novo sentinel exige atualização do mapper (custo baixo; PR isolado).
- Risco de mapper ficar desatualizado: mitigado por teste de mapper table-driven (cada sentinel ⇒ status code esperado).

### Riscos e Mitigações

- **Risco:** dev cria sentinel sem registrar no mapper ⇒ retorna 500 silencioso.
  - **Mitigação:** Teste de cobertura de mapeamento em `problem_test.go`; review de PR conforme `enforcement-matrix.md`.
- **Risco:** mensagem do sentinel vaza informação sensível.
  - **Mitigação:** Mapper substitui `Detail` por mensagem genérica em códigos 5xx; lista de mensagens "seguras" só para 4xx.
- **Risco:** wrap em chain longa esconde a causa raiz.
  - **Mitigação:** `slog.Error` no boundary loga a chain completa via `err.Error()`; usuário recebe só `ProblemDetails`.

## Plano de Implementação

1. `internal/infrastructure/errors/problem.go`: struct `ProblemDetails` + `ToProblemDetails(err) (ProblemDetails, int)`.
2. `internal/infrastructure/errors/problem_test.go`: table-driven cobrindo cada sentinel registrado (database, http, runtime) + default 500.
3. Middleware no `devkit-go/pkg/http_server` consumido em `internal/infrastructure/http/server.go` chama `ToProblemDetails`.
4. Convenção: cada PRD subsequente que crie sentinel deve registrar no mapper + adicionar caso ao teste.

## Monitoramento e Validação

- Métrica `http.server.request.error.count{problem_type}` (label custom).
- Alerta: 5xx >5% em 15 min.
- Log estruturado em error com chain completa + `trace_id` para correlação Grafana.

## Impacto em Documentação e Operação

- Runbook "Mapeamento de erros": lista de sentinels conhecidos ↔ ProblemDetails.
- Convenção em `AGENTS.md` (a expandir): "todo sentinel novo precisa de entrada no mapper + teste".

## Revisão Futura

- Revisitar se a quantidade de sentinels cruzar 50 (sinal de explosão).
- Considerar geração automática de mapper (codegen) se isso ocorrer.
