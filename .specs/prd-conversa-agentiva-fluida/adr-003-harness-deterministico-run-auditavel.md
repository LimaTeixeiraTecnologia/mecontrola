# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Harness determinístico com Run auditável como gate primário
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia / Produto MeControla
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige 100% de retomada correta nos cenários canônicos, 0 confusão entre pendências e 0 sucesso simulado. Scorers LLM-judged são úteis para observabilidade, mas não provam determinismo nem escrita real.

## Decisão

Usar harness determinístico como fonte oficial de aceite para M-01 e M-06. O harness deve verificar estado final, tool calls, escrita real quando aplicável, ausência de escrita quando bloqueado, resposta final, `platform_runs`, `workflow_runs`, `workflow_steps`, `agents_write_ledger` e Run auditável.

## Alternativas Consideradas

- Scorer LLM-judged como gate: rejeitado por ser probabilístico.
- Logs manuais: rejeitado por baixa reprodutibilidade.
- Teste apenas unitário de funções puras: insuficiente para provar Run/tool/write.

## Consequências

### Benefícios Esperados

- Evidência objetiva de produção-ready.
- Regressões conversacionais detectáveis antes do merge.
- Separação entre qualidade observada por scorer e correctness determinística.

### Trade-offs e Custos

- Mais fixtures e doubles determinísticos.
- Necessidade de manter cenários canônicos alinhados ao PRD.

### Riscos e Mitigações

- Risco de harness divergir do runtime real. Mitigação: incluir testes que passam por `AgentRuntime` quando aplicável e integração com workflow store.
- Risco de cenários incompletos. Mitigação: mapear CA-01..CA-12 diretamente no harness.

## Plano de Implementação

1. Criar doubles determinísticos de provider, categories, ledger e gateway.
2. Implementar cenários CA-01..CA-12.
3. Verificar Run, tool calls, estado, snapshots e escrita real.
4. Incluir gate nos comandos de validação da tarefa.

## Monitoramento e Validação

O gate passa somente com 100% dos cenários canônicos verdes e 0 escrita indevida. Scorers continuam rodando como sinal complementar.

## Impacto em Documentação e Operação

Documentar como rodar o harness e interpretar falhas.

## Revisão Futura

Revisar quando novos slots ou múltiplas pendências simultâneas entrarem no escopo do produto.
