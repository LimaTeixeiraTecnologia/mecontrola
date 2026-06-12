# Prompt enriquecido para `internal/identity`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/identity` em modo `advisory` por padrao, preservando contratos publicos, fronteiras entre modulos e comportamento observado. So execute mudancas se eu pedir `execution`.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. Toda proposta deve ser comportamentalmente equivalente ao estado atual.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Carregue `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias apenas sob demanda, conforme o `task_type` e a mudanca realmente analisada.
4. Se a refatoracao tocar DDD, seguranca, erros ou testes, carregue somente as referencias necessarias de `agent-governance`.

Contrato de exploracao
1. Confirme baseline com `AGENTS.md`, `go.mod`, `internal/identity/module.go` e entrypoints reais.
2. Nao invente provedores, políticas, eventos, adapters ou regras de autorizacao que nao existam.
3. Considere explicitamente estas superficies reais: `application/auth`, `application/usecases`, `application/interfaces`, `domain/entities`, `domain/valueobjects`, `domain/services`, `domain/pii`, `infrastructure/http/server`, `infrastructure/http/client`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database`, `infrastructure/repositories/postgres` e `module.go`.

Objetivo tecnico da refatoracao
1. Melhorar a modelagem de identidade, entitlement, auth events e fluxo de principal sem mudar comportamento publico.
2. Siga fielmente SOMENTE o que fizer sentido de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#` no contexto Go.
3. Priorize quando fizer sentido:
   - smart constructors e invariantes fortes para IDs, email, WhatsApp, user e correlatos;
   - estados ou resultados de auth/entitlement mais explicitos quando houver semantica exclusiva por variante;
   - pipelines menores para use cases como `establish_principal`, `upsert_user_by_whatsapp`, `project_auth_event` e `project_subscription_event`;
   - domain events tipados quando o modulo realmente decidir fatos de dominio relevantes;
   - tratamento semantico de PII, masking e auth events sem espalhar regra de negocio.
4. Quando eventos fizerem sentido, a decisao do evento deve nascer no dominio/aplicacao e nao no producer ou adapter.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, principalmente a adocao seletiva de smart constructors, `Decide*` puro e domain events tipados. Nao transplante artefatos ou naming sem aderencia ao modulo.
6. Nao force unions ou abstrações extras se o ganho de clareza nao compensar.

Focos especificos para `identity`
1. Verifique se handlers, jobs e consumers permanecem finos e delegam para use cases.
2. Revise a relacao entre `application/auth`, entidades de usuario, entitlement e projections.
3. Verifique se eventos de autenticacao, projeções de subscription e PII estao em fronteiras corretas.
4. Avalie se os eventos do modulo estao modelados como fatos de dominio tipados quando isso trouxer ganho real, mantendo producers finos.
5. Revise `module.go` para manter DI manual explicita no padrao exigido pelo repositorio.

Restricoes obrigatorias
1. Preserve contratos com outros modulos e comportamento de autenticacao/autorizacao ja existente.
2. Nao mover regra de negocio para adapters HTTP, jobs, consumers ou repositories.
3. Nao criar interfaces sem consumidor real.
4. Nao introduza `panic`, `init()`, `clock.Clock` ou padroes que dificultem auditoria e seguranca.
5. Se houver eventos de dominio, producers nao podem decidir trigger, payload semantico ou identidade do evento.
6. E proibido alterar qualquer comportamento existente; se a melhoria puder mudar decisao de entitlement, anonimização, auth event ou resolucao de principal, pare e declare fora de escopo.

Saida esperada
1. Classifique o `task_type`.
2. Liste hotspots, invariantes, riscos de regressao e cruzamentos com outros modulos.
3. Proponha o menor plano seguro de refatoracao.
4. Em cada passo, explique o ganho concreto de DMMF e o trade-off.
5. Liste validacoes proporcionais, testes alvo e evidencias de nao regressao.
6. Em `execution`, inclua review final e relatorio de refactor.

Criterios de aceitacao
- O plano deve referenciar apenas artefatos reais de `internal/identity`.
- O plano deve preservar isolamento do dominio e contratos cross-module.
- Toda recomendacao de DMMF deve ser justificada por ganho de modelagem, seguranca ou robustez.
- A resposta final deve terminar com `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- `identity` cruza auth, PII, projections e outros modulos, entao explicitei os riscos de regressao e seguranca.
- Direcionei DMMF para estados e invariantes onde ele pode agregar valor real.
- Mantive o foco em adapters finos e DI manual porque isso e regra dura do repositorio para o modulo.
