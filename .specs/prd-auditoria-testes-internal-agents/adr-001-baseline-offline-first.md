# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Baseline offline-first para contratos críticos de `internal/agents`
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma, mantenedor de `internal/agents`
- **Relacionados:** PRD `auditoria-testes-internal-agents`, `techspec.md`, RF-01 a RF-03, RF-09 a RF-12

## Contexto

Parte relevante da robustez atual do módulo depende de suites `integration` e `realllm`. Isso deixa contratos críticos fora do gate mínimo do baseline padrão, permitindo regressões silenciosas em jobs, persistência idempotente e invariantes agentivos.

Ao mesmo tempo, banco real e provider real ainda são necessários para provar unicidade, concorrência e aderência externa. A decisão precisava eliminar a dependência exclusiva dessas suites sem duplicar toda a malha integrada.

## Decisão

Adotar uma estratégia offline-first por contrato para o baseline padrão do módulo:

- mover para o baseline apenas contratos mínimos, determinísticos e estáveis;
- manter `integration` e `realllm` como camada complementar obrigatória;
- impedir que qualquer contrato crítico definido no PRD dependa exclusivamente de ambiente externo para falhar.

Escopo:

- jobs de retenção/confirmação;
- `write_ledger_repository`;
- invariantes agentivos mínimos definidos no PRD.

## Alternativas Consideradas

- Manter somente `integration` e `realllm`.
  - Vantagem: menor esforço.
  - Desvantagem: preserva o gap central do PRD.
  - Motivo da rejeição: não atende robustez do baseline.
- Espelhar quase toda a suíte integrada no baseline.
  - Vantagem: reduz ainda mais dependência de ambiente.
  - Desvantagem: alto custo, redundância e manutenção pior.
  - Motivo da rejeição: escopo desproporcional.
- Usar apenas métricas de cobertura de linhas.
  - Vantagem: métrica simples.
  - Desvantagem: forte risco de cobertura cosmética.
  - Motivo da rejeição: não controla falso positivo comportamental.

## Consequências

### Benefícios Esperados

- Regressões contratuais falham no gate mínimo.
- Redução de falso positivo de cobertura.
- Menor dependência de credenciais e ambiente externo para feedback rápido.

### Trade-offs e Custos

- Mais arquivos `_test.go` e helpers locais.
- Manutenção de duas camadas de prova: offline e complementar.

### Riscos e Mitigações

- Risco: testes offline virarem permissivos demais.
  - Mitigação: oráculos estruturais por contrato e não por keyword.
- Risco: redundância excessiva com `integration`.
  - Mitigação: baseline cobre o mínimo; integração cobre IO real e concorrência.

## Plano de Implementação

1. Adicionar testes offline para jobs.
2. Adicionar suíte offline de `write_ledger_repository`.
3. Introduzir camada offline agentiva mínima.
4. Manter e ajustar suites complementares.

## Monitoramento e Validação

- O baseline deve falhar sem `RUN_REAL_LLM` e sem banco real quando contratos mínimos quebrarem.
- Suites `integration` e `realllm` devem continuar verdes quando ambiente existir.
- Revisar a decisão se o baseline voltar a depender de skips externos.

## Impacto em Documentação e Operação

- Atualizar techspec e futuras tasks.
- Não exige mudanças operacionais em produção.

## Revisão Futura

- Revisar quando os quatro eixos do PRD estiverem implementados e estabilizados.
- Revisar antes de expandir o escopo para blind spots adicionais fora deste PRD.
