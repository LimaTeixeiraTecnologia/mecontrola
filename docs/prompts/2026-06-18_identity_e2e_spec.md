# Prompt Refinado: Implementação de Testes E2E para o Módulo identity

## Contexto
Você é um Engenheiro Go Sênior. Sua tarefa é projetar e planejar a implementação de 100% de cobertura de testes E2E para o módulo `internal/identity`, garantindo que ele atenda aos mais altos padrões arquiteturais. Você deve basear seu trabalho nos padrões encontrados em `internal/billing` e `internal/e2e`, particularmente no que diz respeito a fluxos orientados a eventos e validação de outbox.

> **Expectativa de entrega:** fechar **100% do módulo `internal/identity` com confiança operacional**. "Confiança operacional" significa que cada camada possui um teste que **falha de verdade** quando o comportamento prometido quebra — não basta o teste existir e passar; ele precisa exercitar o caminho real (HTTP, producer, consumer, job, repositório) e **verificar o estado do banco** após a operação.

## Objetivos
1.  **Cobertura Exaustiva:** Identificar e planejar todos os cenários E2E possíveis para `internal/identity`, incluindo:
    *   **HTTP:** Todos os endpoints expostos (Listar, Criar, Atualizar, Deletar, Obter).
    *   **Consumer/Producer:** Cenários onde dados são ingeridos via eventos ou onde mudanças disparam notificações.
    *   **Casos de Erro:** 404 Not Found, 400 Bad Request, 401 Unauthorized (via `gatewayAuth`), e 422 Business Rule violations.
2.  **Validação de Outbox:** Se algum evento de domínio for disparado, valide rigorosamente seu despacho e consumo através do padrão outbox.
3.  **Integração com Banco de Dados:** Use `internal/platform/database/postgres/test_helper.go` para subir uma instância real de PostgreSQL via Testcontainers.
4.  **Domain Modeling Made Functional (DMMF):** Aplique os princípios de DMMF (de `@.agents/skills/go-implementation/**`) onde aplicável.
5.  **Pirâmide Completa de Testes:** Não limite o plano a E2E godog. A cobertura "100%" só é atingida com as três camadas:
    *   **Unit** — domínio (smart constructors, `Decide*` puro) e use cases (happy path + cada erro propagado), sem IO.
    *   **Integração** — repositório, producer, consumer e job handler contra **PostgreSQL real** (Testcontainers).
    *   **E2E (godog)** — jornada de negócio ponta-a-ponta com banco real e verificação de outbox.
6.  **Validação de Banco Obrigatória (criou? atualizou?):** Para toda operação de escrita, o teste DEVE **consultar o banco via SQL após a operação** e afirmar o efeito real: linha **criada**, **atualizada**, **soft-deleted** (`deleted_at`) ou **version incrementada**. Nunca confiar apenas no valor de retorno do método sob teste.
7.  **Confiança Operacional:** O módulo só é considerado 100% coberto quando cada item da "Matriz de Cobertura Obrigatória" tem teste correspondente que falha se o comportamento regredir, e quando todos os gates do "Definition of Done" passam.

## Matriz de Cobertura Obrigatória
Cada linha abaixo é um conjunto mínimo de cenários que DEVE existir antes de declarar o módulo coberto. Adapte os caminhos `identity` à realidade descoberta no "Inventário a Descobrir".

