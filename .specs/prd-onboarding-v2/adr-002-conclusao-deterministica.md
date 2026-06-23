# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Conclusão determinística do onboarding com `completed_at` e detecção de drift
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD (RF-23..26, RF-31, RF-35), techspec.md, ADR-001, ADR-003

## Contexto

`CompleteOnboardingSession` hoje promove `state=active` mas não persiste `completed_at` no payload.
O handoff para o agente principal deve depender de sinal determinístico persistido, nunca de
heurística textual. Sem `completed_at`, um registro `state=active` sem marco de conclusão é
ambíguo (drift) e pode ser lido como sucesso silencioso. A regra de produto exige 0 falso positivo:
a conclusão só ocorre com todos os pré-requisitos (objetivo, renda, distribuição, cartões e primeira
transação).

## Decisão

Concluir o onboarding em um único write transacional (`uow.Do`) que: valida idempotência
(`IsActive` ⇒ `AlreadyActive`) e pré-requisito (`HasFirstTransaction`), aplica `WithCompletion(now)`
(define `state=active`, `payload.CompletedAt=now` e zera `RecentTurns`), faz `Upsert` e publica
`OnboardingCompleted` na mesma transação. Na leitura (`Find`), `state=active` com `CompletedAt==nil`
é tratado como drift explícito: incrementa `onboarding_state_drift_total` e loga warn, sem mascarar
como sucesso.

## Alternativas Consideradas

- **Inferir conclusão por `phase`/texto**: rejeitada — viola RF-26 (0 falso positivo) e o princípio
  de fato de domínio persistido.
- **`completed_at` como coluna dedicada**: rejeitada para o MVP — `payload` JSONB já carrega o estado
  funcional; evita migração e mantém o marco junto do estado que ele conclui.

## Consequências

### Benefícios Esperados

- Handoff inequívoco (RF-26); conclusão auditável e idempotente (RF-24/25).
- Drift visível e acionável (RF-31).

### Trade-offs e Custos

- Acoplamento do marco ao JSONB (consultas por `completed_at` exigem extrair do payload). Aceitável
  no MVP (volume baixo, acesso por `user_id`).

### Riscos e Mitigações

- **Risco:** falha entre `Upsert` e `Publish`. **Mitigação:** ambos no mesmo `uow.Do` (atômico);
  outbox garante entrega.
- **Rollback:** manter `MarkActive` antigo como caminho de contingência (sem `completed_at`),
  sinalizando drift.

## Plano de Implementação

1. Adicionar `WithCompletion` ao agregado (ADR-001 fornece os campos).
2. Atualizar `CompleteOnboardingSession.Execute` para gravar `completed_at` + limpar turns + publicar.
3. Adicionar detecção de drift no `Find` da repository.

## Monitoramento e Validação

- Métrica `onboarding_state_drift_total` (deve ser ~0).
- Teste de integração: `state=active` ⇒ `completed_at` presente + evento na outbox.
- Teste unitário de idempotência (`AlreadyActive`) e pré-requisito (`ErrFirstTransactionRequired`).

## Impacto em Documentação e Operação

- Runbook: critério de handoff = `state=active` ∧ `completed_at` ∨ evento `OnboardingCompleted`.
- Alerta sugerido: `onboarding_state_drift_total > 0`.

## Revisão Futura

- Reavaliar coluna dedicada `completed_at` se surgirem consultas analíticas frequentes por conclusão.
