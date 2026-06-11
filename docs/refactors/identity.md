# Refactor Prompt Enriquecido — `internal/identity`

## Prompt original

> Utilize factories ou algum padrão de projeto que faça sentido. Estou vendo muita regra, muito parser. Utilize DDD tático, procure referência de programação funcional para ajudar a melhorar. Uso obrigatório de `@.claude/skills/go-implementation/`, carregando sob demanda apenas `architecture.md` + `interfaces.md` + `examples-domain-flow.md` e `testing.md` quando reescrever suites, com máximo de 4 referências simultâneas. Foco em eficiência, robustez, production-ready e sem falso positivo.

## Prompt enriquecido

Você vai refatorar exclusivamente o módulo `internal/identity`.

Antes de editar:

1. Carregue `AGENTS.md`.
2. Carregue obrigatoriamente `.claude/skills/go-implementation/SKILL.md`.
3. Carregue somente:
   - `.claude/skills/go-implementation/references/architecture.md`
   - `.claude/skills/go-implementation/references/interfaces.md`
   - `.claude/skills/go-implementation/references/examples-domain-flow.md`
   - `.claude/skills/go-implementation/references/testing.md` apenas se houver reescrita de suites
4. Verifique `go.mod` e registre drift se algum script esperado pela skill não existir.

Arquivos prioritários:

- `internal/identity/module.go`
- `internal/identity/application/usecases/establish_principal.go`
- `internal/identity/application/usecases/upsert_user_by_whatsapp.go`
- `internal/identity/application/usecases/project_auth_event.go`
- `internal/identity/application/usecases/project_subscription_event.go`
- `internal/identity/infrastructure/messaging/database/consumers/`
- `internal/identity/infrastructure/http/server/handlers/`

Problema a atacar:

- O módulo mistura parsing, decisão, publicação de evento e persistência em use cases centrais.
- Há lógica de classificação de erro e construção de payload de evento acoplada ao fluxo principal.
- O bootstrap monta projeções e consumidores de forma funcional, mas ainda com alto acoplamento implícito entre responsabilidades.

Objetivo da refatoração:

- Tornar os fluxos `EstablishPrincipal` e `UpsertUserByWhatsApp` mais explícitos e menos ramificados.
- Isolar criação de eventos de autenticação e regras de decisão em componentes com responsabilidade única.
- Preservar o modelo de DI manual do módulo, sem introduzir composição genérica ou APIs tipo `NewModule(opts...)`.
- Reduzir parsing e transformação incidental perto do caso de uso.

Direção sugerida:

- Avalie factory functions concretas para criação de auth events ou command/result objects quando isso remover branching do use case.
- Avalie um domain service ou application service pequeno para decisão de resolução de principal e classificação de outcomes.
- Avalie funções puras para:
  - classificar erro por reason
  - montar payload/outbox input
  - mapear resultado para DTO
- Se o ganho for real, extraia uma fronteira consumidora pequena para construção/publicação de eventos, mas não crie interface apenas para mock.

Restrições:

- Não empurre regra para handlers HTTP nem consumers.
- Não use factories abstratas, builder genérico ou facade sem benefício mensurável.
- Não altere semântica de erros públicos sem declarar.
- Não introduza `clock.Clock` em use case.
- Se houver escrita com parâmetros primitivos excessivos, considere command objects com linguagem ubíqua.

Critérios de aceitação:

- `EstablishPrincipal` e `UpsertUserByWhatsApp` ficam menores ou com responsabilidades mais nítidas.
- A criação de eventos sai do fluxo principal quando isso reduzir complexidade sem esconder regra.
- Consumers e handlers continuam finos.
- Sem novas interfaces especulativas.
- Testes atualizados de forma proporcional; carregue `testing.md` só se reestruturar suites.

Formato esperado da resposta final do executor:

1. Quais responsabilidades foram separadas.
2. Qual padrão foi aplicado e por que ele faz sentido neste módulo.
3. Como a mudança evitou falso positivo arquitetural.
4. Evidências de validação.

## Justificativas do enriquecimento

- O prompt foca nos use cases que hoje concentram parsing, classificação de erro e construção de evento.
- A sugestão de funções puras é local e pragmática, útil para reduzir ruído sem “funcionalizar” o módulo inteiro.
- O wiring em `module.go` já é próximo do padrão aceito pelo repositório, então a orientação é preservar isso e refinar os hotspots.

## Variante curta

> Refatore `internal/identity` com foco em `establish_principal.go` e `upsert_user_by_whatsapp.go`, separando parsing, decisão e construção de eventos com DDD tático, factory functions concretas quando houver invariantes reais e funções puras locais para classificação/mapeamento. Carregue obrigatoriamente `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md` e `testing.md` apenas se reescrever suites. Preserve DI manual, adapters finos e evite interfaces artificiais.

## Referências

- `architecture.md`, `interfaces.md`, `examples-domain-flow.md` da skill `go-implementation`
- Go Blog: Errors are values — https://go.dev/blog/errors-are-values
- Go 1.13 Errors — https://go.dev/blog/go1.13-errors
- Rob Pike: Self-referential functions and the design of options — https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
