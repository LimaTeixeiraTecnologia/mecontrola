# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estratégia de testes e conformidade (E2E real atrás de flag + integração testcontainers + gates de governança)
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-44, RF-45, RF-46; Métricas de Sucesso), techspec, ADR-002, ADR-004, ADR-005, `R-TESTING-001`

## Contexto

O PRD exige prova de production-ready (não claims): consumidor de referência + suite de conformidade, com OpenRouter real e Postgres real, mas sem tornar o CI caro/flaky. O projeto já usa flag de LLM real (ex.: `RUN_REAL_LLM`) e integration tests com banco real. O storage usa pgvector (ADR-004/005). As regras de governança foram alteradas e precisam de gates reemitidos para os caminhos finais.

## Decisão

Três camadas de teste:

1. **Unit** (sempre no CI): testify/suite whitebox com `dependencies` + IIFE e `fake.NewProvider()` (R-TESTING-001) para use cases; tabelas de cenário para domínio puro (tipos fechados, decode, sampling). Mock só de serviços externos (OpenRouter).
2. **Integração** (`//go:build integration`, no CI com containers): `testcontainers-go` com imagem `pgvector/pgvector:pg16` (PG16 alinhado à produção, que adiciona a extensão `vector` ao `deployment/docker/Dockerfile.postgres`), migrations `000001..000003` aplicadas; cobre `Store`/CAS, suspend/resume por merge-patch, `SemanticRecall` (HNSW real), `WorkingMemory`, `platform_runs`/`platform_scorer_results`, indexação assíncrona via `outbox`/`worker`, e up/down da `000003`.
3. **E2E de conformidade** (`test/conformance/weather`): consumidor weather portado para Go exercitando todas as capacidades. Variante **real** atrás de `RUN_REAL_LLM=1` (OpenRouter real + Postgres real), executada sob demanda/nightly, **fora do gate de merge**; sem a flag, roda determinística (provider fake + testcontainers) e os passos que exigem LLM real fazem `t.Skip`.

Adicionalmente, **gates de governança reemitidos** (grep) rodam no CI apontando para `internal/platform/{agent,memory,llm,scorer,tool}` e para o kernel (import/LLM/comentários/cardinalidade), conforme techspec.

## Alternativas Consideradas

- **OpenRouter real sempre no CI.** Desvantagem: caro, lento, flaky no gate de merge. Rejeitada.
- **Sempre mockado, real só manual ad-hoc.** Desvantagem: não garante o "uso real do OpenRouter" de forma reproduzível. Rejeitada.
- **Fake in-memory da porta Store (sem Postgres real).** Desvantagem: não exercita SQL/migrations/pgvector — onde mora o risco. Rejeitada para integração.

## Consequências

### Benefícios Esperados

- Prova real de production-ready com cobertura E2E mapeada por RF.
- CI determinístico e barato no gate de merge; real sob demanda.
- Governança verificável por automação (gates).

### Trade-offs e Custos

- Manutenção de testcontainers e do consumidor de referência.
- Suite real depende de credencial/quota OpenRouter.

### Riscos e Mitigações

- **Risco:** flakiness do real vazar para o gate. **Mitigação:** real isolado por flag, nunca no gate de merge.
- **Risco:** divergência entre fake e real. **Mitigação:** mesmos cenários E2E rodam em ambos os modos; nightly real detecta drift.

## Plano de Implementação

1. Configurar testcontainers Postgres+pgvector e build tag `integration`.
2. Portar weather (agent/tool/workflow/scorer) para Go em `test/conformance/weather`.
3. Implementar a suite com gate por `RUN_REAL_LLM`.
4. Adicionar os gates grep de governança ao pipeline.

## Monitoramento e Validação

- Cobertura por RF (tabela de mapeamento da techspec) verde no modo determinístico; nightly real verde.
- Critério de sucesso: todas as capacidades nucleares exercitadas E2E; gates grep vazios.

## Impacto em Documentação e Operação

- Runbook de CI/CD documenta as três camadas, a flag `RUN_REAL_LLM` e os gates.

## Revisão Futura

- Revisitar a frequência do nightly real e o conjunto de cenários conforme custo/observado.
