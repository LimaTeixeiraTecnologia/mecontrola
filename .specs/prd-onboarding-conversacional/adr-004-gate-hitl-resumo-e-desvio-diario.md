# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate HITL do Resumo (ETAPA 7) e desvio de comando diário reusando primitivos do kernel
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** JailtonJunior94 (product owner), time de plataforma
- **Relacionados:** PRD (RF-16/17/18/25, QT-02/QT-06), techspec, R-AGENT-WF-001 (.7-A), R-WF-KERNEL-001.7

## Contexto

A ETAPA 7 (Resumo Final) exige um gate de confirmação ("Está tudo certo?") com **correção guiada por LLM** antes de concluir. Durante o onboarding, comandos de Operação Diária (ex.: "Mercado 120 pix") devem ser **redirecionados gentilmente sem registrar** (RF-25). O agent já possui primitivos de Human-in-the-Loop: suspend/resume durável no kernel, `Codec.MergePatch` no resume e o padrão de estado de espera fechado `AwaitingApproval`/`ConfirmState` (HITL de transações). Duplicar esse mecanismo seria desperdício e fonte de divergência.

## Decisão

**Reusar os primitivos genéricos do kernel** para os dois casos:
- **Gate do Resumo:** o `newSummaryStep` suspende com `Awaiting = AwaitingConfirm` (tipo fechado, espelhando `AwaitingApproval`); o resume aplica o texto do usuário via merge-patch (`{"Inbound": ...}`) sobre o snapshot (fonte única de verdade, R-WF-KERNEL-001.7). Um `Decide*` puro interpreta: confirmar → conclui; corrigir → identifica o campo (objetivo/orçamento/cartões/valores) via LLM, atualiza pelo use case e re-exibe o resumo; ambíguo → pergunta qual campo; reprompt único, depois cancela.
- **Desvio diário:** um `Decide*` puro classifica a entrada de qualquer etapa; se for intent de Operação Diária (não resposta da etapa), retorna `OutcomeDeferred` → o step re-suspende com mensagem de redirecionamento gentil, **sem** chamar o agente diário e **sem** registrar nada. Modelado como decisão tipada — **sem** novo `case intent.Kind` (R-AGENT-WF-001.1).

A resolução do resume ocorre **antes** do `ParseInbound` do agente diário (ordem determinística, espelhando o padrão de categoria/aprovação).

## Alternativas Consideradas

- **Mecanismo de confirmação próprio do onboarding (não reusar).** Vantagem: isolamento. Desvantagem: duplica suspend/resume e estado de espera já existentes; mais superfície de bug e divergência de comportamento. Rejeitada.
- **Rotear comando diário para o agente diário durante o onboarding.** Vantagem: registra na hora. Desvantagem: mistura onboarding e operação diária, contraria o Cap. 07 (operação diária só após conclusão). Rejeitada.

## Consequências

### Benefícios Esperados
- Máximo reúso do kernel; comportamento HITL consistente com o resto do agent.
- Estados de espera tipados (state-as-type), resume robusto via merge-patch.

### Trade-offs e Custos
- Acoplamento ao contrato de suspend/resume do kernel (já é dependência do onboarding por ADR-001).

### Riscos e Mitigações
- **Classificação errada (resposta da etapa vs comando diário):** mitigar com `Decide*` testável e prompts de parse específicos por etapa; em dúvida, tratar como resposta da etapa e pedir esclarecimento (não registrar transação).
- **Loop de correção infinito no resumo:** reprompt único; após resposta ambígua repetida, cancelar/relistar campos.

## Plano de Implementação
1. `OnboardingAwaiting`/`CorrectionTarget` (fechados). 2. `newSummaryStep` com gate. 3. `Decide*` de confirmação/correção e de desvio diário. 4. Wiring resume-antes-do-parse. 5. Testes unitários de cada ramo.

## Monitoramento e Validação
- `onboarding_step_total{step="summary",outcome}` (confirm|correct|cancel), `outcome="deferred"` para desvios diários.
- Critério: comandos diários durante o onboarding não geram transação; correção no resumo atualiza o campo correto.

## Impacto em Documentação e Operação
- Runbook com exemplos de correção no resumo e de redirecionamento de comando diário.

## Revisão Futura
- Revisar se a taxa de `deferred`/correções indicar fricção alta, ou se o produto permitir registrar transações durante o onboarding.
