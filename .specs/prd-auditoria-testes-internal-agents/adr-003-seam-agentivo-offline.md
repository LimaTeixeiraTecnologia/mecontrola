# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Camada agentiva offline no boundary `llm.Provider` e `agent.Agent`
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Engenharia de plataforma, mantenedor de `internal/agents`
- **Relacionados:** PRD `auditoria-testes-internal-agents`, `techspec.md`, RF-09 a RF-12

## Contexto

O módulo já possui boa cobertura determinística de workflow/state machine, mas invariantes críticos de onboarding, honestidade em falha de tool e roteamento mínimo ainda dependem demais de `realllm`. A principal lacuna está na costura entre prompt, tool-calling, saída estruturada e resposta final.

Era necessário escolher um seam que reutilizasse o runtime real e evitasse tanto rede externa quanto falsos positivos de scorers permissivos.

## Decisão

Adicionar a camada agentiva offline no boundary `llm.Provider`/`agent.Agent`:

- para onboarding, testar `BuildGoalStep` com agente fake ou provider roteirizado retornando `RawJSON` estruturado por `Schema.Name`;
- para roteamento e honestidade, testar `BuildMeControlaAgent` com provider roteirizado e tools reais/fakes, observando `ToolCalls`, `ToolOutcome` e `Content`;
- manter suites `realllm` como smoke complementar de aderência externa.

## Alternativas Consideradas

- Testar apenas presença de instruções no prompt.
  - Vantagem: simples.
  - Desvantagem: não prova execução.
  - Motivo da rejeição: insuficiente para RF-09 a RF-12.
- Criar workflow fake paralelo para simular decisões do agente.
  - Vantagem: total determinismo.
  - Desvantagem: não usa o runtime real.
  - Motivo da rejeição: alto risco de divergência.
- Usar scorers por keyword como gate principal.
  - Vantagem: baixo custo.
  - Desvantagem: aprova saídas alucinadas ou roteamento parcial.
  - Motivo da rejeição: alto falso positivo.

## Consequências

### Benefícios Esperados

- Prova offline do ponto exato onde hoje existe a maior lacuna.
- Reuso do runtime real sem dependência de provider externo.
- Diagnóstico melhor com sequência real de tool calls e `ToolOutcome`.

### Trade-offs e Custos

- Exige construir provider roteirizado ou usar mocks mais sofisticados.
- Pode aumentar custo de manutenção se o teste validar string demais.

### Riscos e Mitigações

- Risco: overfitting ao transcript do mock.
  - Mitigação: validar semântica mínima do request e da sequência, não a prosa inteira.
- Risco: perder aderência ao provider real.
  - Mitigação: manter suites `realllm` como smoke e contract externo.

## Plano de Implementação

1. Criar helper de provider roteirizado para cenários offline.
2. Cobrir onboarding combinado por `Schema.Name`.
3. Cobrir honestidade em falha de tool por `ToolOutcomeUsecaseError`.
4. Cobrir roteamento mínimo C1, C4, C5 e um cenário de escrita.

## Monitoramento e Validação

- Validar `ToolCalls`, `ToolOutcome`, `Content` e estados de suspensão/conclusão.
- Falhar quando a sequência esperada divergir, quando houver falso sucesso textual ou quando `GoalValueAsked`/`GoalValueCents` saírem do contrato.

## Impacto em Documentação e Operação

- Atualizar techspec e futuras tasks.
- Sem impacto em produção; escopo restrito à malha de testes.

## Revisão Futura

- Revisar se o runtime `agent.Agent` mudar de contrato público.
- Revisar se os testes offline passarem a divergir sistematicamente das suites `realllm`.
