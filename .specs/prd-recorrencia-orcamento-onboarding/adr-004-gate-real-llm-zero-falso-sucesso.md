# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Gate real-LLM estendendo `TestRecurrenceExtractionGate` com 0 falso-sucesso e counter de outcome
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Jailton (owner), MeControla agents
- **Relacionados:** PRD (RF-16, RF-17, RF-19); techspec; regra `R-TXN-004`/`R-AGENT-WF-001.5` (cardinalidade); skill `mastra`.

## Contexto

O RF-17 exige validar a compreensão de linguagem natural com LLM real (≥0,90 aderência, 0 falso-sucesso) cobrindo negativa, positiva-12, N numérico, N por extenso, inválido e ambíguo, além de testes unitários determinísticos. O projeto tem dois harnesses real-LLM: (1) o registry golden (`internal/agents/application/golden/`), que exercita **tools do agente diário** via `BuildMeControlaAgent` — e explicitamente **não** cobre steps do onboarding (`cases_onboarding.go:31,44`); (2) o suite dedicado ao onboarding (`onboarding_workflow_integration_test.go`), que já possui `TestRecurrenceExtractionGate:680` chamando `BuildRecurrenceStep` com agente real e gate ≥0,90. O gate atual mede só o ratio agregado sobre 5 cenários binários (sim/não/ambíguo) e não impõe separadamente "não aplicou recorrência indevida". Não existe métrica de outcome para o step (grep vazio).

## Decisão

Materializar o gate do RF-17 **estendendo `TestRecurrenceExtractionGate`** (não o registry golden). Trocar o `expected{recurrence bool}` por `recurrenceOutcomeKind` e cobrir os 5 tipos de resposta (com múltiplas formas por tipo, extraídas dos exemplos da US). Manter `require.GreaterOrEqual(ratio, 0.90, ...)`. Adicionar asserção explícita de **0 falso-sucesso**: nenhum cenário negativo/inválido/ambíguo pode ter chamado `CreateRecurrence` (via `BudgetPlanner` mock com contagem), e os cenários específicos devem aplicar exatamente o N esperado — não apenas o ratio agregado. Adicionar o counter `agents_onboarding_recurrence_total` com label `outcome` fechado (cardinalidade controlada; proibido `months`/`user_id` como label). Liberação direta sem feature flag (RF-19): o gate é a barreira de merge.

## Alternativas Consideradas

- **Adicionar cases no registry golden (`CategoryRecurrence`/`CategoryOnboarding`)**: o mecanismo golden roda o agente-tool, não `BuildRecurrenceStep`; não exercita o step. Rejeitada por não atender o RF-17.
- **Criar scorer novo de "meses aplicados corretos"**: forçaria atualizar `RegisteredScorers` no `regression_contract.go` e adicionar um scorer ao pipeline. Desnecessário: a correção do N é verificável diretamente no gate do step (mock `CreateRecurrence` + asserção). Rejeitada por custo sem ganho; mantém `postdeploy` inalterado.
- **Só ratio agregado (sem 0 falso-sucesso explícito)**: um cenário negativo que aplicasse 12 meses ainda poderia passar no ratio. Rejeitada por não garantir "0 falso-sucesso" do RF-17.
- **Label de métrica por `months`**: alta cardinalidade e derivado de input. Rejeitada (R-TXN-004); usar `outcome`.

## Consequências

### Benefícios Esperados

- Gate fiel ao padrão do projeto, no slot já dedicado ao step.
- Garantia forte de "0 falso-sucesso" (mutação indevida bloqueia o merge).
- Observabilidade de produto por outcome, cardinalidade controlada.
- `postdeploy/regression_contract.go` inalterado (nenhum scorer novo).

### Trade-offs e Custos

- Mais cenários no gate (custo de tokens do teste real-LLM, executado só com `RUN_REAL_LLM=1`).
- O gate real-LLM não roda no CI padrão (build tag `integration` + env); é uma barreira manual/curada, consistente com os demais gates do projeto.

### Riscos e Mitigações

- Risco: flutuação do modelo derrubar o ratio abaixo de 0,90. Mitigação: exemplos ricos no prompt (ADR-002); modelo default `openai/gpt-4o-mini` fixado por `AGENT_HARNESS_MODEL`.
- Risco: falso-sucesso escapar do ratio. Mitigação: asserção separada de não-chamada de `CreateRecurrence` para não-aplicáveis.

## Plano de Implementação

1. Estender `TestRecurrenceExtractionGate` com outcome fechado + cenários dos 5 tipos.
2. Adicionar asserção de 0 falso-sucesso (mock `CreateRecurrence` com contagem/spy por cenário).
3. Adicionar `agents_onboarding_recurrence_total` + `record...` + wiring em `BuildOnboardingWorkflow`.
4. Testes unitários determinísticos (ADR-001) como gate (b).

## Monitoramento e Validação

- Sucesso: `RUN_REAL_LLM=1` → gate ≥0,90 e 0 falso-sucesso; unit verdes; counter emitindo os 5 outcomes.
- Reverter: se o gate ficar instável, isolar cenários problemáticos e ajustar exemplos do prompt antes de baixar o threshold (não baixar sem decisão de produto).

## Impacto em Documentação e Operação

- Techspec. Dashboard de onboarding pode ganhar a série `agents_onboarding_recurrence_total` (opcional, fora do escopo de código).

## Revisão Futura

- Revisitar o conjunto de cenários do gate quando o counter indicar formas frequentes de `ambiguous_reprompt`/`invalid_reprompt` em produção.
