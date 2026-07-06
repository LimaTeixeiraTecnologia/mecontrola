<!-- review-prompt-enriched: 1 -->
# Prompt de Revisão Criteriosa — PRD Contrato Determinístico de Categorias para Transações Agentivas

**Data:** 2026-07-06
**Identificador:** `review-prd-contrato-categorias-transacoes-agentivas`
**Arquivo:** `docs/reviews/2026-07-06-review-prd-contrato-categorias-transacoes-agentivas.md`
**PRD base:** `.specs/prd-contrato-categorias-transacoes-agentivas/prd.md`
**Especificação técnica:** `.specs/prd-contrato-categorias-transacoes-agentivas/techspec.md`
**Tarefas:** `.specs/prd-contrato-categorias-transacoes-agentivas/tasks.md`
**ADRs:** ADR-001 a ADR-006 no mesmo diretório
**Execution reports:** `1.0_execution_report.md`, `2.0_execution_report.md`, `3.0_execution_report.md`

---

## Prompt Original (entrada)

> Objetivo: Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilização, validando estritamente contra `.specs/prd-contrato-categorias-transacoes-agentivas`.
>
> Critérios obrigatórios:
> - Todos os critérios de aceite atendidos (implementados).
> - DoD 100% atendido (implementados).
> - 0 gaps.
> - 0 lacunas.
> - 0 falsos positivos.
> - 0 ressalvas.
> - Todas as Regras de negócio atendidas (implementadas).
>
> Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
> Dispare subagentes especializados quando agregarem qualidade à revisão.
> Não implemente nada. Apenas execute a revisão e gere o parecer final.

---

## Prompt Enriquecido (prompt de execução)

### 1. Contexto do produto e escopo

A feature **Contrato Determinístico de Categorias para Transações Agentivas** estabelece um contrato funcional único, auditável e bloqueante para categorias em transações de despesa e receita. O objetivo de produto é **0 falso positivo conhecido na escrita**: qualquer categoria que não seja canonicamente resolvida, inequívoca, ativa e compatível com o tipo da transação deve bloquear a persistência e exigir clarificação explícita do usuário.

- **Autoridade canônica:** `internal/categories` (catálogo, dicionário, candidatos, `Outcome`, versão editorial).
- **Consumidor agentivo/classificador:** `internal/agents` (`RegisterEntry.classify`, tool `classify_category`, workflow de confirmação retomável).
- **Gate final de persistência:** `internal/transactions` (`CreateTransaction`, `UpdateTransaction`, `CreateRecurringTemplate`, `UpdateRecurringTemplate` via `CategoryWriteGate`).

Superfícies de escrita obrigatórias:
- Criação e edição de transações.
- Criação e edição de templates recorrentes.
- Fluxos agentivos (`RegisterEntry`, `destructive_confirm_workflow`) e não agentivos (write manual com IDs).
- Tool `classify_category` como adapter explicativo, não autoridade de escrita.

### 2. Insumos obrigatórios

Antes de emitir qualquer parecer, carregue e traceie:

1. `prd.md`: requisitos funcionais RF-01 a RF-35, critérios de aceite CA-01 a CA-23, critérios de sucesso mensuráveis, escopo incluído/excluído, decisões fixadas.
2. `techspec.md`: componentes modificados, contratos (`CategorySearchResult`, `CategoryCandidate`, `ResolveCategoryForWriteInput/Output`, `CategoriesReader`, `CategoryWriteGate`, `CategoryWriteEvidence`, `CategoryDecisionSource`), regras de gate, schema SQL, triggers, matriz de testes.
3. `tasks.md`: tarefas 1.0 a 8.0, dependências, status, cobertura de requisitos.
4. ADRs: ADR-001 a ADR-006 (gate único, evidência normalizada, source enum, bloqueio de drift, folha obrigatória, defesa de banco).
5. Execution reports 1.0, 2.0, 3.0 (status de entrega parcial).
6. Codebase nos paths listados em `techspec.md` → verificar implementação real.

### 3. Critérios absolutos de aprovação

A revisão só pode terminar com `APPROVED` se, e somente se:

- [ ] Todos os critérios de aceite CA-01 a CA-23 estão implementados e comprováveis no código.
- [ ] Todas as regras de negócio (seção 7 do PRD e regras de gate da techspec) estão implementadas.
- [ ] Todos os requisitos funcionais RF-01 a RF-35 estão atendidos.
- [ ] DoD 100% atendido: código, testes, migrations, observabilidade e documentação de decisões conforme tarefas 1.0–8.0.
- [ ] **0 gaps:** nenhum requisito/CA/RF foi omitido sem justificativa documentada.
- [ ] **0 lacunas:** nenhum comportamento crítico está apenas parcialmente implementado.
- [ ] **0 falsos positivos:** nenhuma escrita é autorizada sem evidência canônica completa.
- [ ] **0 ressalvas:** não são aceitos workarounds, TODOs não resolvidos, comentários de débito técnico ou comportamentos “quase atendidos”.
- [ ] Nenhum bypass do gate: triggers, FKs e constraints do banco reforçam o contrato mesmo se o use case for contornado.
- [ ] Testes unitários, de integração e E2E cobrem a matriz de bloqueio/aceite (CA-01 a CA-23).
- [ ] Mocks e testes duplos não mascaram estados inválidos que a integração real rejeitaria.

### 4. Processo de revisão (passo a passo)

