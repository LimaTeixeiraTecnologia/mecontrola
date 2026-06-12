# Prompt enriquecido para `internal/onboarding`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/onboarding` em modo `advisory` por padrao, preservando o comportamento do fluxo de checkout, token magico, ativacao e integracoes existentes. So mude para `execution` se eu pedir explicitamente.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. Toda proposta deve manter equivalencia comportamental estrita.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Carregue `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias apenas sob demanda, respeitando `AGENTS.md` e a matriz da skill Go.
4. Use referencias de `agent-governance` apenas quando arquitetura, DDD, seguranca, erros ou testes exigirem.

Contrato de exploracao
1. Confirme o baseline com `AGENTS.md`, `go.mod`, `internal/onboarding/module.go` e os entrypoints reais do modulo.
2. Nao invente gateways, eventos, use cases, handlers ou estados que nao estejam no workspace.
3. Considere explicitamente estas superficies reais: `application/binding`, `application/events`, `application/interfaces`, `application/services`, `application/usecases`, `domain/entities`, `domain/services`, `domain/valueobjects`, `infrastructure/checkout`, `infrastructure/gateway`, `infrastructure/http/client`, `infrastructure/http/server`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database`, `infrastructure/messaging/database/producers`, `infrastructure/repositories/postgres` e `module.go`.

Objetivo tecnico da refatoracao
1. Melhorar a modelagem do fluxo de onboarding sem mudar comportamento publico nem contratos assincronos.
2. Siga fielmente SOMENTE o que fizer sentido de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#`, adaptado ao contexto Go e ao sistema atual.
3. Priorize quando fizer sentido:
   - smart constructors para tokens, status e value objects com invariantes;
   - modelagem de estado mais explicita para ciclo de vida de magic token e sinais de suporte;
   - workflows menores e mais claros para criacao de checkout session, ativacao, expiracao, outreach e consumo de eventos pagos;
   - domain events tipados quando o modulo realmente decidir fatos de dominio relevantes;
   - separacao nitida entre decisao de dominio, binding/eventos de aplicacao e adapters de infraestrutura;
   - idempotencia, transicoes e erros de negocio mais explicitos.
4. Quando eventos fizerem sentido, a decisao do evento deve nascer no dominio/aplicacao e producers devem apenas mapear/publicar o evento decidido.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, sobretudo a adocao seletiva de smart constructors, `Decide*` puro e domain events tipados. Nao transplante contratos ou naming sem correspondencia real.
6. Nao force state-as-type ou unions onde isso conflitar com custo de manutencao ou integracao do modulo.

Focos especificos para `onboarding`
1. Verifique se jobs, consumers e handlers continuam finos e deixam a orquestracao em use cases.
2. Revise a modelagem de `magic_token`, `token_status`, `activation_path` e transicoes de dominio.
3. Verifique se gateways HTTP, checkout e producers publicam ou consomem apenas o que a aplicacao ja decidiu.
4. Avalie se eventos do modulo devem ser formalizados como domain events tipados e se a decisao deles esta fora do producer.
5. Revise `application/events` e `application/binding` para garantir fronteiras claras e sem duplicacao semantica.

Restricoes obrigatorias
1. Preserve contratos publicos, contratos assincronos e semantica do fluxo de onboarding.
2. Nao mover regra de negocio para handlers, consumers, jobs, producers, gateways ou repositories.
3. Nao criar interfaces sem consumidor real.
4. Nao usar `panic`, `init()`, `clock.Clock` ou abstrações artificiais.
5. Se houver eventos de dominio, producer deve ser apenas adapter fino e nao o lugar onde o evento e decidido.
6. E proibido alterar qualquer comportamento existente; se a melhoria puder alterar transicoes de token, expiracao, outreach, checkout ou eventos publicados, pare e declare fora de escopo.

Saida esperada
1. Classifique o `task_type` antes de carregar referencias extras.
2. Liste hotspots, invariantes, transicoes e riscos de regressao.
3. Proponha um plano incremental pequeno e seguro.
4. Explique onde DMMF ajuda no fluxo e onde nao compensa.
5. Liste validacoes proporcionais, testes de regressao e evidencias minimas.
6. Em `execution`, inclua review final e relatorio de refactor.

Criterios de aceitacao
- O plano deve citar apenas caminhos reais de `internal/onboarding`.
- A modelagem proposta deve reforcar invariantes e idempotencia sem quebrar integracoes.
- Cada uso de DMMF deve ser justificado por ganho concreto de clareza, robustez ou operabilidade.
- A resposta final deve terminar com `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- Direcionei o prompt para estado, transicoes e idempotencia porque esse modulo tem fluxo temporal e assincrono mais forte.
- Citei `application/events`, `binding`, jobs, consumers e producers porque existem e sao superficies criticas para regressao.
- Mantive a incorporacao de DMMF condicionada a ganho real, nao a remodelagem total do fluxo.