| Camada | Tipo | Cenários mínimos | Verificação de banco | Exemplo canônico no repo |
|--------|------|------------------|----------------------|--------------------------|
| HTTP handler | Unit (mock UC) | sucesso (201/200); 400 payload inválido; 401 `gatewayAuth`; 404; 422 regra de negócio | — (adapter fino, sem SQL) | `internal/transactions/infrastructure/http/server/handlers/handlers_test.go` |
| Use case | Unit (mockery) | happy path + **cada** caminho de erro propagado | via mock `.EXPECT()...Once()` | `internal/identity/application/usecases/find_user_by_id_test.go` |
| Domínio | Unit puro (sem mock) | smart constructors acumulando **todos** os erros; `Decide*` puro e determinístico | — | `internal/transactions/domain/services/transaction_workflow_test.go`, `internal/transactions/domain/commands/create_transaction_test.go` |
| Repositório | Integração (Postgres real) | `Create`→`GetByID` confere campos; update com optimistic lock (version++ **e** conflito); soft-delete some da listagem; paginação cursor roundtrip; agregações | `QueryRowContext` direto na tabela | `internal/transactions/infrastructure/repositories/postgres/transaction_repository_integration_test.go` |
| Producer / Outbox | Integração | publica na **mesma tx** → linha em `outbox_events` com `event_type`/`aggregate_id`/`user_id` corretos; **rollback → não persiste** | `ClaimBatch`/`SELECT` em `outbox_events` | `internal/transactions/infrastructure/messaging/database/producers/transaction_event_publisher_integration_test.go` |
| Consumer | Integração + Unit | processa evento → linha criada/atualizada; **idempotência por `event_id`** (reprocessar não duplica); coalescing/dedup quando aplicável | `SELECT COUNT(*)` por `event_id` | `internal/identity/infrastructure/messaging/database/consumers/auth_events_consumer_integration_test.go`, `internal/transactions/infrastructure/messaging/database/consumers/monthly_summary_recompute_consumer_integration_test.go` |
| Job handler | Integração | execução dupla → idempotente (sem linha duplicada); timezone; config de cron/nome | `InsertIfAbsent` retorna `false` no 2º run; `SELECT COUNT(*)` | `internal/transactions/infrastructure/jobs/handlers/recurring_materializer_job_integration_test.go` |
| E2E godog | E2E (Postgres real) | jornada de negócio ponta-a-ponta + verificação de outbox | persistência + outbox via steps | `internal/e2e`, `internal/categories/e2e` |

## Validação de Banco de Dados (criou? atualizou?)
*   **PostgreSQL real:** suba o banco com `internal/platform/database/postgres/test_helper.go` (`NewTestDatabase`) ou `internal/platform/testcontainer`. Cada teste recebe um DB isolado com as migrations aplicadas e `t.Cleanup` removendo o banco.
*   **Build tag obrigatória:** todo teste de integração/E2E que sobe container DEVE declarar `//go:build integration` (ou `e2e`). Testes unitários rodam com `-short` e não dependem de Docker.
*   **Padrão de asserção:** crie helpers na suite (ex.: `countX`, `latestX`) que executam `QueryRowContext` **diretamente na tabela após** a operação. Espelhe `countOutboxByType` (`establish_principal_integration_test.go`) e `countAuthEvents` (`auth_events_consumer_integration_test.go`). A asserção precisa cobrir explicitamente: **criou** (`COUNT == before+1`), **atualizou** (campo/`version` mudou), **soft-delete** (`deleted_at IS NOT NULL` e some da listagem), **idempotência** (`COUNT` estável após reprocesso).

## Orquestração com Subagents (Obrigatório)
*   Em cobertura/refactor amplos, **paralelizar por camada é obrigatório**; trabalho sequencial no main loop é proibido (regra do repositório, alinhada a `AGENTS.md`).
*   Spawne **1 subagent por linha da Matriz de Cobertura** (domínio, usecase, repositório, handler, producer, consumer, job, E2E), rodando em paralelo, cada um responsável por planejar/escrever e validar a sua camada.
*   Faça uma **síntese final** consolidando as evidências de cada subagent (testes criados, gates executados, estado do banco verificado) antes de declarar o módulo 100% coberto.

## Confiança Operacional — Definition of Done
Antes de afirmar "módulo fechado", TODOS os gates abaixo devem passar e ser reportados como evidência:
*   `task test:unit` (com `-race`), `task test:integration` (requer Docker) e `task test:e2e` **verdes**.
*   `golangci-lint run` e `go vet` **limpos** no escopo alterado.
*   Gates de regra do repositório retornando vazio: **zero comentários** em `.go` de produção (R-ADAPTER-001.1); **sem SQL direto** em adapter (R-ADAPTER-001.2); para `transactions`, **regra de domínio fora de `Decide*` bloqueia** (R-TXN-001..004).
*   **Idempotência por `event_id`** validada em todo consumer (contrato Outbox do `AGENTS.md`).
*   **Sem falso positivo (inegociável):** se um teste quebra, encontre a **causa raiz** e corrija o **código**; nunca relaxe, pule ou comente o teste para "passar".

