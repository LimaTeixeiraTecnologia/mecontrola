# Refactor Prompt Enriquecido — `internal/categories`

## Prompt original

> Utilize factories ou algum padrão de projeto que faça sentido. Estou vendo muita regra, muito parser. Utilize DDD tático, procure referência de programação funcional para ajudar a melhorar. Uso obrigatório de `@.claude/skills/go-implementation/`, carregando sob demanda apenas `architecture.md` + `interfaces.md` + `examples-domain-flow.md` e `testing.md` quando reescrever suites, com máximo de 4 referências simultâneas. Foco em eficiência, robustez, production-ready e sem falso positivo.

## Prompt enriquecido

Você vai refatorar exclusivamente o módulo `internal/categories`.

Pré-condições obrigatórias:

1. Carregue `AGENTS.md`.
2. Carregue `.claude/skills/go-implementation/SKILL.md`.
3. Carregue somente:
   - `architecture.md`
   - `interfaces.md`
   - `examples-domain-flow.md`
   - `testing.md` só se reescrever suites
4. Verifique `go.mod` e registre drift de automação ausente em vez de inventar ferramentas.

Arquivos prioritários:

- `internal/categories/module.go`
- `internal/categories/application/usecases/search_dictionary.go`
- `internal/categories/infrastructure/http/server/handlers/search_dictionary_handler.go`
- `internal/categories/domain/services/`
- `internal/categories/domain/valueobjects/`

Problema a atacar:

- O módulo é relativamente enxuto, mas concentra normalização, classificação de outcome, regras de bucket e fallback de resposta em handler e use case.
- Existe risco de espalhar regra de consulta entre handler, use case e serviço sem uma fronteira clara.

Objetivo da refatoração:

- Consolidar regras de busca e decisão sem aumentar indireção desnecessária.
- Preservar a simplicidade do módulo.
- Reduzir parser/regra incidental em handler HTTP.
- Melhorar legibilidade e testabilidade do fluxo de busca sem criar falso positivo arquitetural.

Direção sugerida:

- Avalie mover normalização, classificação de resultado e bucketização para funções puras ou serviços pequenos do domínio/aplicação quando isso reduzir duplicação.
- Considere uma factory function concreta apenas se ela encapsular invariantes úteis de construção de output, query object ou resultado sem inflar o design.
- Priorize funções puras para:
  - normalização da query
  - classificação de outcome
  - cálculo de bucket de tamanho
  - montagem de resposta derivada
- Preserve o handler como adapter fino; ele pode traduzir input/output e métricas, mas não deve concentrar regra semântica de busca.

Restrições:

- Não transformar um módulo simples em arquitetura excessiva.
- Não criar interfaces novas se os tipos concretos atuais já bastam.
- Não introduzir factory por estética.
- Não misturar regra de domínio com detalhes de ETag, HTTP status ou envelope de erro.

Critérios de aceitação:

- `search_dictionary_handler.go` perde regra incidental ou fica claramente mais fino.
- `search_dictionary.go` fica mais previsível sem crescer em acoplamento.
- A solução preserva performance e evita consultas extras desnecessárias.
- Testes seguem proporcionais; `testing.md` só entra se houver reestruturação real de suites.

Formato esperado da resposta final do executor:

1. O que foi mantido simples e por quê.
2. O que foi extraído e por quê.
3. Como a mudança melhora robustez sem overengineering.
4. Evidências de validação.

## Justificativas do enriquecimento

- `categories` pede refino tático, não redesenho pesado.
- A principal oportunidade está em funções puras e encapsulamento de decisão, não em grandes patterns.
- O prompt força o executor a evitar abstração “bonita” mas desnecessária, reduzindo falso positivo.

## Variante curta

> Refatore `internal/categories` focando em `search_dictionary.go` e `search_dictionary_handler.go`, removendo regra incidental de normalização, classificação de outcome e bucketização para funções puras ou serviços pequenos, com DDD tático e factories concretas apenas se encapsularem invariantes reais. Carregue `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md` e `testing.md` somente se reescrever suites. Preserve simplicidade e adapters finos.

## Referências

- `architecture.md`, `interfaces.md`, `examples-domain-flow.md` da skill `go-implementation`
- Go Blog: Errors are values — https://go.dev/blog/errors-are-values
- Go Blog: Go Concurrency Patterns: Context — https://go.dev/blog/context
- Rob Pike: Self-referential functions and the design of options — https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
