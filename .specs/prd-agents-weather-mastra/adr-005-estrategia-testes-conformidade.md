# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Estratégia de testes e conformidade (determinístico no CI + variante `RUN_REAL_LLM`)
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD RF-29,30; techspec.md; ADR-008 de prd-platform-mastra

## Contexto

O agente weather exercita OpenRouter (LLM/embeddings) e Postgres+pgvector. O CI de merge não pode depender de rede LLM (custo/flakiness), mas a entrega precisa de prova end-to-end real. Já existe `test/conformance/weather` (port Go parcial) e o padrão `RUN_REAL_LLM` do projeto.

## Decisão

CI padrão **determinístico**: provider LLM fake + `testcontainers` Postgres `pgvector/pgvector:pg16` com migrations `000001..000003`; cobre unit + integração (memória/recall/runs/scorers/indexação idempotente). Uma **variante E2E real** atrás de `RUN_REAL_LLM=1` (OpenRouter + Postgres reais) roda sob demanda/nightly, fora do gate de merge. A suite `test/conformance/weather` é **promovida** para exercitar o `internal/agents` real (não mocks). Gates de governança (import sem `internal/agent`; zero comentários nas camadas novas; cardinalidade; tipos fechados; gofmt `lint:fmt:check`) são bloqueantes.

## Alternativas Consideradas

- **LLM real no gate de merge**: rejeitada — custo e flakiness; quebra determinismo.
- **Só unit (sem integração)**: rejeitada — IO crítico (pgvector/recall) exige Postgres real para correção.

## Consequências

### Benefícios Esperados
- Merge gate rápido e determinístico; prova real disponível sob flag.
- Conformidade weather vira prova viva do consumidor.

### Trade-offs e Custos
- Manter testcontainers + variante real (operação/nightly).

### Riscos e Mitigações
- Recall só provado em integração (não no CI default unit): rodar integração no pipeline (job dedicado), não só localmente.
- Divergência mock vs real: variante `RUN_REAL_LLM` periódica.

## Plano de Implementação

1. Promover `test/conformance/weather` para `internal/agents` real.
2. Integração `//go:build integration` (testcontainers pgvector) para memória/recall/runs/indexação.
3. E2E inbound WhatsApp determinístico + variante real sob `RUN_REAL_LLM`.
4. Gates de governança/gofmt no CI.

## Monitoramento e Validação

- CI verde (unit+integração); variante real verde sob demanda; gates verdes.

## Impacto em Documentação e Operação

- Runbook de testes; como rodar `RUN_REAL_LLM`; job de integração no CI.

## Revisão Futura

- Revisar cadência da variante real e cobertura conforme novos consumidores.