## Inventário a Descobrir (antes de escrever testes)
Mapeie o módulo real `internal/identity` e liste explicitamente — cobertura "100%" sem este inventário é cega:
*   **Endpoints HTTP** e o router (`infrastructure/http/server/`): método, rota, handler, status esperados.
*   **Use cases** (`application/usecases/`) e suas interfaces/ports.
*   **Eventos de domínio** + **producers** (`messaging/database/producers/`) e **consumers** (`messaging/database/consumers/`): tipo de evento, payload, `event_id`.
*   **Jobs** (`infrastructure/jobs/handlers/`): nome, schedule, idempotência.
*   **Tabelas e migrations** tocadas pelo módulo.
*   **Mocks gerados** declarados em `.mockery.yml`.

## Referências Canônicas
*   `.agents/skills/go-implementation/SKILL.md` e `references/`: `testing-unit.md`, `testing-integration.md`, `examples-testing.md`, `INDEX.yaml`.
*   `.claude/rules/go-adapters.md` (adaptadores finos + zero comentários) e `.claude/rules/transactions-workflows.md` (DMMF `Decide*`).
*   `internal/platform/database/postgres/test_helper.go` e `internal/platform/testcontainer` (Postgres real).
*   `.mockery.yml` (geração de mocks).
*   Arquivos de teste exemplares citados na Matriz de Cobertura.

## Restrições Obrigatórias
*   **Zero Comentários:** NÃO inclua nenhum comentário no código Golang.
*   **Divisão de Idioma:**
    *   **Gherkin & Regex:** Devem estar em **Português (PT-BR)**.
    *   **Métodos/Steps Go:** Devem estar em **Inglês**.
*   **Melhores Práticas Go:** Siga o estilo idiomático "GoLike", favorecendo simplicidade, tipos sobre interfaces (a menos que necessário) e estrutura de pastas clara.
*   **Sem Implementação Imediata:** Apenas forneça o plano completo e os detalhes de alinhamento feature/código. NÃO modifique nenhum arquivo ainda.
*   **Validação e Correção Mandatória:** Você DEVE realmente testar o que a funcionalidade promete. Se o teste quebrar, você DEVE analisar criteriosamente a causa e, se o código for problemático, corrigi-lo. Isso é mandatório e inegociável.

## Estrutura de Saída Esperada
1.  **Arquivos Gherkin (.feature):** Cenários detalhados em PT-BR.
2.  **Definições de Steps (Go):** Assinaturas de métodos em Inglês com regex em PT-BR.
3.  **Estrutura de Pastas:** Proponha uma organização limpa sob `internal/identity/e2e` ou `internal/e2e`.
4.  **Estratégia de Evidência de Validação:** Explique como o outbox e o estado do banco de dados serão verificados.

## Exemplo de Cenário
```gherkin
# language: pt
Funcionalidade: [NOME DA FUNCIONALIDADE]

  Cenário: [DESCRIÇÃO DO CENÁRIO]
    Dado que o ambiente de teste para identity está pronto
    Quando o usuário executa uma ação de negócio
    Então o sistema deve persistir o estado corretamente no banco
    E deve disparar o evento correspondente no outbox
```

## Exemplo de Cenário (com validação de banco, outbox e idempotência)
```gherkin
# language: pt
Funcionalidade: Persistência e propagação de eventos em identity

  Cenário: criação persiste a linha e enfileira o evento no outbox
    Dado que o ambiente de teste para identity está pronto
    E que não existe nenhum registro para o usuário autenticado
    Quando o usuário cria um recurso válido via HTTP
    Então a resposta HTTP deve ser 201 Created
    E o banco deve conter exatamente 1 linha nova para o recurso
    E a tabela outbox_events deve conter 1 evento com o event_type esperado

  Cenário: reprocessar o mesmo evento é idempotente
    Dado que o evento já foi processado uma vez pelo consumer
    Quando o mesmo evento (mesmo event_id) é reprocessado
    Então o banco deve continuar com exatamente 1 linha
    E nenhuma duplicata deve ser criada
```

---
**Nota:** Este documento serve como base para a geração de testes E2E em qualquer módulo do projeto mecontrola.
</content>
</invoke>
