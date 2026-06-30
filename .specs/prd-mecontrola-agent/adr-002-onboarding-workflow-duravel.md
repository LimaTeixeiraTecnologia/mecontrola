# Registro de Decisão Arquitetural (ADR-002)

## Metadados

- **Título:** Onboarding de 8 etapas como workflow durável com fase como tipo fechado
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma; dono do produto
- **Relacionados:** PRD (RF-10..RF-19.1, D-02/D-04/D-07/D-14/D-15/D-23), techspec.md; ADR-005; `.claude/rules/workflow-kernel.md`; `.claude/rules/agent-workflows-tools.md`

## Contexto

O onboarding é obrigatório, sequencial e inegociável (8 etapas), com retomada na etapa exata após dúvidas ou dias de inatividade (RF-18/RF-19/RF-19.1), e deve reaproveitar estado pré-existente (D-23). O kernel `internal/platform/workflow` oferece `Engine[S].Start/Resume` com `Snapshot` durável e `MergePatch` no resume (`engine.go`, `codec.go`), e suspend via `StepOutput{Status:StepStatusSuspended, Suspend:&Suspension{...}}`.

## Decisão

Modelar o onboarding como `workflow.Definition[OnboardingState]` com `Root: Sequence("root", welcome, goal, income, cards, methodology, distribution, summary, conclusion)`, `Durable:true`, `MaxAttempts:3`. Cada step que precisa de input do usuário **suspende** (persistindo `OnboardingState` no `Snapshot`) e o turno seguinte **resume por merge-patch** (`{"ResumeText":"..."}`) **antes de qualquer parse**. `OnboardingPhase` é tipo fechado (state-as-type) com 8 constantes; nunca string livre. Regras de domínio dentro dos steps são **puras** (`Decide*`): validação de distribuição fechar 100% (RF-14/D-07), renda como base de distribuição (D-14). Efeitos (criar/ativar orçamento, cadastrar cartão, gravar objetivo na working memory) ocorrem via bindings (ADR-003), não no kernel. **I/O conversacional do onboarding** (ADR-007): as mensagens de cada etapa são geradas por step que chama `agent.Stream` (call-site sancionada) e as respostas do usuário são extraídas por `llm.StructuredContract[T]` (`Strict:true`) — o parse tipado precede a transição de fase; sem LLM fora das call-sites sancionadas. Reuso de estado pré-existente (D-23): step `cards` lista cartões via binding; steps de orçamento reutilizam/ativam o orçamento da competência (único por mês) em vez de duplicar. Recorrência pergunta no fluxo e aplica 12 meses (D-15).

**ETAPA 6 — distribuição em mensagem única (RF-14)**: o step `distribution` apresenta as 5 categorias e coleta os percentuais/valores **num único turno** (não há sub-loop por categoria); o parse estruturado extrai as 5 alocações de uma vez, valida o fechamento em 100% e, se falhar, reapresenta a distribuição completa para ajuste. `OnboardingState.Allocations map[string]int` recebe as 5 de uma vez.

**ETAPA 4 — cartão com apelido + vencimento (RF-15/RF-15.2)**: o step `cards` coleta do usuário **apenas apelido e vencimento (DueDay)**. O binding `CardManager.CreateCard` preenche os obrigatórios do domínio (`create_card.go:18-35`): `Name := apelido`, `LimitCents := 0`, e **`ClosingDay := DueDay`** como default determinístico (fechamento alinhado ao vencimento) — documentado como simplificação ajustável; impacta a competência das parcelas via `BillingCycleResolver`. Conclusão (RF-17) deriva "onboardado" de orçamento ativo + objetivo na working memory (RF-30.1), sem flag de domínio nova.

## Alternativas Consideradas

- **Máquina de estados ad-hoc no agente (sem kernel)** — Vantagem: simples no começo. Desvantagem: reimplementa durabilidade/resume já existentes; viola "consumir, não reimplementar" e R-AGENT-WF-001. Rejeitada.
- **Onboarding 100% LLM sem estado fechado** — Vantagem: flexível. Desvantagem: não garante ordem/retomada determinística; risco de pular etapas (proíbe RF-11). Rejeitada.
- **`phase` como string** — Rejeitada por DMMF state-as-type (governança).

## Consequências

### Benefícios Esperados

- Retomada exata e durável grátis (Snapshot + merge-patch); ordem garantida; testes determinísticos dos steps puros.

### Trade-offs e Custos

- Cada pergunta = um ciclo suspend/resume; exige disciplina para manter steps puros e efeitos nos bindings.

### Riscos e Mitigações

- **Vazar IO/branching de domínio no kernel** → revisão R-WF-KERNEL-001 + gates; manter `Decide*` puro.
- **Estado órfão** → housekeeping do kernel (`DeleteCompleted`); conclusão sempre fecha o run.

## Plano de Implementação

1. Definir `OnboardingPhase`/`OnboardingState` e os 8 steps (puros + efeitos via binding).
2. Implementar suspend/resume por fase; merge-patch antes do parse.
3. Integrar `ResolveOnboardingOrAgent` (decide onboarding × operação por estado derivado).
4. Testes de transição, distribuição 100%, reuso de estado, recorrência 12m.

## Monitoramento e Validação

- `agents_onboarding_phase_total{phase}` e taxa de conclusão (métrica de produto). `workflow_suspend_total`/`workflow_resume_total` coerentes.
- Sucesso: usuário chega à `PhaseConclusion` com orçamento ativo + objetivo na working memory.

## Impacto em Documentação e Operação

- Runbook da jornada de onboarding com strings verbatim e tabela de fases.

## Revisão Futura

- Revisar se novas etapas forem exigidas ou se o reuso de estado precisar de detecção mais rica.
