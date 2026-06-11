# Refactor Prompt Enriquecido — `internal/onboarding`

## Prompt original

> Utilize factories ou algum padrão de projeto que faça sentido. Estou vendo muita regra, muito parser. Utilize DDD tático, procure referência de programação funcional para ajudar a melhorar. Uso obrigatório de `@.claude/skills/go-implementation/`, carregando sob demanda apenas `architecture.md` + `interfaces.md` + `examples-domain-flow.md` e `testing.md` quando reescrever suites, com máximo de 4 referências simultâneas. Foco em eficiência, robustez, production-ready e sem falso positivo.

## Prompt enriquecido

Você vai refatorar exclusivamente o módulo `internal/onboarding` deste monólito modular Go.

Antes de qualquer edição:

1. Carregue obrigatoriamente `AGENTS.md`.
2. Carregue obrigatoriamente `.claude/skills/go-implementation/SKILL.md`.
3. Carregue somente estas referências, respeitando o limite máximo de 4 simultâneas:
   - `.claude/skills/go-implementation/references/architecture.md` obrigatória
   - `.claude/skills/go-implementation/references/interfaces.md` porque haverá avaliação de novas factories e fronteiras
   - `.claude/skills/go-implementation/references/examples-domain-flow.md` para manter o esqueleto `domain -> service -> usecase -> adapter`
   - `.claude/skills/go-implementation/references/testing.md` somente se você realmente reescrever suites ou estratégia de testes
4. Verifique `go.mod` e registre qualquer drift de tooling. Se algum script obrigatório da skill não existir no workspace, registre o drift explicitamente e prossiga sem inventar substituto.

Escopo real a inspecionar primeiro:

- `internal/onboarding/module.go`
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/application/usecases/try_fallback_activation.go`
- `internal/onboarding/application/usecases/get_token_state.go`
- `internal/onboarding/infrastructure/http/server/handlers/`
- `internal/onboarding/infrastructure/jobs/handlers/`
- `internal/onboarding/infrastructure/messaging/database/consumers/`
- `internal/onboarding/infrastructure/checkout/kiwify_url_builder.go`

Problema a atacar:

- Hoje o módulo concentra parsing e composição em `module.go`, com funções como `parseCheckoutURLs`, `parseCSV` e `buildMessagesMap`.
- O fluxo de consumo do token mistura regras de decisão, persistência, integração com `identity`, binding de assinatura e publicação de evento.
- Há risco de falsos positivos se a refatoração empurrar regra de negócio para handlers, jobs, consumers ou producers, o que é proibido pela governança do repositório.

Objetivo da refatoração:

- Reduzir branching e parsing espalhados.
- Extrair regras de decisão para serviços de domínio ou application services pequenos e explícitos.
- Introduzir factories apenas onde houver ganho concreto de invariantes, legibilidade de wiring ou eliminação de duplicação.
- Manter adapters finos e preservar o fluxo `handler/consumer/job -> usecase -> repository/service/client`.
- Melhorar robustez operacional sem alterar comportamento público sem evidência.

Restrições arquiteturais:

- Não criar abstrações especulativas.
- Não mover regra para `infrastructure/http/server/handlers`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database/consumers` ou `producers`.
- Interface nova somente no consumidor real, pequena e com motivo claro.
- Em use cases de escrita, preferir command objects com linguagem ubíqua se a assinatura atual estiver excessivamente primitiva.
- Não introduzir `init()`, `panic`, `log.Fatal`, `os.Exit` ou globals mutáveis.

Direção sugerida de design:

- Avalie uma factory concreta de configuração do onboarding para transformar strings cruas em objetos já validados, evitando parsing repetido no bootstrap.
- Avalie um domain service ou application service para a política de consumo de token, separando:
  - resolução do estado do token
  - decisão de transição
  - criação de support signal
  - criação/publicação do evento `onboarding.subscription_bound`
- Avalie funções puras pequenas para normalização e mapeamento quando não houver dependência externa. Use programação funcional apenas como apoio local e explícito, não como estilo dominante do módulo.
- Se houver ganho claro, use factory function para objetos com invariantes e decorator apenas para comportamento transversal real. Não use abstract factory.

Critérios de aceitação:

- `module.go` fica menos carregado de parser/regra incidental.
- O fluxo principal de `ConsumeMagicToken` fica mais legível e com responsabilidade melhor separada.
- Handlers, jobs e consumers continuam finos.
- Não surgem interfaces artificiais.
- Testes cobrindo comportamento alterado são atualizados de forma proporcional.
- Validação final proporcional inclui no mínimo build, vet e testes do escopo alterado; se suites forem reestruturadas, então carregue `testing.md`.

Formato esperado da resposta final do executor:

1. Resumo objetivo da causa raiz encontrada.
2. Plano curto do refactor escolhido e por que ele reduz acoplamento.
3. Diff implementado.
4. Riscos evitados explicitamente para não gerar falso positivo.
5. Evidências de validação executadas.

## Justificativas do enriquecimento

- O prompt foi ancorado em arquivos reais do módulo para evitar abstração genérica.
- O foco em `module.go` e `ConsumeMagicToken` vem dos hotspots atuais de parser, wiring e branching.
- A orientação de programação funcional foi limitada a funções puras locais, o que combina melhor com Go e reduz risco de overengineering.

## Variante curta

Use esta variante se quiser um prompt mais operacional:

> Refatore `internal/onboarding` reduzindo parser e branching em `module.go` e `application/usecases/consume_magic_token.go`, usando DDD tático, factories concretas apenas quando houver invariantes reais e funções puras locais para normalização/mapeamento. Carregue obrigatoriamente `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md` e `testing.md` só se reescrever suites. Preserve adapters finos, evite interfaces artificiais, não altere comportamento público sem evidência e entregue validação proporcional.

## Referências

- `architecture.md`, `interfaces.md`, `examples-domain-flow.md` da skill `go-implementation`
- Go Blog: Errors are values — https://go.dev/blog/errors-are-values
- Go Blog: Go Concurrency Patterns: Pipelines and cancellation — https://go.dev/blog/pipelines
- Rob Pike: Self-referential functions and the design of options — https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
