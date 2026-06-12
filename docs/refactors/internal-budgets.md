# Prompt enriquecido para `internal/budgets`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/budgets` em modo `advisory` por padrao, mantendo contratos, comportamento observado e fronteiras atuais. So execute mudancas se eu pedir `execution`.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. Toda proposta deve preservar rigorosamente a semantica atual.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Carregue `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias sob demanda conforme `task_type`, `required_refs`, `optional_refs` e `forbidden_refs`.
4. Use referencias de `agent-governance` apenas quando forem realmente necessarias para DDD, erros, testes, seguranca ou arquitetura.

Contrato de exploracao
1. Valide o baseline com `AGENTS.md`, `go.mod`, `internal/budgets/module.go` e os entrypoints reais do modulo.
2. Nao invente aggregates, eventos, jobs, consumers, producers, DTOs ou repositorios ausentes.
3. Considere explicitamente estas superficies reais: `domain/commands`, `domain/entities`, `domain/services`, `domain/valueobjects`, `application/usecases`, `application/interfaces`, `application/mappers`, `infrastructure/http/server`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database`, `infrastructure/repositories/postgres`, `infrastructure/config` e `module.go`.

Objetivo tecnico da refatoracao
1. Melhorar coesao, clareza de fluxo e protecao de invariantes do dominio de orcamentos.
2. Siga fielmente SOMENTE os principios de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#` que tragam ganho real em Go.
3. Priorize quando fizer sentido:
   - command objects mais explicitos em linguagem ubiqua;
   - smart constructors e invariantes fortes em entities e value objects;
   - workflows menores para criacao de budget, upsert/delete de expense, alertas, recorrencia e pending events;
   - domain events tipados quando a decisao do dominio realmente produzir fatos relevantes;
   - estados explicitos para alertas, pendencias e mutacoes que hoje possam estar implicitos;
   - separacao clara entre decisao de dominio, mapeamento e persistencia.
4. Quando eventos fizerem sentido, a decisao do evento deve nascer no dominio ou no passo puro de decisao, e producers devem apenas mapear/publicar o evento decidido.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, sobretudo a adocao seletiva de smart constructors, `Decide*` puro e publishers finos de domain events. Nao copie naming, aggregates ou contratos sem aderencia ao modulo real.
6. Nao force sealed unions ou state-as-type se isso complicar desnecessariamente a interoperacao com o restante do modulo.

Focos especificos para `budgets`
1. Avalie se `domain/commands` e `application/usecases` estao com responsabilidades bem separadas ou se ha orquestracao demais em uma unica camada.
2. Verifique se jobs, consumers e producers continuam finos e delegam para use cases.
3. Revise o uso de `application/mappers` para evitar logica de negocio acidental em transformacoes.
4. Verifique a fronteira com `categories_reader`, pending events, alerts e recorrencia para reduzir acoplamento e melhorar previsibilidade.
5. Avalie se eventos de dominio ou eventos pendentes estao decididos no lugar certo e se producers/publicadores permanecem como adapters finos.

Restricoes obrigatorias
1. Preserve o fluxo `infrastructure -> application -> domain`.
2. Nao mover regra de negocio para HTTP handlers, jobs, consumers, producers ou repositories.
3. Nao criar interfaces novas sem consumidor real.
4. Nao usar `panic`, `init()`, `clock.Clock` ou adaptacoes artificiais so para encaixar DMMF.
5. Se houver eventos de dominio, nao decidir trigger, semantica ou payload de negocio dentro do producer.
6. E proibido alterar qualquer comportamento existente; se a melhoria exigir mudanca de semantica de budget, expense, alert ou recorrencia, pare e explicite como fora de escopo.

Saida esperada
1. Classifique a mudanca com o `task_type` mais especifico.
2. Liste hotspots, invariantes e pontos de regressao provavel.
3. Proponha um plano incremental curto, com foco no menor diff seguro.
4. Em cada proposta, diga qual principio de DMMF esta sendo usado e por que ele se paga no modulo.
5. Liste validacoes proporcionais, testes alvo e riscos remanescentes.
6. Em `execution`, inclua review final e relatorio de refactor.

Criterios de aceitacao
- O plano deve referenciar apenas diretorios e artefatos reais de `internal/budgets`.
- O plano deve deixar claro o que fica em domain, application e infrastructure.
- Toda incorporacao de DMMF deve vir com trade-off e justificativa operacional.
- A resposta final deve terminar com `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- Foquei em `domain/commands`, `usecases`, `mappers` e eventos pendentes porque essas superficies existem de fato em `budgets`.
- Mantive DMMF como criterio adaptativo, nao como reescrita total do modulo.
- Especifiquei riscos semanticos de budget, expense e alert porque o modulo parece mais sensivel a regressao de dominio.