Execute `@.claude/skills/review/` seguindo rigorosamente:

1. **Carga base:** leia `prd.md`, `techspec.md`, `tasks.md`, ADRs e execution reports. Produza mapa de rastreabilidade RF/CA → arquivo → teste.
2. **Revisão por bounded context:**
   - `internal/categories`: `SearchDictionary` expõe `Outcome`, `Version`, evidence fields e `HasMore`; `ResolveCategoryForWrite` valida IDs, raiz, folha, kind, active/deprecated, versão editorial.
   - `internal/agents`: `CategoriesReader` transporta contrato rico; `RegisterEntry.classify` aplica gate completo; `classify_category` retorna candidatos, outcome, version, `writeDecision`; workflow de confirmação não usa primeiro candidato e revalida escolha.
   - `internal/transactions`: `CategoryWriteGate` implementado; VOs `CategoryWriteEvidence` e `CategoryDecisionSource` com smart constructors; quatro use cases de write exigem evidência; updates revalidam categoria atual; entidades e repositories persistem evidência completa.
   - `migrations/000001_initial_schema.up.sql`: `subcategory_id` NOT NULL, colunas de evidência, FKs, checks, triggers semânticos.
3. **Revisão de testes:** verifique existência e assertividade de testes unitários, integration com `testcontainers-go` e E2E. Cada CA deve ter pelo menos um teste.
4. **Revisão de observabilidade:** métricas `category_write_gate_total`, `category_write_version_drift_total`, `category_write_persisted_total`, `category_clarification_requested_total` com labels permitidas.
5. **Verificação de anti-padrões:**
   - string livre para outcome/source/confidence/quality/signal_type;
   - `panic`, `init()`, interfaces fictícias, `clock.Clock`;
   - LLM/scorer autorizando escrita;
   - fallback para categoria genérica/primeira categoria/similaridade textual;
   - regras de negócio em handlers/tools/consumers/jobs;
   - reimplementação de primitivos de plataforma.
6. **Parecer intermédio:** se encontrar qualquer problema, classifique como `REJECTED`. Não aprove parcialmente.
7. **Ciclo de correção:** se `REJECTED`, invoque `@.claude/skills/bugfix/` para cada problema encontrado. Após correções, repita a revisão completa (review → bugfix → review) até `APPROVED`.
8. **Parecer final:** apenas `APPROVED` quando todos os critérios absolutos forem verdadeiros sem ressalvas.

### 5. Disparo de subagentes especializados

Dispare subagentes quando isso aumentar a profundidade ou reduzir o risco de falso positivo:

- **Subagente `categories`**: focado em `internal/categories`, contratos canônicos, `SearchDictionary`, `ResolveCategoryForWrite`, ADRs de gate/drift/folha.
- **Subagente `agents`**: focado em `internal/agents`, tool `classify_category`, `RegisterEntry`, workflow de confirmação, adapters ricos, conformidade Mastra.
- **Subagente `transactions`**: focado em `internal/transactions`, `CategoryWriteGate`, VOs, entidades, repositories, migrations, triggers.
- **Subagente `tests`**: focado em cobertura de testes unit/integração/E2E, mocks realistas, matriz CA.
- **Subagente `architecture`**: focado em DMMF, Go implementation, fronteiras de camada, anti-padrões.

Cada subagente deve retornar evidências concisas (arquivo:linha, teste, status) e não emitir parecer final isoladamente. O parecer final é do agente coordenador.

### 6. Formato de saída obrigatório

Gere um relatório em markdown com:

- **Resumo executivo:** status final `APPROVED` ou `REJECTED`.
- **Matriz de rastreabilidade:** tabela com RF/CA/ADR → implementação encontrada → teste → status.
- **Diagnóstico detalhado:** para cada não conformidade:
  - Severidade: `bloqueante`, `alta`, `média`.
  - Local: `path/to/file.go:linha` ou `migrations/...`.
  - Evidência: trecho de código, erro de teste, ou ausência detectada.
  - Regra violada: RF/CA/ADR.
  - Sugestão de correção.
- **Lista de bugs para bugfix:** se `REJECTED`, itens acionáveis para o ciclo de correção.
- **Plano de re-revisão:** ordem de reexecução após correções.

### 7. Restrições inegociáveis

- **Não implemente código, migrations, handlers, rotas ou testes durante a revisão.** A revisão é somente leitura e parecer.
- **Não aceite ressalvas, "falta apenas...", "será feito depois" ou TODOs não resolvidos.**
- **Não confie apenas em testes unitários com mocks:** exija integração real com `categories` + Postgres para gates críticos.
- **Não permita que LLM, scorer ou texto livre do usuário desbloqueiem escrita sem nova resolução canônica.**
- **Não aprove se houver qualquer string livre para outcome, source, confidence, quality ou signal_type em fronteiras de escrita.**
- **Não finalize com `APPROVED` se houver gaps, lacunas ou falsos positivos.**

### 8. Instrução final de execução

> Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilização, validando estritamente contra `.specs/prd-contrato-categorias-transacoes-agentivas` e este prompt enriquecido. Aplique todos os critérios absolutos de aprovação. Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo review → bugfix → review até obter `APPROVED`, sem falsos positivos e em conformidade total com a especificação. Dispare subagentes especializados quando agregarem qualidade à revisão. Não implemente nada; apenas execute a revisão e gere o parecer final no formato obrigatório.
