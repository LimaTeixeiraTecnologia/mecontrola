# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** `buildFinancialTools` como fonte única de verdade do inventário de tools
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma, mantenedor de `internal/agents`
- **Relacionados:** PRD `auditoria-testes-internal-agents`, `techspec.md`, RF-06 a RF-08

## Contexto

O módulo monta 23 tools reais em `buildFinancialTools`, mas testes e scorers usam listas manuais divergentes. Isso já causou drift estrutural: contagem fixa em `module_test.go`, lista incompleta em `mecontrola_scorers.go` e inventário duplicado no harness `realllm`.

Sem uma fonte única de verdade, a suite pode declarar cobertura total com conjunto incorreto de tools.

## Decisão

Usar `buildFinancialTools` como fonte única de verdade do inventário de tools:

- o harness deve construir handles reais com doubles mínimos;
- a cobertura obrigatória por inventário deve derivar `tool.ID()` desses handles;
- cenários de roteamento extra devem ficar fora da conta de completude por inventário.

## Alternativas Consideradas

- Manter lista manual compartilhada.
  - Vantagem: implementação simples.
  - Desvantagem: continua sujeita a drift de IDs.
  - Motivo da rejeição: não elimina falso positivo estrutural.
- Validar apenas quantidade total de tools.
  - Vantagem: baixo custo.
  - Desvantagem: aceita renomeação/omissão com mesmo total.
  - Motivo da rejeição: insuficiente.
- Deixar cada harness manter seu próprio inventário.
  - Vantagem: autonomia local.
  - Desvantagem: múltiplas fontes conflitantes.
  - Motivo da rejeição: piora rastreabilidade.

## Consequências

### Benefícios Esperados

- Falha automática quando nova tool surgir sem cenário correspondente.
- Eliminação de listas duplicadas como fonte primária.
- Melhora do diagnóstico de cobertura real vs cenários extras.

### Trade-offs e Custos

- Harnesses precisarão montar mais dependências fake para chamar `buildFinancialTools`.
- Alguns testes ficarão um pouco mais verbosos.

### Riscos e Mitigações

- Risco: construção dos handles reais exigir doubles complexos.
  - Mitigação: limitar o helper ao bootstrap mínimo do módulo.
- Risco: suites complementares continuarem duplicando IDs.
  - Mitigação: extrair helper compartilhado de inventário no package de teste.

## Plano de Implementação

1. Criar helper de inventário real a partir de `buildFinancialTools`.
2. Substituir contagens fixas por comparação de conjuntos de IDs.
3. Separar matriz `coverageByTool` de `routingScenarios`.
4. Ajustar scorers/listas auxiliares para usar o conjunto derivado.

## Monitoramento e Validação

- O teste deve falhar se `actualIDs != keys(coverageByTool)`.
- O harness deve reportar explicitamente tools sem cenário e cenários sem tool correspondente.

## Impacto em Documentação e Operação

- Atualizar techspec, tasks e convenções de novos testes de tools.
- Sem impacto operacional em produção.

## Revisão Futura

- Revisar se `buildFinancialTools` deixar de ser a composição canônica do módulo.
- Revisar ao introduzir novo mecanismo de registry de tools, se ocorrer.
