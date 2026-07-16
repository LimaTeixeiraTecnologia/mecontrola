# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Persistência dual-write do nome de tratamento (metadata estruturado + seção de working memory) com writer único
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Autor do PRD/techspec (produto + plataforma), confirmado pelo solicitante
- **Relacionados:** `.specs/prd-nome-de-tratamento-usuario/prd.md` (RF-03, RF-09, RF-11, RF-13), `techspec.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.8-A), ADR-002, ADR-003

## Contexto

- RF-03 exige persistir o nome de tratamento "de forma estruturada" e, ao mesmo tempo (RF-05/RF-09), disponibilizá-lo para o agente usar nas interações imediatamente.
- Fato de código decisivo: o runtime injeta no system prompt **apenas** o conteúdo de `working_memory` (`internal/platform/agent/runtime.go:308-312`), nunca o `metadata` JSONB. A interface `memory.WorkingMemory` (`internal/platform/memory/ports.go:18-22`) tem `Get→string` (conteúdo), `Upsert(content)` e `UpsertMetadata(map)`; `buildMessages` chama só `Get`. `InboundRequest` não carrega metadata (`internal/platform/agent/ports.go`).
- `WorkingMemory.Upsert` faz overwrite da coluna inteira (`SET working_memory = EXCLUDED.working_memory`, `working_memory_repository.go:58-60`); `UpsertMetadata` faz merge JSONB (`metadata || EXCLUDED.metadata`, `:89`).
- O onboarding-concluído é detectado por substring `## Objetivo Financeiro` no conteúdo (`resolve_onboarding_or_agent.go:78,119`) — o conteúdo é sentinel, não pode ser perdido.
- O padrão dual-write já existe para o objetivo financeiro (conteúdo + metadata) na conclusão do onboarding (`onboarding_workflow.go:1563-1572`) e no `goal-edit` (`goal_edit_workflow.go:182-189`).

## Decisão

Persistir o nome de tratamento nas duas superfícies de `platform_resources`:

1. Conteúdo `working_memory`: seção `## Nome de Tratamento` (markdown) — fonte de verdade para o comportamento conversacional, pois é o que alcança o LLM.
2. `metadata["nome_tratamento"]` (JSONB) — mirror estruturado para análise/consulta, gravado por `UpsertMetadata` (merge, seguro para múltiplos writers).

Regra hard de **writer único de conteúdo no onboarding**: nenhuma etapa do onboarding além da conclusão escreve `working_memory` via `Upsert`. O nome viaja no `OnboardingState.TreatmentName` e a conclusão compõe, num único `Upsert`, todas as seções (`## Nome de Tratamento` quando presente + `## Objetivo Financeiro`). Na edição pós-onboarding, a atualização usa substituição de seção (`replaceWorkingMemorySection`, encapsulando `goalEditReplaceSection`, `goal_edit_workflow.go:241-267`), que preserva as seções irmãs. Escopo: `internal/agents/application/workflows/` e `internal/agents/application/tools/`.

## Alternativas Consideradas

- **Somente `metadata` + alterar o runtime para injetar metadata no system prompt.** Vantagem: atende "estruturado" com um único write. Desvantagem: toca o primitivo genérico de plataforma (`internal/platform/agent`), aumentando o raio de regressão sobre todos os agentes; introduz um segundo canal de contexto. Rejeitada por violar 0-regressão e por ampliar superfície da plataforma sem necessidade.
- **Somente conteúdo `working_memory`.** Vantagem: simples, alcança o LLM. Desvantagem: ignora o requisito de "metadata estruturado" (RF-03) e a paridade com `objetivo_financeiro`. Rejeitada.
- **Escritas de conteúdo por múltiplas etapas (uma por seção).** Desvantagem: o overwrite de coluna faria uma etapa clobbar a outra e potencialmente perder o sentinel `## Objetivo Financeiro`. Rejeitada — writer único elimina a classe de bug.

## Consequências

### Benefícios Esperados

- Nome estruturado (metadata) e utilizável (conteúdo) sem alterar o kernel/plataforma.
- Paridade com o padrão consolidado (`objetivo_financeiro`), reduzindo carga cognitiva e risco.
- Sentinel de onboarding preservado por construção.

### Trade-offs e Custos

- Dois writes por atualização (conteúdo + metadata) — custo desprezível.
- Janela de inconsistência se o `UpsertMetadata` falhar após o `Upsert` de conteúdo: comportamento observável permanece correto (conteúdo é fonte de verdade); metadata cicatriza na próxima edição.

### Riscos e Mitigações

- Risco: regressão do sentinel/objetivo por escrita concorrente. Mitigação: writer único no onboarding + merge de seção na edição; testes de integração asseguram as duas seções e a chave; unit garante preservação do sentinel.
- Rollback: reverter os diffs; nenhuma migração de schema é introduzida (colunas já existem).

## Plano de Implementação

1. Adicionar `replaceWorkingMemorySection`/`workingMemorySectionBody` (aliases dos helpers de seção).
2. Compor as seções na conclusão do onboarding num único `Upsert`; adicionar `nome_tratamento` ao `UpsertMetadata`.
3. Edição usa merge de seção + `UpsertMetadata`.
4. Testes unit + integração (Postgres) validando conteúdo, metadata e sentinel.

## Monitoramento e Validação

- Métrica `agents_onboarding_treatment_name_total{outcome}`; erros de persistência em logs error.
- Critério de sucesso: teste de integração confirma `working_memory` com ambas as seções e `metadata->>'nome_tratamento'` populado; sentinel intacto.
- Revisar se algum dia o runtime passar a injetar metadata (tornaria o mirror suficiente sozinho).

## Impacto em Documentação e Operação

- Documentar a chave `nome_tratamento` e a seção `## Nome de Tratamento` no runbook de agents.
- Sem mudança de dashboards além do novo counter.

## Revisão Futura

- Revisitar se o contrato de `runtime.buildMessages` mudar (injeção de metadata) ou se surgir necessidade de múltiplas seções escritas fora da conclusão.
