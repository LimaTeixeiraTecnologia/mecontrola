# Refactor Prompt Enriquecido — `internal/billing`

## Prompt original

> Utilize factories ou algum padrão de projeto que faça sentido. Estou vendo muita regra, muito parser. Utilize DDD tático, procure referência de programação funcional para ajudar a melhorar. Uso obrigatório de `@.claude/skills/go-implementation/`, carregando sob demanda apenas `architecture.md` + `interfaces.md` + `examples-domain-flow.md` e `testing.md` quando reescrever suites, com máximo de 4 referências simultâneas. Foco em eficiência, robustez, production-ready e sem falso positivo.

## Prompt enriquecido

Você vai refatorar exclusivamente o módulo `internal/billing`.

Antes de editar:

1. Carregue `AGENTS.md`.
2. Carregue `.claude/skills/go-implementation/SKILL.md`.
3. Carregue somente:
   - `architecture.md`
   - `interfaces.md`
   - `examples-domain-flow.md`
   - `testing.md` apenas se reescrever suites
4. Verifique `go.mod` e registre drift de automação ausente.

Arquivos prioritários:

- `internal/billing/module.go`
- `internal/billing/application/usecases/process_kiwify_webhook.go`
- `internal/billing/application/usecases/reconcile_subscriptions.go`
- `internal/billing/domain/valueobjects/status.go`
- `internal/billing/domain/valueobjects/plan.go`
- `internal/billing/infrastructure/http/server/handlers/kiwify_webhook_handler.go`
- `internal/billing/infrastructure/messaging/database/producers/subscription_event_publisher.go`

Problema a atacar:

- O módulo concentra parsing, validação de configuração, conversão temporal, decisão por tipo de evento e wiring operacional.
- `process_kiwify_webhook.go` acumula parsing de payload, classificação de evento, extração de tracking token e resolução de timestamps.
- `module.go` também concentra regra incidental de configuração e bootstrap de planos.

Objetivo da refatoração:

- Reduzir complexidade acidental do fluxo de webhook e reconciliação.
- Encapsular parsing e criação de objetos sem mover regra para handlers.
- Tornar a composição de casos de uso e políticas de evento mais explícita.
- Manter o módulo production-ready, robusto a payloads variados e sem falsos positivos arquiteturais.

Direção sugerida:

- Avalie factory functions ou mappers concretos para converter webhook bruto em command objects semânticos do domínio/aplicação.
- Avalie strategy map ou dispatch explícito por tipo de evento somente se isso reduzir branching real em `ProcessKiwifyWebhook`.
- Avalie value objects/funções puras para:
  - parse de status
  - resolução de timestamps
  - detecção de carrier de funnel token
  - normalização de payload Kiwify
- Considere extrair a validação/configuração de planos do bootstrap para componente concreto próprio se isso reduzir regra incidental em `module.go`.
- Use programação funcional apenas como apoio local em pipelines de transformação pura, sem esconder efeitos colaterais.

Restrições:

- Não mover regra de negócio para handler HTTP, job ou producer.
- Não criar abstrações genéricas de webhook sem necessidade concreta.
- Não piorar a rastreabilidade de erros.
- Não quebrar a semântica atual de observabilidade, idempotência e publicação de eventos.

Critérios de aceitação:

- `process_kiwify_webhook.go` fica mais previsível e com menos responsabilidade acumulada.
- O bootstrap de billing perde regra incidental onde fizer sentido.
- Value objects e factories ficam ancorados em invariantes reais.
- Testes de webhook e reconciliação continuam cobrindo casos críticos; `testing.md` só entra se houver reescrita estrutural de suites.

Formato esperado da resposta final do executor:

1. Onde estavam parser/regra acidental.
2. Qual padrão foi usado e por que ele reduz complexidade real.
3. Como a solução preserva idempotência, observabilidade e handlers finos.
4. Evidências de validação.

## Justificativas do enriquecimento

- O prompt foi centrado nos dois maiores hotspots atuais: `module.go` e `process_kiwify_webhook.go`.
- Aqui factories e mapeadores concretos tendem a ser mais úteis que interfaces novas.
- A parte funcional foi delimitada a transformações puras de payload e decisão, onde ela pode reduzir ruído sem comprometer clareza.

## Variante curta

> Refatore `internal/billing` focando em `module.go` e `application/usecases/process_kiwify_webhook.go`, removendo parser e regra incidental do fluxo principal por meio de factories concretas, value objects e funções puras locais para normalização de payload, status e timestamps. Carregue `AGENTS.md`, `.claude/skills/go-implementation/SKILL.md`, `architecture.md`, `interfaces.md`, `examples-domain-flow.md` e `testing.md` apenas se reescrever suites. Preserve handlers finos, idempotência, observabilidade e semântica pública.

## Referências

- `architecture.md`, `interfaces.md`, `examples-domain-flow.md` da skill `go-implementation`
- Go Blog: Errors are values — https://go.dev/blog/errors-are-values
- Go Blog: Working with Errors in Go 1.13 — https://go.dev/blog/go1.13-errors
- Go Blog: Go Concurrency Patterns: Pipelines and cancellation — https://go.dev/blog/pipelines
- Rob Pike: Self-referential functions and the design of options — https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
