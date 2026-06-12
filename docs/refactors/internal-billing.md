# Prompt enriquecido para `internal/billing`

## Prompt enriquecido

```text
Objetivo
Refatorar o modulo `internal/billing` em modo `advisory` por padrao, preservando comportamento publico, contratos e integracoes reais do workspace. So mude para `execution` se eu pedir explicitamente.
Mandatorio: nao mudar nenhum comportamento ja existente, nem publico, nem interno observavel, nem operacional. Toda proposta deve ser estritamente comportamentalmente equivalente ao estado atual.

Skills obrigatorias
1. Carregue `.agents/skills/refactor/SKILL.md`.
2. Como o escopo toca Go, carregue tambem `.agents/skills/go-implementation/SKILL.md`.
3. Carregue referencias apenas sob demanda, respeitando `AGENTS.md`, o limite de contexto e a matriz de `go-implementation`.
4. Se DDD, erros, seguranca ou testes forem relevantes, carregue tambem apenas as referencias necessarias de `agent-governance`.

Contrato de exploracao
1. Antes de propor qualquer mudanca, confirme o baseline lendo `AGENTS.md`, `go.mod`, `internal/billing/module.go` e somente os arquivos reais necessarios para mapear o fluxo atual.
2. Nao invente handlers, repositorios, eventos, consumers, jobs, adapters ou invariantes que nao existam no workspace.
3. Considere explicitamente as superficies reais do modulo: `application/usecases`, `application/interfaces`, `application/usecases/kiwifypayload`, `domain/entities`, `domain/services`, `domain/valueobjects`, `infrastructure/http/server`, `infrastructure/http/client`, `infrastructure/jobs/handlers`, `infrastructure/messaging/database`, `infrastructure/repositories/postgres` e `module.go`.

Objetivo tecnico da refatoracao
1. Identifique os pontos em que o modulo pode ficar mais coeso, testavel e previsivel sem alterar comportamento publico.
2. Siga fielmente SOMENTE o que fizer sentido de `Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F#`, adaptado ao contexto Go e ao codebase atual, sem copiar idiomatica de F# cegamente.
3. Priorize quando couber:
   - smart constructors para value objects e entidades com invariantes;
   - modelagem de estados mais explicita quando houver status com campos exclusivos por variante;
   - pipelines de workflow menores dentro de use cases longos;
   - domain events tipados quando o modulo realmente decidir fatos de dominio relevantes;
   - separacao mais nitida entre decisao de dominio, orquestracao de aplicacao e adapters de infraestrutura;
   - erros semanticos e transicoes de estado mais explicitas.
4. Quando eventos fizerem sentido, trate a decisao do evento como responsabilidade do dominio ou do passo puro de decisao, e deixe producers como adapters finos que apenas mapeiam `domain event -> outbox/envelope`.
5. Use como inspiracao estrutural, se necessario, `.specs/prd-transactions-monthly`, especialmente a adocao seletiva de smart constructors, passo `Decide*` puro e domain events tipados. Nao transplante artefatos ou naming sem correspondencia no modulo.
6. Nao force DMMF quando isso aumentar indirecao, custo de teste ou complexidade operacional sem ganho claro.

Focos especificos para `billing`
1. Revisar se processamento de webhook Kiwify, reconciliacao, expiracao/grace e notificacoes mantem regra de negocio dentro de use cases e deixa handlers, jobs e consumers finos.
2. Verificar se parsing de payload, transicoes de subscription e publish de eventos estao em fronteiras coerentes.
3. Verificar se eventos de dominio do modulo, quando existirem ou forem introduzidos de forma justificada, nascem na decisao de dominio/aplicacao e nao dentro do producer.
4. Verificar se repositories Postgres e clients HTTP estao servindo apenas como adapters, sem decisao de negocio.
5. Revisar `module.go` para garantir DI manual explicita no padrao do repositorio.

Restricoes obrigatorias
1. Preserve o fluxo `infrastructure -> application -> domain`.
2. Nao mova regra de negocio para handlers, consumers, jobs, producers ou repositories.
3. Nao crie interfaces sem consumidor real.
4. Nao introduza `panic`, `init()`, `clock.Clock`, `var _ Interface = (*Type)(nil)` ou globais novas.
5. Se houver eventos de dominio, producers devem permanecer finos e nao decidir `event_type`, semantica, trigger ou payload de negocio fora da decisao de dominio.
6. E proibido alterar qualquer comportamento existente; se a melhoria exigir mudanca comportamental, pare e reporte como fora de escopo.

Saida esperada
1. Classifique o `task_type` de `go-implementation` antes de carregar referencias extras.
2. Liste dores atuais, invariantes, riscos e oportunidades.
3. Proponha o menor plano seguro de refatoracao em passos incrementais.
4. Para cada passo, diga quais principios de DMMF entram, quais nao entram e por que.
5. Defina validacoes proporcionais minimas e evidencias de nao regressao.
6. Se estiver em `execution`, exija review final e relatorio de refactor conforme a skill `refactor`.

Criterios de aceitacao
- Nenhuma sugestao deve depender de artefato inexistente no workspace.
- O plano deve citar arquivos ou diretorios reais de `internal/billing`.
- O plano deve preservar contratos publicos salvo instrucao explicita em contrario.
- Toda incorporacao de DMMF deve ser justificada por ganho concreto de modelagem, clareza ou robustez operacional.
- A resposta final deve terminar com um estado entre `done`, `needs_input`, `blocked` ou `failed`.
```

## Justificativas curtas

- Adicionei o modo `advisory` por padrao para alinhar com a skill `refactor`.
- Explicitei as superficies reais de `billing` para evitar alucinacao estrutural.
- Restrinji a incorporacao de DMMF ao que fizer sentido em Go e no modulo atual, sem reescrita ideologica.
- Fixei criterios de aceitacao e saida final para tornar o prompt deterministico.
