# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Persistência isolada do onboarding em `onboarding_sessions.payload`
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD `.specs/prd-onboarding-v2/prd.md` (RF-19..22, RF-29, DR-08, LG-01/02/03), techspec.md

## Contexto

O onboarding LLM (`RunOnboardingTurn`) lê e grava `recent_turns` em `mecontrola.agent_sessions`
(`loadOnbHistory`/`saveOnbTurn`), tabela compartilhada com o agente principal pela chave
`(user_id, channel)`. Como onboarding e daily agent operam no mesmo canal WhatsApp, há risco de
colisão: histórico e estado transitório de um podem sobrescrever o do outro. Além disso, a
idempotência da saudação proativa (RF-29) e a marcação de conclusão precisam de marcos persistidos
(`welcome_sent_at`, `completed_at`) que hoje não existem no payload do onboarding.

## Decisão

Tornar `mecontrola.onboarding_sessions.payload` (JSONB) a única fonte do histórico e dos marcos do
onboarding. Adicionar ao `OnboardingSessionPayload`: `RecentTurns []OnboardingTurn` (bounded em 3
pares), `WelcomeSentAt *time.Time`, `CompletedAt *time.Time`. Introduzir `OnboardingTurn{Role, Text,
OccurredAt}`. O acesso ao histórico passa a ser feito por **usecases do onboarding**
(`AppendOnboardingTurn`, `LoadOnboardingTurns`, `MarkWelcomeSent`); o agente substitui o
`onboardingSessionReader` (sobre `agent_sessions`) por um `OnboardingHistoryGateway` cuja
implementação é um **adapter fino de binding** para esses usecases — o agente nunca acessa
`onboarding_sessions` diretamente (ADR-006). `emitWelcome` chama `MarkWelcomeSent` e não reemite
quando `welcome_sent_at` já presente. Sem migração de schema (coluna `payload` já é JSONB).
`agent_sessions` permanece exclusivo do agente principal.

## Alternativas Consideradas

- **Manter `recent_turns` em `agent_sessions` com discriminador**: adicionar coluna
  `pending_action_kind`/namespacing. Vantagem: menos código. Desvantagem: não elimina a colisão de
  fronteira nem o acoplamento; contraria a decisão de produto de isolamento. Rejeitada.
- **Tabela nova `onboarding_turns`**: normalizar turnos. Vantagem: queracapacidade. Desvantagem:
  migração + join + overhead para uma janela efêmera de 3 pares. Rejeitada (over-engineering p/ MVP).

## Consequências

### Benefícios Esperados

- Elimina a colisão `(user_id, channel)` entre onboarding e agente principal (RF-19/21).
- Habilita idempotência da saudação (RF-29) e conclusão determinística (ADR-002) via marcos no payload.
- Retomada confiável a partir do estado persistido (RF-30).

### Trade-offs e Custos

- Histórico de onboarding em voo (pré-deploy) permanece em `agent_sessions` e será descartado
  (aceitável: janela efêmera; estado funcional vive em `onboarding_sessions`).
- `payload` cresce; mitigado pelo bound de 3 pares e limpeza na conclusão (RF-35).

### Riscos e Mitigações

- **Risco:** payload inchar. **Mitigação:** append bounded (6 entradas) + `RecentTurns=nil` ao concluir.
- **Rollback:** reverter o gateway para o `onboardingSessionReader` anterior; o payload estendido é
  retrocompatível (campos `omitempty`).

## Plano de Implementação

1. Estender `OnboardingSessionPayload` + `OnboardingTurn` (domínio) e o JSON da repository.
2. Implementar `OnboardingHistoryGateway` sobre `onboarding_sessions`.
3. Trocar `loadOnbHistory`/`saveOnbTurn` em `RunOnboardingTurn`; `emitWelcome` marca `welcome_sent_at`.
4. Remover dependência de `agent_sessions` do fluxo de onboarding.

## Monitoramento e Validação

- `agent_onboarding_welcome_dedup_total{result}`.
- Teste de integração: pós-onboarding, `agent_sessions` sem histórico do onboarding.
- Gate de revisão: grep confirma ausência de `onboardingSessionReader` no fluxo de onboarding.

## Impacto em Documentação e Operação

- Atualizar runbook do onboarding (fonte de verdade = `onboarding_sessions`).
- Dashboard: painel de dedup de saudação.

## Revisão Futura

- Revisar se o volume de turnos exigir tabela dedicada ou se múltiplos canais por usuário forem
  suportados.
