# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Conciliação streaming × structured output (validação na conclusão do stream)
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma, autor do PRD
- **Relacionados:** PRD (RF-03, RF-04, RF-05, RF-06, RF-07, RF-08), techspec, ADR-002

## Contexto

O PRD exige, no MVP, execução síncrona **e** streaming, e structured output validável na fronteira como capacidade não negociável. Há tensão conhecida: structured output (JSON schema strict) só é integralmente validável com a resposta completa; o streaming entrega tokens incrementais. Hoje o client OpenRouter (`client.go`) faz request/response com `response_format: json_schema, strict: true` e não tem streaming.

## Decisão

Em streaming, os deltas de texto são entregues incrementalmente via canal (`ResultStream.Deltas()`); o conteúdo é acumulado e, **na conclusão do stream**, quando há `StructuredContract[T]` declarado, o resultado é validado contra o schema. `Result(ctx)` só retorna após o fechamento do canal e, em caso de não-conformidade, retorna **erro explícito e auditável** (nunca resultado não conforme). Em execução síncrona, a validação ocorre imediatamente sobre a resposta completa. O provider `llm.Stream` consome SSE (`stream: true`); o agente não expõe structured output "parcial" como contrato — apenas texto incremental + resultado estruturado final.

## Alternativas Consideradas

- **Validação parcial incremental (JSON parcial a cada chunk).** Vantagem: feedback estruturado contínuo. Desvantagem: fronteira ambígua, alto custo/risco, schema strict não suporta validação parcial confiável. Rejeitada para o MVP.
- **Caminhos separados (streaming só texto livre; structured só síncrono).** Vantagem: simplicidade. Desvantagem: quebra paridade Mastra (lá coexistem) e fragmenta o contrato do agente. Rejeitada.

## Consequências

### Benefícios Esperados

- Mantém ambas as exigências do MVP sem suavizar nenhuma.
- Contrato de agente único (sync e stream compartilham `Result`).
- Falha determinística e auditável quando o contrato não conforma.

### Trade-offs e Custos

- O resultado estruturado não está disponível durante o stream, apenas ao final.
- Acúmulo de buffer do stream em memória até a conclusão.

### Riscos e Mitigações

- **Risco:** stream truncado/incompleto gera JSON inválido. **Mitigação:** detectar `finish_reason: length`/erro upstream e retornar erro explícito; limite de tokens configurável.
- **Risco:** consumo de memória com respostas longas. **Mitigação:** limite superior de bytes acumulados; erro explícito ao exceder.

## Plano de Implementação

1. Implementar `llm.Stream` (SSE) em `internal/platform/llm`.
2. Implementar `agent.Stream` com acúmulo + validação final via `StructuredContract`.
3. Testar fim-de-stream conforme/não-conforme (unit) e E2E weather (`translationScorer`, structured).

## Monitoramento e Validação

- Métrica `agent_stream_total` por `status` (`completed`/`invalid_contract`/`truncated`).
- Critério de sucesso: 100% das saídas estruturadas validadas; falha explícita medida em teste.

## Impacto em Documentação e Operação

- Techspec e contrato do agente documentam a semântica fim-de-stream.

## Revisão Futura

- Revisitar se houver demanda real por validação parcial incremental (novo ADR).
