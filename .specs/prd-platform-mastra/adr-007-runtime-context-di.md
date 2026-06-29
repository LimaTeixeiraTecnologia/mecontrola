# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Runtime context tipado (DI) via `context.Context`, não persistido
- **Data:** 2026-06-29
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-26, RF-27), techspec, ADR-001, `R-WF-KERNEL-001.7`

## Contexto

O Mastra oferece runtimeContext: injeção de dependências e valores efêmeros de request acessíveis a steps, agents e tools durante uma execução, sem fazer parte do estado durável. O kernel atual compartilha apenas o estado genérico `S` (serializado no snapshot via `Codec`). É preciso adicionar DI sem contaminar o estado durável nem violar o merge-patch de resume (que opera só sobre `S`).

## Decisão

Modelar runtime context como valor tipado carregado por `context.Context` (`WithRuntime`/`RuntimeFrom`), acessível a steps/agents/tools. Ele **nunca** é serializado: o `Codec` continua codificando apenas `S`; o snapshot e o merge-patch de resume operam exclusivamente sobre `S` (preserva `R-WF-KERNEL-001.7`). Dados que precisam sobreviver a suspend/resume devem ser colocados em `S` explicitamente pelo consumidor; runtime context é estritamente efêmero (conexões, clients, correlação de request, flags). O kernel apenas repassa o `ctx` aos steps — não conhece o tipo concreto do runtime context.

## Alternativas Consideradas

- **Colocar dependências em `S`.** Desvantagem: polui o estado durável, serializa o que não deve persistir (clients/conexões não são serializáveis), incha o snapshot. Rejeitada.
- **Singleton/global de dependências.** Desvantagem: dificulta teste, acopla, quebra isolamento por execução. Rejeitada.
- **Parâmetro extra em `Step.Execute`.** Desvantagem: muda a assinatura pública do kernel e quebra steps existentes. Rejeitada em favor de `context.Context` (idiomático).

## Consequências

### Benefícios Esperados

- Paridade com runtimeContext do Mastra.
- DI testável e isolada por execução.
- Estado durável permanece enxuto e serializável.

### Trade-offs e Custos

- Valores em `context.Context` são não-tipados na fronteira; exige helper tipado para segurança.
- Disciplina: o que precisa persistir vai para `S`, não para o runtime context.

### Riscos e Mitigações

- **Risco:** confundir efêmero com durável (perda no resume). **Mitigação:** documentação + teste explícito de não-persistência; revisão.
- **Risco:** uso de `context.Value` como passagem geral de parâmetros. **Mitigação:** restringir a dependências/valores de request; chave de tipo privada.

## Plano de Implementação

1. Definir `Runtime`, `WithRuntime`, `RuntimeFrom` em `internal/platform/agent` (ou pacote `runtime` dedicado).
2. Repasse do `ctx` pelos steps do kernel (ADR-001) sem serialização.
3. Teste: runtime context disponível no step; ausente após resume a partir de snapshot (prova de não-persistência).

## Monitoramento e Validação

- Teste unitário de não-persistência verde.
- Critério de sucesso: RF-26/RF-27 atendidos; snapshot não contém dados de runtime context.

## Impacto em Documentação e Operação

- Guia do consumidor explica o que vai em `S` (durável) vs runtime context (efêmero).

## Revisão Futura

- Revisitar se surgir necessidade de runtime context com escopo além da execução (novo ADR).
